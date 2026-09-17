package bot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"agy-tele/config"
	"agy-tele/internal/artifact"
	"agy-tele/internal/engine"
	"agy-tele/internal/i18n"
	"agy-tele/internal/renderer"
	"agy-tele/internal/session"
	"agy-tele/internal/throttler"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type ActiveTask struct {
	UserID    int64
	ChatID    int64
	Prompt    string
	StartedAt time.Time
	Cancel    context.CancelFunc
	IsGoal    bool
}

type Router struct {
	cfg          *config.Config
	bot          *tgbotapi.BotAPI
	sm           *session.SessionManager
	oneShot      *engine.OneShotRunner
	streamRunner *engine.StreamAgentRunner
	taskMu       sync.Mutex
	activeTasks  map[int64]*ActiveTask
}

func NewRouter(cfg *config.Config, bot *tgbotapi.BotAPI, sm *session.SessionManager) *Router {
	return &Router{
		cfg:          cfg,
		bot:          bot,
		sm:           sm,
		oneShot:      engine.NewOneShotRunner(cfg.Agy.BinaryPath),
		streamRunner: engine.NewStreamAgentRunner(cfg.Agy.BinaryPath),
		activeTasks:  make(map[int64]*ActiveTask),
	}
}

func (r *Router) getActiveTask(userID int64) *ActiveTask {
	r.taskMu.Lock()
	defer r.taskMu.Unlock()
	return r.activeTasks[userID]
}

func (r *Router) registerActiveTask(userID, chatID int64, prompt string, cancel context.CancelFunc, isGoal bool) *ActiveTask {
	r.taskMu.Lock()
	defer r.taskMu.Unlock()
	task := &ActiveTask{
		UserID:    userID,
		ChatID:    chatID,
		Prompt:    prompt,
		StartedAt: time.Now(),
		Cancel:    cancel,
		IsGoal:    isGoal,
	}
	r.activeTasks[userID] = task
	return task
}

func (r *Router) unregisterActiveTask(userID int64) {
	r.taskMu.Lock()
	defer r.taskMu.Unlock()
	delete(r.activeTasks, userID)
}

func (r *Router) cancelActiveTask(userID int64, chatID int64) bool {
	r.taskMu.Lock()
	task, ok := r.activeTasks[userID]
	if ok && task != nil {
		if task.Cancel != nil {
			task.Cancel()
		}
		delete(r.activeTasks, userID)
	}
	r.taskMu.Unlock()

	killed := r.streamRunner.CancelActive(userID)
	return ok || killed
}

func (r *Router) HandleUpdate(update tgbotapi.Update) {
	// Handle Callback Queries (Inline Buttons)
	if update.CallbackQuery != nil {
		r.handleCallbackQuery(update.CallbackQuery)
		return
	}

	// Handle Messages
	if update.Message == nil {
		return
	}

	msg := update.Message
	userID := msg.From.ID
	chatID := msg.Chat.ID
	userMsgID := msg.MessageID

	// Security: Whitelist check
	if !r.cfg.IsUserAllowed(userID) {
		reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("⛔ <b>Akses Ditolak</b>\nUser ID <code>%d</code> tidak terdaftar dalam whitelist bot.", userID))
		reply.ParseMode = "HTML"
		_, _ = r.bot.Send(reply)
		return
	}

	sess := r.sm.GetSession(userID, chatID)

	text := strings.TrimSpace(msg.Text)

	// Immediate /cancel or /stop interceptor (always allowed even if a task is running)
	if text != "" && (strings.EqualFold(text, "/cancel") || strings.EqualFold(text, "/stop")) {
		if r.cancelActiveTask(userID, chatID) {
			r.sendText(chatID, "🛑 <b>Proses aktif berhasil dihentikan!</b>")
		} else {
			r.sendText(chatID, "ℹ️ Tidak ada proses yang sedang aktif berjalan.")
		}
		return
	}

	// Concurrency Guard: prevent duplicate concurrent agy processes for the same user
	if task := r.getActiveTask(userID); task != nil {
		// Allow /status to inspect ongoing progress
		if text != "" && strings.HasPrefix(strings.ToLower(text), "/status") {
			dur := time.Since(task.StartedAt).Round(time.Second)
			promptDesc := fmt.Sprintf("%s (%s)", task.Prompt, dur.String())
			reply := tgbotapi.NewMessage(chatID, FormatStatus(sess.Language, sess, true, promptDesc))
			reply.ParseMode = "HTML"
			reply.ReplyMarkup = StatusActionKeyboard(sess.Language)
			_, _ = r.bot.Send(reply)
			return
		}

		dur := time.Since(task.StartedAt).Round(time.Second)
		promptSummary := task.Prompt
		if len(promptSummary) > 50 {
			promptSummary = promptSummary[:47] + "..."
		}
		warningMsg := fmt.Sprintf(
			"⚠️ <b>Antigravity sedang aktif memproses instruksi:</b>\n"+
				"• <i>\"%s\"</i>\n"+
				"• <b>Durasi berjalan:</b> <code>%s</code>\n\n"+
				"⏳ <i>Harap tunggu hingga proses selesai sebelum mengirim pesan baru, atau gunakan tombol di bawah untuk membatalkan proses yang sedang berjalan.</i>",
			renderer.EscapeHTML(promptSummary), dur.String(),
		)
		reply := tgbotapi.NewMessage(chatID, warningMsg)
		reply.ParseMode = "HTML"
		kb := ActiveTaskKeyboard(sess.Language)
		reply.ReplyMarkup = &kb
		_, _ = r.bot.Send(reply)
		return
	}

	// Check if message contains media/file attachments (Photo, Document, Video, Audio, Voice)
	hasMedia := len(msg.Photo) > 0 || msg.Document != nil || msg.Video != nil || msg.Audio != nil || msg.Voice != nil
	if hasMedia {
		media, err := r.DownloadMessageMedia(msg, sess.CWD)
		if err != nil {
			r.sendText(chatID, fmt.Sprintf("❌ Gagal mengunduh file lampiran: %v", err))
			return
		}
		if media != nil {
			sizeKB := float64(media.SizeBytes) / 1024.0
			r.sendText(chatID, fmt.Sprintf("📥 <b>File Diterima:</b> <code>%s</code> (%.1f KB)\n💾 Disimpan ke: <code>%s</code>\n⏳ <i>Antigravity sedang menganalisis file...</i>",
				renderer.EscapeHTML(media.FileName), sizeKB, renderer.EscapeHTML(media.FilePath)))

			prompt := ""
			caption := strings.TrimSpace(msg.Caption)
			if caption != "" {
				prompt = fmt.Sprintf("User telah mengunggah file ke workspace: %s\n\nCatatan/instruksi user:\n%s\n\nSilakan periksa, baca, dan tanggapi permintaan user mengenai file tersebut.", media.FilePath, caption)
			} else {
				prompt = fmt.Sprintf("User telah mengunggah file ke workspace: %s\n\nSilakan periksa dan jelaskan/analisis isi dari file tersebut.", media.FilePath)
			}

			r.executeAgentTurn(chatID, sess, engine.StreamRunOptions{
				UserID:         userID,
				Prompt:         prompt,
				CWD:            sess.CWD,
				ConversationID: sess.ActiveConversationID,
				PermissionMode: sess.PermissionMode,
				Model:          sess.ActiveModel,
				Effort:         sess.ActiveEffort,
				PrintTimeout:   r.cfg.Agy.PrintTimeout,
			}, userMsgID)
			return
		}
	}

	if text == "" {
		return
	}

	// Command routing
	if strings.HasPrefix(text, "/") {
		r.handleCommand(msg, sess, text)
		return
	}

	// Default: Mode B Agent Prompt
	r.executeAgentTurn(chatID, sess, engine.StreamRunOptions{
		UserID:         userID,
		Prompt:         text,
		CWD:            sess.CWD,
		ConversationID: sess.ActiveConversationID,
		PermissionMode: sess.PermissionMode,
		Model:          sess.ActiveModel,
		Effort:         sess.ActiveEffort,
		PrintTimeout:   r.cfg.Agy.PrintTimeout,
	}, userMsgID)
}

