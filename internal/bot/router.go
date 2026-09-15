package bot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"


	"agy-tele/config"
	"agy-tele/internal/engine"
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
			r.sendText(chatID, fmt.Sprintf("✅ Active model diatur ke: <code>%s</code>", args))
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
				r.sendText(chatID, fmt.Sprintf("✅ Reasoning effort diatur ke: <b>%s</b>", effort))
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
				r.sendText(chatID, fmt.Sprintf("✅ Mode permission diatur ke: <b>%s</b>", mode))
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
			r.sendText(chatID, fmt.Sprintf("📁 Workspace CWD berhasil diubah ke:\n<code>%s</code>", newPath))
		} else {
			r.sendText(chatID, fmt.Sprintf("📁 <b>Direktori Kerja Saat Ini:</b>\n<code>%s</code>\n\nGunakan <code>/cwd &lt;path&gt;</code> untuk berpindah folder.", sess.CWD))
		}

	case "/pwd":
		r.sendText(chatID, fmt.Sprintf("📁 <b>Current Working Directory:</b>\n<code>%s</code>", sess.CWD))

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
		r.sendText(chatID, fmt.Sprintf("🔄 <b>Sesi Percakapan Direset</b>\nSiap memulai sesi percakapan baru di workspace:\n<code>%s</code>", sess.CWD))

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
		r.sendText(chatID, FormatSessions(sess.RecentConversations, sess.ActiveConversationID))

	case "/switch":
		if args == "" {
			r.sendText(chatID, "Format: <code>/switch &lt;conversation_id&gt;</code>\nGunakan <code>/sessions</code> untuk melihat daftar ID.")
			return
		}
		r.sm.UpdateConversation(userID, args, "")
		r.sendText(chatID, fmt.Sprintf("✅ Berhasil berpindah ke sesi: <code>%s</code>", args))

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

	// Answer callback to remove loading clock
	callbackResp := tgbotapi.NewCallback(cb.ID, "")
	_, _ = r.bot.Request(callbackResp)

	if !r.cfg.IsUserAllowed(userID) {
		return
	}

	sess := r.sm.GetSession(userID, chatID)

	switch {
	case data == "cmd_help_menu":
		edit := tgbotapi.NewEditMessageText(chatID, cb.Message.MessageID, FormatHelp())
		edit.ParseMode = "HTML"
		kb := QuickActionKeyboard()
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case data == "cmd_usage":
		r.executeOneShot(chatID, sess.CWD, "📊 Model Quota & Limit", "/usage")

	case data == "cmd_credits":
		r.executeOneShot(chatID, sess.CWD, "💰 G1 Credits", "/credits")

	case data == "cmd_skills":
		r.executeOneShot(chatID, sess.CWD, "🧰 Available Skills", "/skills")

	case data == "cmd_status":
		reply := tgbotapi.NewMessage(chatID, FormatStatus(sess, false))
		reply.ParseMode = "HTML"
		reply.ReplyMarkup = QuickActionKeyboard()
		_, _ = r.bot.Send(reply)

	case data == "cmd_new":
		r.sm.ResetConversation(userID)
		r.sendText(chatID, fmt.Sprintf("🔄 Sesi percakapan direset.\nWorkspace aktif: <code>%s</code>", sess.CWD))

	case data == "cmd_model_menu":
		edit := tgbotapi.NewEditMessageText(chatID, cb.Message.MessageID, "🧠 <b>Pilih Model Antigravity:</b>")
		edit.ParseMode = "HTML"
		kb := ModelSelectionKeyboard()
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case strings.HasPrefix(data, "set_model:"):
		modelName := strings.TrimPrefix(data, "set_model:")
		r.sm.SetModel(userID, modelName)
		r.sendText(chatID, fmt.Sprintf("✅ Active model diatur ke: <code>%s</code>", modelName))

	case data == "cmd_effort_menu":
		edit := tgbotapi.NewEditMessageText(chatID, cb.Message.MessageID, "⚡ <b>Pilih Reasoning Effort:</b>")
		edit.ParseMode = "HTML"
		kb := EffortSelectionKeyboard()
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case strings.HasPrefix(data, "set_effort:"):
		effort := strings.TrimPrefix(data, "set_effort:")
		r.sm.SetEffort(userID, effort)
		r.sendText(chatID, fmt.Sprintf("✅ Reasoning effort diatur ke: <b>%s</b>", effort))

	case data == "cmd_perm_menu":
		edit := tgbotapi.NewEditMessageText(chatID, cb.Message.MessageID, "🔒 <b>Pilih Mode Persetujuan Tools:</b>")
		edit.ParseMode = "HTML"
		kb := PermissionSelectionKeyboard()
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case strings.HasPrefix(data, "set_perm:"):
		mode := strings.TrimPrefix(data, "set_perm:")
		r.sm.SetPermissionMode(userID, mode)
		r.sendText(chatID, fmt.Sprintf("✅ Mode permission diatur ke: <b>%s</b>", mode))
	}
}

func (r *Router) executeOneShot(chatID int64, cwd string, title string, command string) {
	ctx := context.Background()
	loadingMsg, err := r.bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("⏳ Menjalankan <code>%s</code>...", command)))
	if err != nil {
		return
	}

	out, err := r.oneShot.Run(ctx, cwd, command)
	if err != nil {
		edit := tgbotapi.NewEditMessageText(chatID, loadingMsg.MessageID, fmt.Sprintf("❌ Error menjalankan <code>%s</code>:\n<pre>%s</pre>", command, EscapeHTML(err.Error())))
		edit.ParseMode = "HTML"
		_, _ = r.bot.Send(edit)
		return
	}

	if out == "" {
		out = "(Output kosong)"
	}

	edit := tgbotapi.NewEditMessageText(chatID, loadingMsg.MessageID, FormatCodeBlock(title, out))
	edit.ParseMode = "HTML"
	_, _ = r.bot.Send(edit)
}

func (r *Router) executeAgentTurn(chatID int64, sess *session.UserSession, opts engine.StreamRunOptions) {
	ctx := context.Background()

	initialText := "⏳ <b>Antigravity sedang berpikir...</b>"
	if opts.Mode == "plan" {
		initialText = "📋 <b>Menyiapkan rencana (/plan)...</b>"
	}

	initMsg := tgbotapi.NewMessage(chatID, initialText)
	initMsg.ParseMode = "HTML"
	sentMsg, err := r.bot.Send(initMsg)
	if err != nil {
		return
	}

	// Create throttled streamer
	streamBuffer := throttler.NewMessageThrottler(r.bot, chatID, sentMsg.MessageID, r.cfg.Telegram.StreamEditIntervalMs)

	var lastConvID string

	callbacks := engine.StreamCallbacks{
		OnInit: func(conversationID string, init *engine.StreamInitPayload) {
			lastConvID = conversationID
			if sess.ActiveConversationID != conversationID {
				r.sm.UpdateConversation(opts.UserID, conversationID, "")
			}
		},
		OnDelta: func(delta string) {
			streamBuffer.Append(delta)
		},
		OnError: func(err error) {
			streamBuffer.Append(fmt.Sprintf("\n\n❌ [Error]: %v", err))
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

	streamBuffer.Finalize(finalText, footer)
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

	r.sendText(chatID, sb.String())
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
