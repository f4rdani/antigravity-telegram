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
	"agy-tele/internal/engine"
	"agy-tele/internal/renderer"
	"agy-tele/internal/session"
	"agy-tele/internal/throttler"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Router struct {
	cfg          *config.Config
	bot          *tgbotapi.BotAPI
	sm           *session.SessionManager
	oneShot      *engine.OneShotRunner
	streamRunner *engine.StreamAgentRunner
}

func NewRouter(cfg *config.Config, bot *tgbotapi.BotAPI, sm *session.SessionManager) *Router {
	return &Router{
		cfg:          cfg,
		bot:          bot,
		sm:           sm,
		oneShot:      engine.NewOneShotRunner(cfg.Agy.BinaryPath),
		streamRunner: engine.NewStreamAgentRunner(cfg.Agy.BinaryPath),
	}
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

	// Security: Whitelist check
	if !r.cfg.IsUserAllowed(userID) {
		reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("⛔ <b>Akses Ditolak</b>\nUser ID <code>%d</code> tidak terdaftar dalam whitelist bot.", userID))
		reply.ParseMode = "HTML"
		_, _ = r.bot.Send(reply)
		return
	}

	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}

	sess := r.sm.GetSession(userID, chatID)

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
	})
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
		reply := tgbotapi.NewMessage(chatID, FormatHelp())
		reply.ParseMode = "HTML"
		reply.ReplyMarkup = QuickActionKeyboard()
		_, _ = r.bot.Send(reply)

	case "/usage", "/quota":
		r.executeOneShot(chatID, sess.CWD, "📊 Model Quota & Limit", "/usage")

	case "/credits":
		r.executeOneShot(chatID, sess.CWD, "💰 G1 Credits", "/credits")

	case "/skills":
		r.executeOneShot(chatID, sess.CWD, "🧰 Available Skills", "/skills")

	case "/agents":
		r.executeOneShot(chatID, sess.CWD, "🤖 Custom Agents", "/agents")

	case "/changelog":
		r.executeOneShot(chatID, sess.CWD, "📝 Changelog", "/changelog")

	case "/status":
		reply := tgbotapi.NewMessage(chatID, FormatStatus(sess, false))
		reply.ParseMode = "HTML"
		reply.ReplyMarkup = QuickActionKeyboard()
		_, _ = r.bot.Send(reply)

	case "/model":
		if args != "" {
			r.sm.SetModel(userID, args)
			reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Active model diatur ke: <code>%s</code>", args))
			reply.ParseMode = "HTML"
			reply.ReplyMarkup = CloseOnlyKeyboard()
			_, _ = r.bot.Send(reply)
		} else {
			active := sess.ActiveModel
			if active == "" {
				active = "(Default agy)"
			}
			reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("🧠 <b>Pilih Model Antigravity</b>\nModel aktif saat ini: <code>%s</code>", active))
			reply.ParseMode = "HTML"
			reply.ReplyMarkup = ModelSelectionKeyboard()
			_, _ = r.bot.Send(reply)
		}

	case "/effort":
		if args != "" {
			effort := strings.ToLower(args)
			if effort == "low" || effort == "medium" || effort == "high" {
				r.sm.SetEffort(userID, effort)
				reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Reasoning effort diatur ke: <b>%s</b>", effort))
				reply.ParseMode = "HTML"
				reply.ReplyMarkup = CloseOnlyKeyboard()
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
			reply.ReplyMarkup = EffortSelectionKeyboard()
			_, _ = r.bot.Send(reply)
		}

	case "/permission", "/perm":
		if args != "" {
			mode := strings.ToLower(args)
			if mode == "auto" || mode == "ask" {
				r.sm.SetPermissionMode(userID, mode)
				reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Mode permission diatur ke: <b>%s</b>", mode))
				reply.ParseMode = "HTML"
				reply.ReplyMarkup = CloseOnlyKeyboard()
				_, _ = r.bot.Send(reply)
			} else {
				r.sendText(chatID, "Pilihan valid: <code>/permission auto</code> atau <code>/permission ask</code>")
			}
		} else {
			reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("🔒 <b>Mode Persetujuan Tools</b>\nSaat ini: <b>%s</b>\n\n• <b>Auto-Approve</b>: Aksi & perubahan file disetujui otomatis.\n• <b>Ask User</b>: Konfirmasi manual sebelum eksekusi.", sess.PermissionMode))
			reply.ParseMode = "HTML"
			reply.ReplyMarkup = PermissionSelectionKeyboard()
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
			reply.ReplyMarkup = CloseOnlyKeyboard()
			_, _ = r.bot.Send(reply)
		} else {
			reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("📁 <b>Direktori Kerja Saat Ini:</b>\n<code>%s</code>\n\nGunakan <code>/cwd &lt;path&gt;</code> untuk berpindah folder.", sess.CWD))
			reply.ParseMode = "HTML"
			reply.ReplyMarkup = CloseOnlyKeyboard()
			_, _ = r.bot.Send(reply)
		}

	case "/pwd":
		reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("📁 <b>Current Working Directory:</b>\n<code>%s</code>", sess.CWD))
		reply.ParseMode = "HTML"
		reply.ReplyMarkup = CloseOnlyKeyboard()
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
		r.handleListDir(chatID, targetPath)

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
		reply.ReplyMarkup = CloseOnlyKeyboard()
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
			kb := ResumeConfirmedKeyboard(convID)
			reply.ReplyMarkup = &kb
			_, _ = r.bot.Send(reply)
			return
		}

		convs := r.sm.GetAvailableConversations(userID)
		if len(convs) == 0 {
			reply := tgbotapi.NewMessage(chatID, "📂 <b>Tidak ada riwayat sesi percakapan ditemukan.</b>\nKirim pesan baru untuk memulai percakapan.")
			reply.ParseMode = "HTML"
			reply.ReplyMarkup = CloseOnlyKeyboard()
			_, _ = r.bot.Send(reply)
			return
		}
		reply := tgbotapi.NewMessage(chatID, "📂 <b>Pilih Sesi untuk Di-Resume (/resume):</b>\nSilakan pilih salah satu sesi di bawah:")
		reply.ParseMode = "HTML"
		reply.ReplyMarkup = ResumeKeyboard(convs, sess.ActiveConversationID)
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
		})

	case "/sessions":
		convs := r.sm.GetAvailableConversations(userID)
		reply := tgbotapi.NewMessage(chatID, FormatSessions(convs, sess.ActiveConversationID))
		reply.ParseMode = "HTML"
		reply.ReplyMarkup = ResumeKeyboard(convs, sess.ActiveConversationID)
		_, _ = r.bot.Send(reply)

	case "/cancel", "/stop":
		if r.streamRunner.CancelActive(userID) {
			r.sendText(chatID, "🛑 Proses aktif berhasil dihentikan!")
		} else {
			r.sendText(chatID, "Tidak ada proses yang sedang aktif berjalan.")
		}

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
		})

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
		})

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
		})
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

	case data == "cmd_help_menu":
		edit := tgbotapi.NewEditMessageText(chatID, msgID, FormatHelp())
		edit.ParseMode = "HTML"
		kb := QuickActionKeyboard()
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case data == "cmd_resume_menu":
		convs := r.sm.GetAvailableConversations(userID)
		if len(convs) == 0 {
			edit := tgbotapi.NewEditMessageText(chatID, msgID, "📂 <b>Tidak ada riwayat sesi percakapan ditemukan.</b>")
			edit.ParseMode = "HTML"
			kb := CloseOnlyKeyboard()
			edit.ReplyMarkup = &kb
			_, _ = r.bot.Send(edit)
		} else {
			edit := tgbotapi.NewEditMessageText(chatID, msgID, "📂 <b>Pilih Sesi untuk Di-Resume (/resume):</b>\nSilakan pilih sesi di bawah untuk melanjutkan percakapan:")
			edit.ParseMode = "HTML"
			kb := ResumeKeyboard(convs, sess.ActiveConversationID)
			edit.ReplyMarkup = &kb
			_, _ = r.bot.Send(edit)
		}

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
		kb := ResumeConfirmedKeyboard(convID)
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case data == "cmd_usage":
		r.executeOneShotInPlace(chatID, msgID, sess.CWD, "📊 Model Quota & Limit", "/usage", "cmd_usage")

	case data == "cmd_credits":
		r.executeOneShotInPlace(chatID, msgID, sess.CWD, "💰 G1 Credits", "/credits", "cmd_credits")

	case data == "cmd_skills":
		r.executeOneShotInPlace(chatID, msgID, sess.CWD, "🧰 Available Skills", "/skills", "cmd_skills")

	case data == "cmd_status":
		edit := tgbotapi.NewEditMessageText(chatID, msgID, FormatStatus(sess, false))
		edit.ParseMode = "HTML"
		kb := QuickActionKeyboard()
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case data == "cmd_new":
		r.sm.ResetConversation(userID)
		edit := tgbotapi.NewEditMessageText(chatID, msgID, fmt.Sprintf("🔄 <b>Sesi Percakapan Direset</b>\nWorkspace aktif: <code>%s</code>", sess.CWD))
		edit.ParseMode = "HTML"
		kb := BackAndCloseKeyboard("cmd_help_menu")
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case data == "cmd_model_menu":
		active := sess.ActiveModel
		if active == "" {
			active = "(Default agy)"
		}
		edit := tgbotapi.NewEditMessageText(chatID, msgID, fmt.Sprintf("🧠 <b>Pilih Model Antigravity:</b>\nModel aktif saat ini: <code>%s</code>", active))
		edit.ParseMode = "HTML"
		kb := ModelSelectionKeyboard()
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case strings.HasPrefix(data, "set_model:"):
		modelName := strings.TrimPrefix(data, "set_model:")
		r.sm.SetModel(userID, modelName)
		edit := tgbotapi.NewEditMessageText(chatID, msgID, fmt.Sprintf("✅ <b>Active model diatur ke:</b>\n<code>%s</code>", modelName))
		edit.ParseMode = "HTML"
		kb := BackAndCloseKeyboard("cmd_model_menu")
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case data == "cmd_effort_menu":
		active := sess.ActiveEffort
		if active == "" {
			active = "(Default agy)"
		}
		edit := tgbotapi.NewEditMessageText(chatID, msgID, fmt.Sprintf("⚡ <b>Pilih Reasoning Effort:</b>\nEffort aktif saat ini: <b>%s</b>", active))
		edit.ParseMode = "HTML"
		kb := EffortSelectionKeyboard()
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case strings.HasPrefix(data, "set_effort:"):
		effort := strings.TrimPrefix(data, "set_effort:")
		r.sm.SetEffort(userID, effort)
		edit := tgbotapi.NewEditMessageText(chatID, msgID, fmt.Sprintf("✅ <b>Reasoning effort diatur ke:</b>\n<b>%s</b>", effort))
		edit.ParseMode = "HTML"
		kb := BackAndCloseKeyboard("cmd_effort_menu")
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case data == "cmd_perm_menu":
		edit := tgbotapi.NewEditMessageText(chatID, msgID, fmt.Sprintf("🔒 <b>Pilih Mode Persetujuan Tools:</b>\nSaat ini: <b>%s</b>\n\n• <b>Auto-Approve</b>: Aksi disetujui otomatis tanpa menunggu.\n• <b>Ask User</b>: Konfirmasi manual tiap aksi.", sess.PermissionMode))
		edit.ParseMode = "HTML"
		kb := PermissionSelectionKeyboard()
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case strings.HasPrefix(data, "set_perm:"):
		mode := strings.TrimPrefix(data, "set_perm:")
		r.sm.SetPermissionMode(userID, mode)
		edit := tgbotapi.NewEditMessageText(chatID, msgID, fmt.Sprintf("✅ <b>Mode permission diatur ke:</b>\n<b>%s</b>", mode))
		edit.ParseMode = "HTML"
		kb := BackAndCloseKeyboard("cmd_perm_menu")
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)
	}
}