func (r *Router) handleCommand(msg *tgbotapi.Message, sess *session.UserSession, text string) {
	chatID := msg.Chat.ID
	userID := msg.From.ID
	parts := strings.Fields(text)
	cmd := strings.ToLower(parts[0])
	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(text[len(parts[0]):])
	}

	switch cmd {
	case "/start", "/help":
		reply := tgbotapi.NewMessage(chatID, FormatHelp(sess.Language))
		reply.ParseMode = "HTML"
		reply.ReplyMarkup = QuickActionKeyboard(sess.Language)
		_, _ = r.bot.Send(reply)

	case "/usage", "/quota":
		r.executeOneShot(chatID, sess.CWD, "📊 Model Quota & Limit", "/usage", sess.Language)

	case "/credits":
		r.executeOneShot(chatID, sess.CWD, "💰 G1 Credits", "/credits", sess.Language)

	case "/skills":
		r.executeOneShot(chatID, sess.CWD, "🧰 Available Skills", "/skills", sess.Language)

	case "/agents":
		r.executeOneShot(chatID, sess.CWD, "🤖 Custom Agents", "/agents", sess.Language)

	case "/changelog":
		r.executeOneShot(chatID, sess.CWD, "📝 Changelog", "/changelog", sess.Language)

	case "/status":
		task := r.getActiveTask(userID)
		isRunning := task != nil
		taskDesc := ""
		if isRunning {
			dur := time.Since(task.StartedAt).Round(time.Second)
			taskDesc = fmt.Sprintf("%s (%s)", task.Prompt, dur.String())
		}
		reply := tgbotapi.NewMessage(chatID, FormatStatus(sess.Language, sess, isRunning, taskDesc))
		reply.ParseMode = "HTML"
		reply.ReplyMarkup = StatusActionKeyboard(sess.Language)
		_, _ = r.bot.Send(reply)

	case "/model":
		if args != "" {
			if strings.ToLower(args) == "default" || strings.ToLower(args) == "reset" {
				r.sm.SetModel(userID, "")
				reply := tgbotapi.NewMessage(chatID, "✅ Active model direset ke <b>Default (otomatis agy)</b>.")
				reply.ParseMode = "HTML"
				reply.ReplyMarkup = CloseOnlyKeyboard(sess.Language)
				_, _ = r.bot.Send(reply)
			} else {
				r.sm.SetModel(userID, args)
				reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Active model diatur ke: <code>%s</code>", args))
				reply.ParseMode = "HTML"
				reply.ReplyMarkup = CloseOnlyKeyboard(sess.Language)
				_, _ = r.bot.Send(reply)
			}
		} else {
			active := sess.ActiveModel
			if active == "" {
				active = "(Default agy)"
			}
			reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("🧠 <b>Pilih Model Antigravity</b>\nModel aktif saat ini: <code>%s</code>", active))
			reply.ParseMode = "HTML"
			reply.ReplyMarkup = ModelSelectionKeyboard(sess.Language)
			_, _ = r.bot.Send(reply)
		}

	case "/effort":
		if args != "" {
			effort := strings.ToLower(args)
			if effort == "low" || effort == "medium" || effort == "high" {
				r.sm.SetEffort(userID, effort)
				reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Reasoning effort diatur ke: <b>%s</b>", effort))
				reply.ParseMode = "HTML"
				reply.ReplyMarkup = CloseOnlyKeyboard(sess.Language)
				_, _ = r.bot.Send(reply)
			} else {
				r.sendText(chatID, "Pilihan effort valid: <code>low</code>, <code>medium</code>, atau <code>high</code>.")
			}
		} else {
			active := sess.ActiveEffort
			if active == "" {
				active = "(Default agy)"
			}
			reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("⚡ <b>Set Reasoning Effort</b>\nEffort aktif saat ini: <b>%s</b>", active))
			reply.ParseMode = "HTML"
			reply.ReplyMarkup = EffortSelectionKeyboard(sess.Language)
			_, _ = r.bot.Send(reply)
		}

	case "/permission", "/perm":
		if args != "" {
			mode := strings.ToLower(args)
			if mode == "auto" || mode == "ask" {
				r.sm.SetPermissionMode(userID, mode)
				reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Mode permission diatur ke: <b>%s</b>", mode))
				reply.ParseMode = "HTML"
				reply.ReplyMarkup = CloseOnlyKeyboard(sess.Language)
				_, _ = r.bot.Send(reply)
			} else {
				r.sendText(chatID, "Pilihan valid: <code>/permission auto</code> atau <code>/permission ask</code>")
			}
		} else {
			reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("🔒 <b>Mode Persetujuan Tools</b>\nSaat ini: <b>%s</b>\n\n• <b>Auto-Approve</b>: Aksi & perubahan file disetujui otomatis.\n• <b>Ask User</b>: Konfirmasi manual sebelum eksekusi.", sess.PermissionMode))
			reply.ParseMode = "HTML"
			reply.ReplyMarkup = PermissionSelectionKeyboard(sess.Language)
			_, _ = r.bot.Send(reply)
		}

	case "/cwd":
		if args != "" {
			newPath := filepath.Clean(args)
			info, err := os.Stat(newPath)
			if err != nil {
				r.sendText(chatID, fmt.Sprintf("❌ Direktori tidak ditemukan: <code>%s</code> (%v)", newPath, err))
				return
			}
			if !info.IsDir() {
				r.sendText(chatID, fmt.Sprintf("❌ Path bukan merupakan direktori: <code>%s</code>", newPath))
				return
			}
			r.sm.UpdateCWD(userID, newPath)
			reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("📁 Workspace CWD berhasil diubah ke:\n<code>%s</code>", newPath))
			reply.ParseMode = "HTML"
			reply.ReplyMarkup = CloseOnlyKeyboard(sess.Language)
			_, _ = r.bot.Send(reply)
		} else {
			reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("📁 <b>Direktori Kerja Saat Ini:</b>\n<code>%s</code>\n\nGunakan <code>/cwd &lt;path&gt;</code> untuk berpindah folder.", sess.CWD))
			reply.ParseMode = "HTML"
			reply.ReplyMarkup = CloseOnlyKeyboard(sess.Language)
			_, _ = r.bot.Send(reply)
		}

	case "/pwd":
		reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("📁 <b>Current Working Directory:</b>\n<code>%s</code>", sess.CWD))
		reply.ParseMode = "HTML"
		reply.ReplyMarkup = CloseOnlyKeyboard(sess.Language)
		_, _ = r.bot.Send(reply)

	case "/ls":
		targetPath := sess.CWD
		if args != "" {
			if filepath.IsAbs(args) {
				targetPath = filepath.Clean(args)
			} else {
				targetPath = filepath.Join(sess.CWD, args)
			}
		}
		r.handleListDir(chatID, targetPath, sess.Language)

	case "/new":
		if args != "" {
			newPath := filepath.Clean(args)
			if info, err := os.Stat(newPath); err == nil && info.IsDir() {
				r.sm.UpdateCWD(userID, newPath)
			}
		}
		r.sm.ResetConversation(userID)
		reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("🔄 <b>Sesi Percakapan Direset</b>\nSiap memulai sesi percakapan baru di workspace:\n<code>%s</code>", sess.CWD))
		reply.ParseMode = "HTML"
		reply.ReplyMarkup = CloseOnlyKeyboard(sess.Language)
		_, _ = r.bot.Send(reply)

	case "/resume", "/switch":
		if args != "" {
			target := strings.TrimSpace(args)
			convs := r.sm.GetAvailableConversations(userID)
			var selected *session.AvailableConversation
			for _, c := range convs {
				if c.ID == target || strings.HasPrefix(c.ID, target) {
					selected = &c
					break
				}
			}

			convID := target
			title := "Percakapan"
			wsPath := sess.CWD
			timeLabel := ""

			if selected != nil {
				convID = selected.ID
				if selected.Title != "" {
					title = selected.Title
				}
				timeLabel = selected.TimeLabel
				if selected.Workspace != "" {
					wsPath = selected.Workspace
					if info, err := os.Stat(wsPath); err == nil && info.IsDir() {
						r.sm.UpdateCWD(userID, wsPath)
					}
				}
			}

			r.sm.UpdateConversation(userID, convID, "")

			timeLine := ""
			if timeLabel != "" {
				timeLine = fmt.Sprintf("• <b>Waktu</b>: %s\n", timeLabel)
			}

			reply := tgbotapi.NewMessage(chatID, fmt.Sprintf(
				"✅ <b>Sesi Obrolan Berhasil Di-Resume!</b>\n\n"+
					"• <b>Topik</b>: %s\n"+
					"• <b>Workspace</b>: <code>%s</code>\n"+
					"%s\n"+
					"💬 <i>Silakan langsung kirim pesan apa saja di chat untuk melanjutkan percakapan ini.</i>",
				renderer.EscapeHTML(title), wsPath, timeLine,
			))
			reply.ParseMode = "HTML"
			kb := ResumeConfirmedKeyboard(convID, sess.Language)
			reply.ReplyMarkup = &kb
			_, _ = r.bot.Send(reply)
			return
		}

		convs := r.sm.GetAvailableConversations(userID)
		if len(convs) == 0 {
			reply := tgbotapi.NewMessage(chatID, i18n.T(sess.Language, "no_conversation_history"))
			reply.ParseMode = "HTML"
			reply.ReplyMarkup = CloseOnlyKeyboard(sess.Language)
			_, _ = r.bot.Send(reply)
			return
		}
		reply := tgbotapi.NewMessage(chatID, "📂 <b>Pilih Sesi untuk Di-Resume (/resume):</b>\nSilakan pilih salah satu sesi di bawah:")
		reply.ParseMode = "HTML"
		reply.ReplyMarkup = ResumeKeyboard(convs, sess.ActiveConversationID, sess.Language)
		_, _ = r.bot.Send(reply)

	case "/continue":
		r.sendText(chatID, "⏳ Melanjutkan sesi percakapan sebelumnya...")
		r.executeAgentTurn(chatID, sess, engine.StreamRunOptions{
			UserID:         userID,
			Prompt:         "Continue the previous task.",
			CWD:            sess.CWD,
			ConversationID: sess.ActiveConversationID,
			PermissionMode: sess.PermissionMode,
			Model:          sess.ActiveModel,
			Effort:         sess.ActiveEffort,
			PrintTimeout:   r.cfg.Agy.PrintTimeout,
		}, msg.MessageID)

	case "/sessions":
		convs := r.sm.GetAvailableConversations(userID)
		reply := tgbotapi.NewMessage(chatID, FormatSessions(sess.Language, convs, sess.ActiveConversationID))
		reply.ParseMode = "HTML"
		reply.ReplyMarkup = ResumeKeyboard(convs, sess.ActiveConversationID, sess.Language)
		_, _ = r.bot.Send(reply)

	case "/cancel", "/stop":
		if r.cancelActiveTask(userID, chatID) {
			r.sendText(chatID, "🛑 <b>Proses aktif berhasil dihentikan!</b>")
		} else {
			r.sendText(chatID, "ℹ️ Tidak ada proses yang sedang aktif berjalan.")
		}

	case "/artifact", "/artifacts":
		items, err := artifact.ListArtifacts(sess.ActiveConversationID)
		if err != nil || len(items) == 0 {
			reply := tgbotapi.NewMessage(chatID, i18n.T(sess.Language, "no_artifacts_found"))
			reply.ParseMode = "HTML"
			reply.ReplyMarkup = CloseOnlyKeyboard(sess.Language)
			_, _ = r.bot.Send(reply)
			return
		}
		reply := tgbotapi.NewMessage(chatID, FormatArtifacts(sess.Language, items))
		reply.ParseMode = "HTML"
		reply.ReplyMarkup = ArtifactListKeyboard(items, sess.Language)
		_, _ = r.bot.Send(reply)

	case "/file":
		if args == "" {
			r.sendText(chatID, "Format: <code>/file &lt;relative_or_abs_path&gt;</code>")
			return
		}
		filePath := args
		if !filepath.IsAbs(filePath) {
			filePath = filepath.Join(sess.CWD, filePath)
		}
		r.handleSendFile(chatID, filePath)

	case "/plan":
		if args == "" {
			r.sendText(chatID, "Format: <code>/plan &lt;uraian tugas perencanaan&gt;</code>")
			return
		}
		r.executeAgentTurn(chatID, sess, engine.StreamRunOptions{
			UserID:         userID,
			Prompt:         args,
			CWD:            sess.CWD,
			ConversationID: sess.ActiveConversationID,
			PermissionMode: sess.PermissionMode,
			Mode:           "plan",
			Model:          sess.ActiveModel,
			Effort:         sess.ActiveEffort,
			PrintTimeout:   r.cfg.Agy.PrintTimeout,
		}, msg.MessageID)

	case "/goal":
		if args == "" {
			r.sendText(chatID, "Format: <code>/goal &lt;target autonomous task&gt;</code>")
			return
		}
		r.executeAgentTurn(chatID, sess, engine.StreamRunOptions{
			UserID:         userID,
			Prompt:         "/goal " + args,
			CWD:            sess.CWD,
			ConversationID: sess.ActiveConversationID,
			PermissionMode: sess.PermissionMode,
			Model:          sess.ActiveModel,
			Effort:         sess.ActiveEffort,
			PrintTimeout:   r.cfg.Agy.PrintTimeout,
			IsGoal:         true,
		}, msg.MessageID)

	case "/autodelete":
		if args != "" {
			argLower := strings.ToLower(args)
			if argLower == "off" || argLower == "disable" || argLower == "disabled" || argLower == "0" {
				r.sm.SetMaxTelegramTurns(userID, -1)
				reply := tgbotapi.NewMessage(chatID, i18n.T(sess.Language, "autodelete_disabled"))
				reply.ParseMode = "HTML"
				reply.ReplyMarkup = CloseOnlyKeyboard(sess.Language)
				_, _ = r.bot.Send(reply)
			} else if argLower == "clean" || argLower == "clear" {
				cleared := r.sm.ClearAllTrackedTurns(userID)
				if len(cleared) > 0 {
					go r.deleteTurnMessagesAsync(chatID, cleared)
					reply := tgbotapi.NewMessage(chatID, fmt.Sprintf(i18n.T(sess.Language, "autodelete_cleared"), len(cleared)*2))
					reply.ParseMode = "HTML"
					reply.ReplyMarkup = CloseOnlyKeyboard(sess.Language)
					_, _ = r.bot.Send(reply)
				} else {
					reply := tgbotapi.NewMessage(chatID, i18n.T(sess.Language, "autodelete_already_empty"))
					reply.ParseMode = "HTML"
					reply.ReplyMarkup = CloseOnlyKeyboard(sess.Language)
					_, _ = r.bot.Send(reply)
				}
			} else {
				var limit int
				if _, err := fmt.Sscanf(args, "%d", &limit); err == nil && limit > 0 {
					r.sm.SetMaxTelegramTurns(userID, limit)
					reply := tgbotapi.NewMessage(chatID, fmt.Sprintf(i18n.T(sess.Language, "autodelete_set"), limit))
					reply.ParseMode = "HTML"
					reply.ReplyMarkup = CloseOnlyKeyboard(sess.Language)
					_, _ = r.bot.Send(reply)
				} else {
					r.sendText(chatID, "Format: <code>/autodelete [20|50|100|off|clean]</code>")
				}
			}
		} else {
			reply := tgbotapi.NewMessage(chatID, i18n.GetAutoDeleteText(sess.Language, sess.MaxTelegramTurns, len(sess.TrackedTurns)))
			reply.ParseMode = "HTML"
			reply.ReplyMarkup = AutoDeleteKeyboard(sess.Language, sess.MaxTelegramTurns)
			_, _ = r.bot.Send(reply)
		}

	case "/lang", "/language":
		if args != "" {
			target := strings.ToLower(args)
			if target == "en" || target == "english" {
				r.sm.SetLanguage(userID, "en")
				reply := tgbotapi.NewMessage(chatID, i18n.T("en", "lang_changed"))
				reply.ParseMode = "HTML"
				reply.ReplyMarkup = CloseOnlyKeyboard("en")
				_, _ = r.bot.Send(reply)
			} else if target == "id" || target == "indonesia" || target == "indonesian" {
				r.sm.SetLanguage(userID, "id")
				reply := tgbotapi.NewMessage(chatID, i18n.T("id", "lang_changed"))
				reply.ParseMode = "HTML"
				reply.ReplyMarkup = CloseOnlyKeyboard("id")
				_, _ = r.bot.Send(reply)
			} else {
				r.sendText(chatID, "Format: <code>/lang id</code> (Bahasa Indonesia) atau <code>/lang en</code> (English)")
			}
		} else {
			reply := tgbotapi.NewMessage(chatID, i18n.GetLanguageMenuText(sess.Language))
			reply.ParseMode = "HTML"
			reply.ReplyMarkup = LanguageSelectionKeyboard(sess.Language)
			_, _ = r.bot.Send(reply)
		}

	default:
		// Forward any other custom slash command or skill to the agent
		r.executeAgentTurn(chatID, sess, engine.StreamRunOptions{
			UserID:         userID,
			Prompt:         text,
			CWD:            sess.CWD,
			ConversationID: sess.ActiveConversationID,
			PermissionMode: sess.PermissionMode,
			Model:          sess.ActiveModel,
			Effort:         sess.ActiveEffort,
			PrintTimeout:   r.cfg.Agy.PrintTimeout,
		}, msg.MessageID)
	}
}

func (r *Router) handleCallbackQuery(cb *tgbotapi.CallbackQuery) {
	data := cb.Data
	userID := cb.From.ID
	chatID := cb.Message.Chat.ID
	msgID := cb.Message.MessageID

	// Answer callback to remove loading clock
	callbackResp := tgbotapi.NewCallback(cb.ID, "")
	_, _ = r.bot.Request(callbackResp)

	if !r.cfg.IsUserAllowed(userID) {
		return
	}

	sess := r.sm.GetSession(userID, chatID)

	switch {
	case data == "cmd_delete_msg":
		delMsg := tgbotapi.NewDeleteMessage(chatID, msgID)
		_, _ = r.bot.Request(delMsg)

	case data == "cmd_cancel_active_task":
		if r.cancelActiveTask(userID, chatID) {
			edit := tgbotapi.NewEditMessageText(chatID, msgID, "🛑 <b>Tugas aktif berhasil dihentikan.</b> Anda dapat mengirim instruksi baru sekarang.")
			edit.ParseMode = "HTML"
			kb := CloseOnlyKeyboard(sess.Language)
			edit.ReplyMarkup = &kb
			_, _ = r.bot.Send(edit)
		} else {
			edit := tgbotapi.NewEditMessageText(chatID, msgID, "ℹ️ Tidak ada proses aktif yang sedang berjalan.")
			edit.ParseMode = "HTML"
			kb := CloseOnlyKeyboard(sess.Language)
			edit.ReplyMarkup = &kb
			_, _ = r.bot.Send(edit)
		}

	case data == "cmd_help_menu":
		edit := tgbotapi.NewEditMessageText(chatID, msgID, FormatHelp(sess.Language))
		edit.ParseMode = "HTML"
		kb := QuickActionKeyboard(sess.Language)
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case data == "cmd_resume_menu":
		convs := r.sm.GetAvailableConversations(userID)
		if len(convs) == 0 {
			edit := tgbotapi.NewEditMessageText(chatID, msgID, i18n.T(sess.Language, "no_conversation_history"))
			edit.ParseMode = "HTML"
			kb := CloseOnlyKeyboard(sess.Language)
			edit.ReplyMarkup = &kb
			_, _ = r.bot.Send(edit)
		} else {
			edit := tgbotapi.NewEditMessageText(chatID, msgID, "📂 <b>Pilih Sesi untuk Di-Resume (/resume):</b>\nSilakan pilih sesi di bawah untuk melanjutkan percakapan:")
			edit.ParseMode = "HTML"
			kb := ResumeKeyboard(convs, sess.ActiveConversationID, sess.Language)
			edit.ReplyMarkup = &kb
			_, _ = r.bot.Send(edit)
		}

	case data == "cmd_artifact_menu":
		items, err := artifact.ListArtifacts(sess.ActiveConversationID)
		if err != nil || len(items) == 0 {
			edit := tgbotapi.NewEditMessageText(chatID, msgID, i18n.T(sess.Language, "no_artifacts_found"))
			edit.ParseMode = "HTML"
			kb := CloseOnlyKeyboard(sess.Language)
			edit.ReplyMarkup = &kb
			_, _ = r.bot.Send(edit)
		} else {
			edit := tgbotapi.NewEditMessageText(chatID, msgID, FormatArtifacts(sess.Language, items))
			edit.ParseMode = "HTML"
			kb := ArtifactListKeyboard(items, sess.Language)
			edit.ReplyMarkup = &kb
			_, _ = r.bot.Send(edit)
		}

	case strings.HasPrefix(data, "art_select:"):
		artID := strings.TrimPrefix(data, "art_select:")
		items, _ := artifact.ListArtifacts(sess.ActiveConversationID)
		var found *artifact.Item
		for _, it := range items {
			if it.ID == artID {
				found = &it
				break
			}
		}
		if found == nil {
			r.sendText(chatID, "❌ Artifact tidak ditemukan.")
			return
		}
		edit := tgbotapi.NewEditMessageText(chatID, msgID, FormatArtifactDetail(sess.Language, *found))
		edit.ParseMode = "HTML"
		kb := ArtifactDetailKeyboard(*found, sess.Language)
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case strings.HasPrefix(data, "art_open:"):
		artID := strings.TrimPrefix(data, "art_open:")
		items, _ := artifact.ListArtifacts(sess.ActiveConversationID)
		var found *artifact.Item
		for _, it := range items {
			if it.ID == artID {
				found = &it
				break
			}
		}
		if found == nil {
			r.sendText(chatID, "❌ Artifact tidak ditemukan.")
			return
		}
		content, err := os.ReadFile(found.Path)
		if err != nil {
			r.sendText(chatID, fmt.Sprintf("❌ Gagal membaca file artifact: %v", err))
			return
		}
		strContent := string(content)
		if len(strContent) > 3500 {
			strContent = strContent[:3500] + "\n\n...<i>(konten dipotong, gunakan tombol 'Unduh Dokumen' untuk melihat seluruh file)</i>"
		}
		reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("📖 <b>Isi Dokumen: %s</b>\n\n%s", renderer.EscapeHTML(found.FileName), renderer.FormatMarkdownForTelegram(strContent)))
		reply.ParseMode = "HTML"
		kb := ArtifactDetailKeyboard(*found, sess.Language)
		reply.ReplyMarkup = &kb
		_, _ = r.bot.Send(reply)

	case strings.HasPrefix(data, "art_download:"):
		artID := strings.TrimPrefix(data, "art_download:")
		items, _ := artifact.ListArtifacts(sess.ActiveConversationID)
		var found *artifact.Item
		for _, it := range items {
			if it.ID == artID {
				found = &it
				break
			}
		}
		if found == nil {
			r.sendText(chatID, "❌ Artifact tidak ditemukan.")
			return
		}
		doc := tgbotapi.NewDocument(chatID, tgbotapi.FilePath(found.Path))
		doc.Caption = fmt.Sprintf("📑 Artifact: %s (%.1f KB)", found.FileName, float64(found.SizeBytes)/1024.0)
		_, _ = r.bot.Send(doc)

	case strings.HasPrefix(data, "art_approve:"):
		artID := strings.TrimPrefix(data, "art_approve:")
		r.sendText(chatID, fmt.Sprintf("✅ <b>Artifact Disetujui:</b> <code>%s</code>\n🚀 Melanjutkan eksekusi rencana...", artID))
		prompt := fmt.Sprintf("Saya telah meninjau dan menyetujui artifact '%s'. Silakan lanjutkan ke langkah implementasi dan eksekusi selanjutnya secara bertahap.", artID)
		r.executeAgentTurn(chatID, sess, engine.StreamRunOptions{
			UserID:         userID,
			Prompt:         prompt,
			CWD:            sess.CWD,
			ConversationID: sess.ActiveConversationID,
			PermissionMode: sess.PermissionMode,
			Model:          sess.ActiveModel,
			Effort:         sess.ActiveEffort,
		}, 0)

	case strings.HasPrefix(data, "art_reject:"):
		artID := strings.TrimPrefix(data, "art_reject:")
		r.sendText(chatID, fmt.Sprintf("❌ <b>Artifact Ditolak / Meminta Revisi:</b> <code>%s</code>\nSilakan berikan instruksi revisi atau agen akan meninjau ulang alternatif rencana ini.", artID))
		prompt := fmt.Sprintf("Saya menolak artifact/rencana '%s'. Tolong tinjau kembali kekurangan rencana tersebut dan buat revisi alternatif yang lebih baik.", artID)
		r.executeAgentTurn(chatID, sess, engine.StreamRunOptions{
			UserID:         userID,
			Prompt:         prompt,
			CWD:            sess.CWD,
			ConversationID: sess.ActiveConversationID,
			PermissionMode: sess.PermissionMode,
			Model:          sess.ActiveModel,
			Effort:         sess.ActiveEffort,
		}, 0)

	case strings.HasPrefix(data, "resume_id:"):
		convID := strings.TrimPrefix(data, "resume_id:")
		r.sm.UpdateConversation(userID, convID, "")

		// Check if we know the workspace and title for this conversation
		convs := r.sm.GetAvailableConversations(userID)
		title := "Percakapan"
		wsPath := sess.CWD
		timeLabel := ""
		for _, c := range convs {
			if c.ID == convID {
				if c.Title != "" {
					title = c.Title
				}
				timeLabel = c.TimeLabel
				if c.Workspace != "" {
					wsPath = c.Workspace
					if info, err := os.Stat(wsPath); err == nil && info.IsDir() {
						r.sm.UpdateCWD(userID, wsPath)
					}
				}
				break
			}
		}

		timeLine := ""
		if timeLabel != "" {
			timeLine = fmt.Sprintf("• <b>Waktu</b>: %s\n", timeLabel)
		}

		edit := tgbotapi.NewEditMessageText(chatID, msgID, fmt.Sprintf(
			"✅ <b>Sesi Obrolan Berhasil Di-Resume!</b>\n\n"+
				"• <b>Topik</b>: %s\n"+
				"• <b>Workspace</b>: <code>%s</code>\n"+
				"%s\n"+
				"💬 <i>Silakan langsung kirim pesan apa saja di chat untuk melanjutkan percakapan ini.</i>",
			renderer.EscapeHTML(title), wsPath, timeLine,
		))
		edit.ParseMode = "HTML"
		kb := ResumeConfirmedKeyboard(convID, sess.Language)
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case data == "cmd_usage":
		r.executeOneShotInPlace(chatID, msgID, sess.CWD, "📊 Model Quota & Limit", "/usage", "cmd_usage", sess.Language)

	case data == "cmd_credits":
		r.executeOneShotInPlace(chatID, msgID, sess.CWD, "💰 G1 Credits", "/credits", "cmd_credits", sess.Language)

	case data == "cmd_skills":
		r.executeOneShotInPlace(chatID, msgID, sess.CWD, "🧰 Available Skills", "/skills", "cmd_skills", sess.Language)

	case data == "cmd_status":
		task := r.getActiveTask(userID)
		isRunning := task != nil
		taskDesc := ""
		if isRunning {
			dur := time.Since(task.StartedAt).Round(time.Second)
			taskDesc = fmt.Sprintf("%s (%s)", task.Prompt, dur.String())
		}
		toast := tgbotapi.NewCallback(cb.ID, i18n.T(sess.Language, "status_refreshed"))
		_, _ = r.bot.Request(toast)
		edit := tgbotapi.NewEditMessageText(chatID, msgID, FormatStatus(sess.Language, sess, isRunning, taskDesc))
		edit.ParseMode = "HTML"
		kb := StatusActionKeyboard(sess.Language)
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case data == "cmd_ls_cwd":
		toast := tgbotapi.NewCallback(cb.ID, "📁 "+sess.CWD)
		_, _ = r.bot.Request(toast)
		r.handleListDir(chatID, sess.CWD, sess.Language)

	case data == "cmd_autodelete_menu":
		edit := tgbotapi.NewEditMessageText(chatID, msgID, i18n.GetAutoDeleteText(sess.Language, sess.MaxTelegramTurns, len(sess.TrackedTurns)))
		edit.ParseMode = "HTML"
		kb := AutoDeleteKeyboard(sess.Language, sess.MaxTelegramTurns)
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case strings.HasPrefix(data, "set_autodelete:"):
		valStr := strings.TrimPrefix(data, "set_autodelete:")
		var limit int
		_, _ = fmt.Sscanf(valStr, "%d", &limit)
		if limit <= 0 {
			r.sm.SetMaxTelegramTurns(userID, -1)
			toast := tgbotapi.NewCallback(cb.ID, i18n.T(sess.Language, "autodelete_disabled"))
			_, _ = r.bot.Request(toast)
		} else {
			r.sm.SetMaxTelegramTurns(userID, limit)
			toast := tgbotapi.NewCallback(cb.ID, fmt.Sprintf(i18n.T(sess.Language, "autodelete_set"), limit))
			_, _ = r.bot.Request(toast)
		}
		sess = r.sm.GetSession(userID, chatID)
		edit := tgbotapi.NewEditMessageText(chatID, msgID, i18n.GetAutoDeleteText(sess.Language, sess.MaxTelegramTurns, len(sess.TrackedTurns)))
		edit.ParseMode = "HTML"
		kb := AutoDeleteKeyboard(sess.Language, sess.MaxTelegramTurns)
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case data == "cmd_clean_chat_now":
		cleared := r.sm.ClearAllTrackedTurns(userID)
		if len(cleared) > 0 {
			go r.deleteTurnMessagesAsync(chatID, cleared)
			toast := tgbotapi.NewCallback(cb.ID, fmt.Sprintf(i18n.T(sess.Language, "autodelete_cleared"), len(cleared)*2))
			_, _ = r.bot.Request(toast)
		} else {
			toast := tgbotapi.NewCallback(cb.ID, i18n.T(sess.Language, "autodelete_already_empty"))
			_, _ = r.bot.Request(toast)
		}
		sess = r.sm.GetSession(userID, chatID)
		edit := tgbotapi.NewEditMessageText(chatID, msgID, i18n.GetAutoDeleteText(sess.Language, sess.MaxTelegramTurns, len(sess.TrackedTurns)))
		edit.ParseMode = "HTML"
		kb := AutoDeleteKeyboard(sess.Language, sess.MaxTelegramTurns)
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case data == "cmd_lang_menu":
		edit := tgbotapi.NewEditMessageText(chatID, msgID, i18n.GetLanguageMenuText(sess.Language))
		edit.ParseMode = "HTML"
		kb := LanguageSelectionKeyboard(sess.Language)
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case strings.HasPrefix(data, "set_lang:"):
		newLang := strings.TrimPrefix(data, "set_lang:")
		r.sm.SetLanguage(userID, newLang)
		sess = r.sm.GetSession(userID, chatID)
		toast := tgbotapi.NewCallback(cb.ID, i18n.T(sess.Language, "lang_changed"))
		_, _ = r.bot.Request(toast)
		edit := tgbotapi.NewEditMessageText(chatID, msgID, i18n.GetLanguageMenuText(sess.Language))
		edit.ParseMode = "HTML"
		kb := LanguageSelectionKeyboard(sess.Language)
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case data == "cmd_new":
		r.sm.ResetConversation(userID)
		edit := tgbotapi.NewEditMessageText(chatID, msgID, fmt.Sprintf("🔄 <b>Sesi Percakapan Direset</b>\nWorkspace aktif: <code>%s</code>", sess.CWD))
		edit.ParseMode = "HTML"
		kb := BackAndCloseKeyboard("cmd_help_menu", sess.Language)
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case data == "cmd_model_menu":
		active := sess.ActiveModel
		if active == "" {
			active = "(Default agy)"
		}
		edit := tgbotapi.NewEditMessageText(chatID, msgID, fmt.Sprintf("🧠 <b>Pilih Model Antigravity:</b>\nModel aktif saat ini: <code>%s</code>", active))
		edit.ParseMode = "HTML"
		kb := ModelSelectionKeyboard(sess.Language)
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case strings.HasPrefix(data, "set_model:"):
		modelName := strings.TrimPrefix(data, "set_model:")
		if modelName == "default" {
			r.sm.SetModel(userID, "")
			edit := tgbotapi.NewEditMessageText(chatID, msgID, "✅ <b>Active model direset ke Default (otomatis agy).</b>")
			edit.ParseMode = "HTML"
			kb := BackAndCloseKeyboard("cmd_model_menu", sess.Language)
			edit.ReplyMarkup = &kb
			_, _ = r.bot.Send(edit)
		} else {
			r.sm.SetModel(userID, modelName)
			edit := tgbotapi.NewEditMessageText(chatID, msgID, fmt.Sprintf("✅ <b>Active model diatur ke:</b>\n<code>%s</code>", modelName))
			edit.ParseMode = "HTML"
			kb := BackAndCloseKeyboard("cmd_model_menu", sess.Language)
			edit.ReplyMarkup = &kb
			_, _ = r.bot.Send(edit)
		}

	case data == "cmd_effort_menu":
		active := sess.ActiveEffort
		if active == "" {
			active = "(Default agy)"
		}
		edit := tgbotapi.NewEditMessageText(chatID, msgID, fmt.Sprintf("⚡ <b>Pilih Reasoning Effort:</b>\nEffort aktif saat ini: <b>%s</b>", active))
		edit.ParseMode = "HTML"
		kb := EffortSelectionKeyboard(sess.Language)
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case strings.HasPrefix(data, "set_effort:"):
		effort := strings.TrimPrefix(data, "set_effort:")
		r.sm.SetEffort(userID, effort)
		edit := tgbotapi.NewEditMessageText(chatID, msgID, fmt.Sprintf("✅ <b>Reasoning effort diatur ke:</b>\n<b>%s</b>", effort))
		edit.ParseMode = "HTML"
		kb := BackAndCloseKeyboard("cmd_effort_menu", sess.Language)
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case data == "cmd_perm_menu":
		edit := tgbotapi.NewEditMessageText(chatID, msgID, fmt.Sprintf("🔒 <b>Pilih Mode Persetujuan Tools:</b>\nSaat ini: <b>%s</b>\n\n• <b>Auto-Approve</b>: Aksi disetujui otomatis tanpa menunggu.\n• <b>Ask User</b>: Konfirmasi manual tiap aksi.", sess.PermissionMode))
		edit.ParseMode = "HTML"
		kb := PermissionSelectionKeyboard(sess.Language)
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case strings.HasPrefix(data, "set_perm:"):
		mode := strings.TrimPrefix(data, "set_perm:")
		r.sm.SetPermissionMode(userID, mode)
		edit := tgbotapi.NewEditMessageText(chatID, msgID, fmt.Sprintf("✅ <b>Mode permission diatur ke:</b>\n<b>%s</b>", mode))
		edit.ParseMode = "HTML"
		kb := BackAndCloseKeyboard("cmd_perm_menu", sess.Language)
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)
	}
}