func (r *Router) executeOneShot(chatID int64, cwd string, title string, command string) {
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
	kb := CloseOnlyKeyboard()
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

func (r *Router) executeOneShotInPlace(chatID int64, messageID int, cwd string, title string, command string, refreshCmd string) {
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
	kb := RefreshAndBackKeyboard(refreshCmd, "cmd_help_menu")
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

func (r *Router) executeAgentTurn(chatID int64, sess *session.UserSession, opts engine.StreamRunOptions) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	initialText := "💭 <i>Menganalisis instruksi...</i>"
	if opts.Mode == "plan" {
		initialText = "📋 <i>Menyiapkan rencana (/plan)...</i>"
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
			badge := FormatActivityBadge(step)

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

				activityTracker.Delete()

				formattedFirst := renderer.FormatMarkdownForTelegram(delta)
				if strings.TrimSpace(formattedFirst) == "" {
					formattedFirst = "..."
				}
				initResp := tgbotapi.NewMessage(chatID, formattedFirst)
				initResp.ParseMode = "HTML"
				sentMsg, err := r.bot.Send(initResp)
				msgID := 0
				if err == nil {
					msgID = sentMsg.MessageID
				} else {
					// Fallback to plain text if HTML parse error
					initPlain := tgbotapi.NewMessage(chatID, delta)
					if sentPlain, errPlain := r.bot.Send(initPlain); errPlain == nil {
						msgID = sentPlain.MessageID
					}
				}
				newBuf := throttler.NewMessageThrottler(r.bot, chatID, msgID, r.cfg.Telegram.StreamEditIntervalMs)

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
	} else if err != nil {
		finalText = fmt.Sprintf("❌ Terjadi kesalahan:\n%s", err.Error())
	}

	mu.Lock()
	started := responseStarted
	savedBuffer := streamBuffer
	actions := make([]string, len(recentActions))
	copy(actions, recentActions)
	mu.Unlock()

	if started && savedBuffer != nil {
		activityTracker.Delete()
		savedBuffer.Finalize(finalText, footer, actions)
	} else {
		text := finalText
		if text == "" {
			if len(actions) > 0 {
				var sb strings.Builder
				sb.WriteString("✅ <b>Tugas selesai dieksekusi.</b>\n\n<i>Ringkasan:</i>\n")
				for _, a := range actions {
					sb.WriteString(fmt.Sprintf("• %s\n", a))
				}
				text = sb.String()
			} else {
				text = "✅ <b>Tugas selesai.</b>"
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
			} else {
				editPlain := tgbotapi.NewEditMessageText(chatID, trackerMsgID, renderer.StripHTML(formatted))
				if _, sendPlain := r.bot.Send(editPlain); sendPlain == nil {
					edited = true
				}
			}
		}

		if !edited {
			activityTracker.Delete()
			msg := tgbotapi.NewMessage(chatID, formatted)
			msg.ParseMode = "HTML"
			_, sendErr := r.bot.Send(msg)
			if sendErr != nil {
				plain := renderer.StripHTML(formatted)
				msgPlain := tgbotapi.NewMessage(chatID, plain)
				_, _ = r.bot.Send(msgPlain)
			}
		}
	}
}

func (r *Router) handleListDir(chatID int64, targetPath string) {
	entries, err := os.ReadDir(targetPath)
	if err != nil {
		r.sendText(chatID, fmt.Sprintf("❌ Gagal membaca direktori: <code>%s</code> (%v)", targetPath, err))
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📂 <b>Isi Direktori:</b> <code>%s</code>\n\n", targetPath))

	count := 0
	for _, e := range entries {
		if count >= 40 {
			sb.WriteString(fmt.Sprintf("<i>... dan %d file/folder lainnya</i>\n", len(entries)-count))
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
		sb.WriteString("<i>(Direktori kosong)</i>\n")
	}

	reply := tgbotapi.NewMessage(chatID, sb.String())
	reply.ParseMode = "HTML"
	reply.ReplyMarkup = CloseOnlyKeyboard()
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