func (r *Router) deleteTurnMessagesAsync(chatID int64, entries []session.TurnMessageEntry) {
	for _, e := range entries {
		if e.BotMsgID > 0 {
			delBot := tgbotapi.NewDeleteMessage(chatID, e.BotMsgID)
			_, _ = r.bot.Request(delBot)
		}
		if e.UserMsgID > 0 {
			delUser := tgbotapi.NewDeleteMessage(chatID, e.UserMsgID)
			_, _ = r.bot.Request(delUser)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (r *Router) executeOneShot(chatID int64, cwd string, title string, command string, lang string) {
	ctx := context.Background()

	// Show typing status while command is running
	stopTyping := make(chan struct{})
	go func() {
		_, _ = r.bot.Send(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping))
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopTyping:
				return
			case <-ticker.C:
				_, _ = r.bot.Send(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping))
			}
		}
	}()
	defer close(stopTyping)

	loadingMsg, err := r.bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("⏳ Menjalankan <code>%s</code>...", command)))
	if err != nil {
		return
	}

	out, err := r.oneShot.Run(ctx, cwd, command)
	kb := CloseOnlyKeyboard(lang)
	if err != nil {
		edit := tgbotapi.NewEditMessageText(chatID, loadingMsg.MessageID, fmt.Sprintf("❌ Error menjalankan <code>%s</code>:\n<pre>%s</pre>", command, renderer.EscapeHTML(err.Error())))
		edit.ParseMode = "HTML"
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)
		return
	}

	if out == "" {
		out = "(Output kosong)"
	}

	edit := tgbotapi.NewEditMessageText(chatID, loadingMsg.MessageID, FormatCodeBlock(title, out))
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &kb
	_, _ = r.bot.Send(edit)
}

func (r *Router) executeOneShotInPlace(chatID int64, messageID int, cwd string, title string, command string, refreshCmd string, lang string) {
	ctx := context.Background()

	// Show typing status while command is running
	stopTyping := make(chan struct{})
	go func() {
		_, _ = r.bot.Send(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping))
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopTyping:
				return
			case <-ticker.C:
				_, _ = r.bot.Send(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping))
			}
		}
	}()
	defer close(stopTyping)

	loadingEdit := tgbotapi.NewEditMessageText(chatID, messageID, fmt.Sprintf("⏳ Menjalankan <code>%s</code>...", command))
	loadingEdit.ParseMode = "HTML"
	_, _ = r.bot.Send(loadingEdit)

	out, err := r.oneShot.Run(ctx, cwd, command)
	kb := RefreshAndBackKeyboard(refreshCmd, "cmd_help_menu", lang)
	if err != nil {
		edit := tgbotapi.NewEditMessageText(chatID, messageID, fmt.Sprintf("❌ Error menjalankan <code>%s</code>:\n<pre>%s</pre>", command, renderer.EscapeHTML(err.Error())))
		edit.ParseMode = "HTML"
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)
		return
	}

	if out == "" {
		out = "(Output kosong)"
	}

	edit := tgbotapi.NewEditMessageText(chatID, messageID, FormatCodeBlock(title, out))
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &kb
	_, _ = r.bot.Send(edit)
}

func (r *Router) executeAgentTurn(chatID int64, sess *session.UserSession, opts engine.StreamRunOptions, userMsgID int) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if opts.PrintTimeout == "" {
		opts.PrintTimeout = r.cfg.Agy.PrintTimeout
	}
	if opts.PrintTimeout == "" {
		opts.PrintTimeout = "24h"
	}

	r.registerActiveTask(opts.UserID, chatID, opts.Prompt, cancel, opts.IsGoal)
	defer r.unregisterActiveTask(opts.UserID)

	// Continuous typing action so user always sees "typing..." in chat header while agent works
	stopTyping := make(chan struct{})
	defer close(stopTyping)
	go func() {
		_, _ = r.bot.Send(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping))
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopTyping:
				return
			case <-ticker.C:
				_, _ = r.bot.Send(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping))
			}
		}
	}()

	maxTurns := 1
	if opts.IsGoal {
		maxTurns = 15
	}

	iteration := 1
	for iteration <= maxTurns {
		if ctx.Err() != nil {
			break
		}

		turnUserMsgID := 0
		if iteration == 1 {
			turnUserMsgID = userMsgID
		}

		result, err := r.runSingleStreamTurn(ctx, chatID, sess, opts, iteration, turnUserMsgID)
		if ctx.Err() != nil {
			break
		}
		if err != nil {
			break
		}

		if !opts.IsGoal {
			break
		}

		// Check if goal completed or cancelled
		if result != nil {
			resp := result.Response
			if strings.Contains(resp, "<!-- GOAL_COMPLETE -->") || strings.Contains(resp, "<!-- GOAL_CANCELLED -->") {
				break
			}
			if result.ConversationID != "" {
				opts.ConversationID = result.ConversationID
			}
		}

		iteration++
		if iteration > maxTurns {
			r.sendText(chatID, "ℹ️ <i>Batas maksimal iterasi /goal (15 turn) tercapai. Anda dapat mengetik /continue untuk melanjutkan jika masih diperlukan.</i>")
			break
		}

		// Notify user that next iteration starts automatically
		notice := fmt.Sprintf("🔄 <b>Goal belum selesai (Turn %d selesai). Melanjutkan iterasi otomatis ke-%d...</b>", iteration-1, iteration)
		r.sendText(chatID, notice)

		opts.Prompt = "Lanjutkan pengerjaan sasaran /goal hingga 100% selesai dan terverifikasi secara tuntas. Jika seluruh target telah tercapai, sertakan komentar <!-- GOAL_COMPLETE --> di akhir respon."
	}
}

func (r *Router) runSingleStreamTurn(ctx context.Context, chatID int64, sess *session.UserSession, opts engine.StreamRunOptions, iteration int, userMsgID int) (*engine.ResultPayload, error) {
	initialText := "💭 <i>Menganalisis instruksi...</i>"
	if sess.Language == "en" {
		initialText = "💭 <i>Analyzing instructions...</i>"
	}
	if opts.Mode == "plan" {
		if sess.Language == "en" {
			initialText = "📋 <i>Preparing plan (/plan)...</i>"
		} else {
			initialText = "📋 <i>Menyiapkan rencana (/plan)...</i>"
		}
	} else if opts.IsGoal {
		if iteration > 1 {
			if sess.Language == "en" {
				initialText = fmt.Sprintf("🎯 <i>Continuing goal (/goal) turn %d...</i>", iteration)
			} else {
				initialText = fmt.Sprintf("🎯 <i>Melanjutkan eksekusi sasaran (/goal) turn %d...</i>", iteration)
			}
		} else {
			if sess.Language == "en" {
				initialText = "🎯 <i>Starting autonomous goal (/goal)...</i>"
			} else {
				initialText = "🎯 <i>Memulai pengerjaan sasaran (/goal)...</i>"
			}
		}
	}

	activityTracker := throttler.NewActivityTracker(r.bot, chatID, initialText)

	var lastConvID string
	var recentActions []string
	var mu sync.Mutex

	var streamBuffer *throttler.MessageThrottler
	var responseStarted bool

	callbacks := engine.StreamCallbacks{
		OnInit: func(conversationID string, init *engine.StreamInitPayload) {
			lastConvID = conversationID
			if sess.ActiveConversationID != conversationID {
				r.sm.UpdateConversation(opts.UserID, conversationID, "")
			}
		},
		OnStepUpdate: func(step *engine.StepUpdatePayload) {
			if step == nil {
				return
			}
			badge := FormatActivityBadge(sess.Language, step)

			mu.Lock()
			if step.State == "DONE" && step.StepType == "tool" {
				dur := ""
				if step.DurationSeconds > 0 {
					dur = fmt.Sprintf(" (%.1fs)", step.DurationSeconds)
				}
				recentActions = append(recentActions, fmt.Sprintf("%s%s", badge, dur))
			}
			isStreaming := responseStarted
			mu.Unlock()

			if !isStreaming {
				activityTracker.Update(badge)
			}
		},
		OnDelta: func(delta string) {
			mu.Lock()
			if !responseStarted {
				responseStarted = true
				mu.Unlock()

				msgID := activityTracker.AdoptMessageID()
				if msgID == 0 {
					// Fallback: send fresh message if tracker message was unavailable
					formattedFirst := renderer.FormatMarkdownForTelegram(delta)
					if strings.TrimSpace(formattedFirst) == "" {
						formattedFirst = "..."
					}
					initResp := tgbotapi.NewMessage(chatID, formattedFirst)
					initResp.ParseMode = "HTML"
					sentMsg, err := r.bot.Send(initResp)
					if err == nil {
						msgID = sentMsg.MessageID
					} else {
						// Fallback to plain text if HTML parse error
						initPlain := tgbotapi.NewMessage(chatID, delta)
						if sentPlain, errPlain := r.bot.Send(initPlain); errPlain == nil {
							msgID = sentPlain.MessageID
						}
					}
				}

				newBuf := throttler.NewMessageThrottler(r.bot, chatID, msgID, r.cfg.Telegram.StreamEditIntervalMs)
				newBuf.Append(delta)

				mu.Lock()
				streamBuffer = newBuf
				mu.Unlock()
				return
			}
			buf := streamBuffer
			mu.Unlock()

			if buf != nil {
				buf.Append(delta)
			}
		},
		OnError: func(err error) {
			mu.Lock()
			buf := streamBuffer
			mu.Unlock()
			if buf != nil {
				buf.Append(fmt.Sprintf("\n\n❌ [Error]: %v", err))
			}
		},
	}

	result, err := r.streamRunner.RunStream(ctx, opts, callbacks)

	footer := ""
	finalText := ""
	if result != nil {
		footer = FormatResultFooter(result)
		finalText = result.Response
		if result.ConversationID != "" {
			lastConvID = result.ConversationID
			r.sm.UpdateConversation(opts.UserID, lastConvID, "")
		}

		if result.Error != "" {
			errPrefix := i18n.T(sess.Language, "err_occurred")
			if strings.TrimSpace(finalText) == "" {
				finalText = fmt.Sprintf("❌ <b>%s</b>\n%s", errPrefix, renderer.EscapeHTML(result.Error))
			} else {
				finalText = fmt.Sprintf("%s\n\n⚠️ <i>Peringatan / Error: %s</i>", finalText, renderer.EscapeHTML(result.Error))
			}
		}
	} else if err != nil {
		if ctx.Err() != nil {
			if sess.Language == "en" {
				finalText = "🛑 <b>Task stopped by user.</b>"
			} else {
				finalText = "🛑 <b>Tugas dihentikan oleh pengguna.</b>"
			}
		} else {
			errPrefix := i18n.T(sess.Language, "err_occurred")
			finalText = fmt.Sprintf("❌ <b>%s</b>\n%s", errPrefix, renderer.EscapeHTML(err.Error()))
		}
	}

	mu.Lock()
	started := responseStarted
	savedBuffer := streamBuffer
	actions := make([]string, len(recentActions))
	copy(actions, recentActions)
	mu.Unlock()

	var finalBotMsgID int
	if started && savedBuffer != nil {
		activityTracker.Delete()
		savedBuffer.Finalize(finalText, footer, actions)
		finalBotMsgID = savedBuffer.MessageID()
	} else {
		text := finalText
		if text == "" {
			if len(actions) > 0 {
				var sb strings.Builder
				if sess.Language == "en" {
					sb.WriteString("✅ <b>Task execution completed.</b>\n\n<i>Activity summary:</i>\n")
				} else {
					sb.WriteString("✅ <b>Tugas selesai dieksekusi.</b>\n\n<i>Ringkasan:</i>\n")
				}
				for _, a := range actions {
					sb.WriteString(fmt.Sprintf("• %s\n", a))
				}
				text = sb.String()
			} else {
				if sess.Language == "en" {
					text = "✅ <b>Task completed.</b>"
				} else {
					text = "✅ <b>Tugas selesai.</b>"
				}
			}
		}
		formatted := renderer.FormatMarkdownForTelegram(text)
		if footer != "" {
			formatted = formatted + "\n\n" + footer
		}

		// Edit the activity message in-place if available, transforming it directly into the final answer!
		edited := false
		trackerMsgID := activityTracker.MessageID()
		if trackerMsgID != 0 {
			edit := tgbotapi.NewEditMessageText(chatID, trackerMsgID, formatted)
			edit.ParseMode = "HTML"
			if _, sendErr := r.bot.Send(edit); sendErr == nil {
				edited = true
				finalBotMsgID = trackerMsgID
			} else {
				editPlain := tgbotapi.NewEditMessageText(chatID, trackerMsgID, renderer.StripHTML(formatted))
				if _, sendPlain := r.bot.Send(editPlain); sendPlain == nil {
					edited = true
					finalBotMsgID = trackerMsgID
				}
			}
		}

		if !edited {
			activityTracker.Delete()
			msg := tgbotapi.NewMessage(chatID, formatted)
			msg.ParseMode = "HTML"
			if sentMsg, sendErr := r.bot.Send(msg); sendErr == nil {
				finalBotMsgID = sentMsg.MessageID
			} else {
				plain := renderer.StripHTML(formatted)
				msgPlain := tgbotapi.NewMessage(chatID, plain)
				if sentPlain, errPlain := r.bot.Send(msgPlain); errPlain == nil {
					finalBotMsgID = sentPlain.MessageID
				}
			}
		}
	}

	// Auto-delete turn tracking: record turn and evict expired messages in Telegram
	if finalBotMsgID > 0 {
		toDelete := r.sm.RecordTurnMessages(opts.UserID, userMsgID, finalBotMsgID)
		if len(toDelete) > 0 {
			go r.deleteTurnMessagesAsync(chatID, toDelete)
		}
	}

	return result, err
}

func (r *Router) handleListDir(chatID int64, targetPath string, lang string) {
	entries, err := os.ReadDir(targetPath)
	if err != nil {
		r.sendText(chatID, fmt.Sprintf("❌ Gagal membaca direktori: <code>%s</code> (%v)", targetPath, err))
		return
	}

	var sb strings.Builder
	if lang == "en" {
		sb.WriteString(fmt.Sprintf("📂 <b>Directory Contents:</b> <code>%s</code>\n\n", targetPath))
	} else {
		sb.WriteString(fmt.Sprintf("📂 <b>Isi Direktori:</b> <code>%s</code>\n\n", targetPath))
	}

	count := 0
	for _, e := range entries {
		if count >= 40 {
			if lang == "en" {
				sb.WriteString(fmt.Sprintf("<i>... and %d other files/folders</i>\n", len(entries)-count))
			} else {
				sb.WriteString(fmt.Sprintf("<i>... dan %d file/folder lainnya</i>\n", len(entries)-count))
			}
			break
		}

		icon := "📄"
		if e.IsDir() {
			icon = "📁"
		}
		sb.WriteString(fmt.Sprintf("%s <code>%s</code>\n", icon, e.Name()))
		count++
	}

	if len(entries) == 0 {
		if lang == "en" {
			sb.WriteString("<i>(Directory is empty)</i>\n")
		} else {
			sb.WriteString("<i>(Direktori kosong)</i>\n")
		}
	}

	reply := tgbotapi.NewMessage(chatID, sb.String())
	reply.ParseMode = "HTML"
	reply.ReplyMarkup = CloseOnlyKeyboard(lang)
	_, _ = r.bot.Send(reply)
}

func (r *Router) handleSendFile(chatID int64, filePath string) {
	info, err := os.Stat(filePath)
	if err != nil {
		r.sendText(chatID, fmt.Sprintf("❌ File tidak ditemukan: <code>%s</code>", filePath))
		return
	}
	if info.IsDir() {
		r.sendText(chatID, fmt.Sprintf("❌ Path adalah direktori, bukan file: <code>%s</code>", filePath))
		return
	}
	if info.Size() > 50*1024*1024 {
		r.sendText(chatID, "❌ Ukuran file melebihi batas upload Telegram (50 MB).")
		return
	}

	doc := tgbotapi.NewDocument(chatID, tgbotapi.FilePath(filePath))
	doc.Caption = fmt.Sprintf("📄 %s (%d bytes)", filepath.Base(filePath), info.Size())
	_, err = r.bot.Send(doc)
	if err != nil {
		r.sendText(chatID, fmt.Sprintf("❌ Gagal mengirim file: %v", err))
	}
}

func (r *Router) sendText(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	_, _ = r.bot.Send(msg)
}

