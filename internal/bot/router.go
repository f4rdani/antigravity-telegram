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
	"agy-tele/internal/auth"
	"agy-tele/internal/engine"
	"agy-tele/internal/i18n"
	"agy-tele/internal/renderer"
	"agy-tele/internal/session"
	"agy-tele/internal/throttler"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type ActiveTask struct {
	UserID          int64
	ChatID          int64
	Prompt          string
	StartedAt       time.Time
	Cancel          context.CancelFunc
	IsGoal          bool
	LastWarningTime time.Time
}

type Router struct {
	cfg          *config.Config
	bot          *tgbotapi.BotAPI
	sm           *session.SessionManager
	oneShot      *engine.OneShotRunner
	streamRunner *engine.StreamAgentRunner
	accumulator  *MessageAccumulator
	authMgr      *auth.AuthManager
	taskMu       sync.Mutex
	activeTasks  map[int64]*ActiveTask
	queueMu      sync.Mutex
	// pendingQueues holds FIFO queued agent turns per user while a task is running.
	// Queue is explicit: every message received during a run is either executed
	// immediately (instant commands) or enqueued with a position number — never silently dropped.
	pendingQueues map[int64][]*QueuedItem
	busyMu        sync.Mutex
	// busyCards tracks one dynamic live status card per user, edited in place
	// (duration ticks, positions shift, queued -> running -> done) so the
	// feedback never looks frozen/stuck.
	busyCards map[int64]*BusyCard
}

func NewRouter(cfg *config.Config, bot *tgbotapi.BotAPI, sm *session.SessionManager) *Router {
	return &Router{
		cfg:           cfg,
		bot:           bot,
		sm:            sm,
		oneShot:       engine.NewOneShotRunner(cfg.Agy.BinaryPath),
		streamRunner:  engine.NewStreamAgentRunner(cfg.Agy.BinaryPath),
		accumulator:   NewMessageAccumulator(),
		authMgr:       auth.NewAuthManager(cfg.Agy.BinaryPath),
		activeTasks:   make(map[int64]*ActiveTask),
		pendingQueues: make(map[int64][]*QueuedItem),
		busyCards:     make(map[int64]*BusyCard),
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
	r.accumulator.Cancel(userID)

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

	// Intercept auth code or cancel if user is currently in an active login session
	if r.authMgr.HasActiveSession(userID) {
		if text != "" && (strings.EqualFold(text, "/cancel") || strings.EqualFold(text, "/stop")) {
			r.authMgr.CancelLogin(userID)
			r.sendText(chatID, i18n.T(sess.Language, "auth_cancelled"))
			return
		}

		if text != "" && (!strings.HasPrefix(text, "/") || strings.HasPrefix(strings.ToLower(text), "/code")) {
			r.handleAuthCodeSubmission(chatID, userID, sess, text)
			return
		}
	}

	// Immediate /cancel or /stop interceptor (always allowed even if a task is running).
	// Keeps the explicit FIFO queue intact: current task stops, next queued item auto-starts.
	if text != "" && (strings.EqualFold(text, "/cancel") || strings.EqualFold(text, "/stop")) {
		qlen := r.queueLength(userID)
		if r.cancelActiveTask(userID, chatID) {
			if qlen > 0 {
				r.sendText(chatID, fmt.Sprintf("🛑 <b>Tugas aktif dihentikan.</b>\n⏭️ <i>Antrean %d pesan tetap tersimpan dan akan dijalankan otomatis berikutnya.</i>\n<i>Gunakan /clearqueue untuk menghapus antrean.</i>", qlen))
			} else {
				r.sendText(chatID, "🛑 <b>Proses aktif berhasil dihentikan!</b>")
			}
		} else {
			if qlen > 0 {
				r.sendQueueCard(chatID, sess)
			} else {
				r.sendText(chatID, "ℹ️ Tidak ada proses yang sedang aktif berjalan.")
			}
		}
		return
	}

	// Check if user is signed out when sending prompts
	if !auth.IsLoggedIn() {
		if text != "" && !IsInstantCommand(text) && !strings.HasPrefix(text, "/") {
			accounts, _ := auth.ListSavedAccounts()
			var reply tgbotapi.MessageConfig
			if sess.Language == "en" {
				reply = tgbotapi.NewMessage(chatID, "⚠️ <b>Not Logged In:</b>\nYou must be logged in with a Google account to use Antigravity CLI.\n\nPlease log in or select a saved account below:")
			} else {
				reply = tgbotapi.NewMessage(chatID, "⚠️ <b>Belum Login:</b>\nAnda harus login ke akun Google Antigravity terlebih dahulu untuk menggunakan bot ini.\n\nSilakan klik tombol di bawah untuk login atau pilih akun tersimpan:")
			}
			reply.ParseMode = "HTML"
			kb := AccountsKeyboard(accounts, "", sess.Language)
			reply.ReplyMarkup = &kb
			_, _ = r.bot.Send(reply)
			return
		}
	}

	// NOTE: No more "drop message while busy" guard here.
	// Queueable prompts always flow into the debounce accumulator below, even when
	// a task is running. The explicit FIFO queue decision happens in
	// dispatchAccumulatedMessage / tryEnqueueOrExecute, so split long messages
	// are still merged correctly and every follow-up gets a clear position number.

	// Check if message contains media/file attachments (Photo, Document, Video, Audio, Voice)
	// Media is downloaded immediately (so the file is not lost), then the analysis
	// prompt is either executed now or explicitly queued if a task is running.
	hasMedia := len(msg.Photo) > 0 || msg.Document != nil || msg.Video != nil || msg.Audio != nil || msg.Voice != nil
	if hasMedia {
		r.accumulator.Cancel(userID)
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

			r.tryEnqueueOrExecute(chatID, sess, engine.StreamRunOptions{
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

	// Check for instant commands (bypass debounce for zero latency on control commands)
	if IsInstantCommand(text) {
		r.accumulator.Cancel(userID)
		r.handleCommand(msg, sess, text)
		return
	}

	// Route text prompts and multi-part inputs through the debounce accumulator
	r.accumulator.Add(msg, sess, func(batch *AccumulatedBatch, combinedText string) {
		r.dispatchAccumulatedMessage(batch, combinedText)
	})
}

func formatNumber(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var res []string
	for len(s) > 3 {
		res = append([]string{s[len(s)-3:]}, res...)
		s = s[:len(s)-3]
	}
	if len(s) > 0 {
		res = append([]string{s}, res...)
	}
	return strings.Join(res, ",")
}

// IsQueueableAgentCommand reports whether a slash command should be queued
// (instead of executed instantly) when a task is already running.
// Instant control commands (/cancel, /status, /queue, /model, ...) always run immediately.
// Agent-turn commands (/plan, /goal, /continue, unknown skill commands) are queueable.
func IsQueueableAgentCommand(text string) bool {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "/") {
		return true // plain text prompt is always queueable
	}
	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return false
	}
	cmd := strings.ToLower(parts[0])
	switch cmd {
	case "/plan", "/goal", "/continue":
		return true
	case "/cancel", "/stop", "/status", "/start", "/help",
		"/model", "/effort", "/perm", "/permission", "/cwd", "/ls", "/pwd", "/clear",
		"/sessions", "/resume", "/switch", "/new", "/lang", "/language", "/usage",
		"/quota", "/credits", "/skills", "/agents", "/changelog",
		"/artifact", "/artifacts", "/file", "/autodelete",
		"/signout", "/logout", "/signin", "/login",
		"/accounts", "/account", "/whoami", "/code",
		"/queue", "/clearqueue", "/queuelist":
		return false
	default:
		return true // unknown slash / skill command -> forwarded to agent -> queueable
	}
}

func truncatePreview(s string, max int) string {
	t := strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(t) <= max {
		return t
	}
	return t[:max-3] + "..."
}

// tryEnqueueOrExecute executes the agent turn immediately when idle,
// or appends it to the explicit FIFO queue when busy.
// Returns true if queued, false if executed immediately.
func (r *Router) tryEnqueueOrExecute(chatID int64, sess *session.UserSession, opts engine.StreamRunOptions, userMsgIDs ...int) bool {
	r.ensureQueueInit()
	task := r.getActiveTask(opts.UserID)
	if task == nil {
		r.executeAgentTurn(chatID, sess, opts, userMsgIDs...)
		return false
	}
	item := &QueuedItem{
		ChatID:         chatID,
		UserID:         opts.UserID,
		Prompt:         opts.Prompt,
		Mode:           opts.Mode,
		IsGoal:         opts.IsGoal,
		CWD:            opts.CWD,
		ConversationID: opts.ConversationID,
		PermissionMode: opts.PermissionMode,
		Model:          opts.Model,
		Effort:         opts.Effort,
		PrintTimeout:   opts.PrintTimeout,
		UserMsgIDs:     append([]int(nil), userMsgIDs...),
	}
	pos := r.enqueue(opts.UserID, item)
	if pos < 0 {
		r.sendText(chatID, fmt.Sprintf("⚠️ <b>Antrean penuh (%d/%d).</b>\nTugas aktif masih berjalan. Tunggu hingga selesai atau gunakan /cancel, lalu kirim ulang pesan Anda.", MaxQueuePerUser, MaxQueuePerUser))
		return true
	}
	r.sendQueuedConfirmation(chatID, sess, item, pos)
	return true
}

func (r *Router) sendQueuedConfirmation(chatID int64, sess *session.UserSession, item *QueuedItem, pos int) {
	total := r.queueLength(item.UserID)
	hl := queuedHighlight(sess.Language, pos, total, truncatePreview(item.Prompt, 120))
	r.upsertBusyCard(chatID, item.UserID, sess.Language, hl)
}

func (r *Router) sendQueueCard(chatID int64, sess *session.UserSession) {
	items := r.listQueue(sess.UserID)
	task := r.getActiveTask(sess.UserID)
	reply := tgbotapi.NewMessage(chatID, FormatQueueList(sess.Language, task, items))
	reply.ParseMode = "HTML"
	kb := QueueActionKeyboard(sess.Language)
	reply.ReplyMarkup = &kb
	_, _ = r.bot.Send(reply)
}

func (r *Router) dispatchAccumulatedMessage(batch *AccumulatedBatch, combinedText string) {
	chatID := batch.ChatID
	userID := batch.UserID
	sess := batch.Session
	userMsgIDs := batch.MsgIDs
	isSplit := len(batch.Chunks) > 1

	// If split messages were stitched together, notify the user
	if isSplit {
		totalChars := len(combinedText)
		var notice string
		if sess.Language == "en" {
			notice = fmt.Sprintf("📥 <b>Long text detected (%d parts, %s characters).</b>\n⏳ <i>Merged successfully. Processing instructions...</i>", len(batch.Chunks), formatNumber(totalChars))
		} else {
			notice = fmt.Sprintf("📥 <b>Teks panjang terdeteksi (%d bagian, %s karakter).</b>\n⏳ <i>Berhasil digabungkan. Memproses instruksi...</i>", len(batch.Chunks), formatNumber(totalChars))
		}
		r.sendText(chatID, notice)
	}

	trimmed := strings.TrimSpace(combinedText)
	if strings.HasPrefix(trimmed, "/") {
		r.handleCommandWithIDs(batch.LastMsg, sess, trimmed, userMsgIDs)
		return
	}

	// Default: Stream Agent prompt with all user message IDs tracked for auto-delete.
	// If a task is already running, this is explicitly queued (never dropped).
	r.tryEnqueueOrExecute(chatID, sess, engine.StreamRunOptions{
		UserID:         userID,
		Prompt:         combinedText,
		CWD:            sess.CWD,
		ConversationID: sess.ActiveConversationID,
		PermissionMode: sess.PermissionMode,
		Model:          sess.ActiveModel,
		Effort:         sess.ActiveEffort,
		PrintTimeout:   r.cfg.Agy.PrintTimeout,
	}, userMsgIDs...)
}

func (r *Router) handleCommand(msg *tgbotapi.Message, sess *session.UserSession, text string) {
	r.handleCommandWithIDs(msg, sess, text, []int{msg.MessageID})
}

func (r *Router) handleCommandWithIDs(msg *tgbotapi.Message, sess *session.UserSession, text string, userMsgIDs []int) {
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
		r.handleUsage(chatID, sess)

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
		queueItems := r.listQueue(userID)
		queueSection := FormatQueueSection(sess.Language, queueItems)
		reply := tgbotapi.NewMessage(chatID, FormatStatusWithQueue(sess.Language, sess, isRunning, taskDesc, queueSection))
		reply.ParseMode = "HTML"
		reply.ReplyMarkup = StatusActionKeyboard(sess.Language)
		_, _ = r.bot.Send(reply)

	case "/queue", "/queuelist":
		r.sendQueueCard(chatID, sess)

	case "/clearqueue":
		removed := r.clearQueue(userID)
		if removed > 0 {
			if sess.Language == "en" {
				r.sendText(chatID, fmt.Sprintf("🧹 <b>Queue cleared:</b> %d pending message(s) dropped. Active task (if any) keeps running.", removed))
			} else {
				r.sendText(chatID, fmt.Sprintf("🧹 <b>Antrean dihapus:</b> %d pesan tertunda dibuang. Tugas aktif (jika ada) tetap berjalan.", removed))
			}
		} else {
			if sess.Language == "en" {
				r.sendText(chatID, "ℹ️ Queue is already empty — nothing to clear.")
			} else {
				r.sendText(chatID, "ℹ️ Antrean sudah kosong — tidak ada yang dihapus.")
			}
		}
		// Keep the live card truthful: re-render without the dropped items.
		r.refreshBusyCard(userID)

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
		resetMsg := fmt.Sprintf("🔄 <b>Sesi Percakapan Direset</b>\nSiap memulai sesi percakapan baru di workspace:\n<code>%s</code>", sess.CWD)
		if sess.Language == "en" {
			resetMsg = fmt.Sprintf("🔄 <b>Conversation Session Reset</b>\nReady to start fresh in workspace:\n<code>%s</code>", sess.CWD)
		}
		reply := tgbotapi.NewMessage(chatID, resetMsg)
		reply.ParseMode = "HTML"
		reply.ReplyMarkup = CloseOnlyKeyboard(sess.Language)
		_, _ = r.bot.Send(reply)

	case "/resume":
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

	case "/switch":
		if args != "" {
			target := strings.TrimSpace(args)
			// First check if target matches a saved Google account
			if accounts, _ := auth.ListSavedAccounts(); len(accounts) > 0 {
				for _, a := range accounts {
					if strings.EqualFold(a.Email, target) || strings.HasPrefix(strings.ToLower(a.Email), strings.ToLower(target)) {
						r.handleSwitchAccount(chatID, userID, sess, a.Email)
						return
					}
				}
			}
			// Fallback: check if target matches a conversation ID
			convs := r.sm.GetAvailableConversations(userID)
			for _, c := range convs {
				if c.ID == target || strings.HasPrefix(c.ID, target) {
					r.sm.UpdateConversation(userID, c.ID, "")
					reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ <b>Beralih ke Sesi Obrolan:</b> <code>%s</code>", c.ID))
					reply.ParseMode = "HTML"
					kb := ResumeConfirmedKeyboard(c.ID, sess.Language)
					reply.ReplyMarkup = &kb
					_, _ = r.bot.Send(reply)
					return
				}
			}
		}
		// If no argument, open the Google Accounts menu
		r.handleAccountsMenu(chatID, sess)

	case "/continue":
		r.sendText(chatID, "⏳ Melanjutkan sesi percakapan sebelumnya...")
		r.tryEnqueueOrExecute(chatID, sess, engine.StreamRunOptions{
			UserID:         userID,
			Prompt:         "Continue the previous task.",
			CWD:            sess.CWD,
			ConversationID: sess.ActiveConversationID,
			PermissionMode: sess.PermissionMode,
			Model:          sess.ActiveModel,
			Effort:         sess.ActiveEffort,
			PrintTimeout:   r.cfg.Agy.PrintTimeout,
		}, userMsgIDs...)

	case "/sessions":
		convs := r.sm.GetAvailableConversations(userID)
		reply := tgbotapi.NewMessage(chatID, FormatSessions(sess.Language, convs, sess.ActiveConversationID))
		reply.ParseMode = "HTML"
		reply.ReplyMarkup = ResumeKeyboard(convs, sess.ActiveConversationID, sess.Language)
		_, _ = r.bot.Send(reply)

	case "/cancel", "/stop":
		qlen := r.queueLength(userID)
		if r.cancelActiveTask(userID, chatID) {
			if qlen > 0 {
				r.sendText(chatID, fmt.Sprintf("🛑 <b>Tugas aktif dihentikan.</b>\n⏭️ <i>%d pesan dalam antrean tetap tersimpan dan akan dijalankan otomatis berikutnya.</i>\n<i>Gunakan /clearqueue untuk menghapus antrean.</i>", qlen))
			} else {
				r.sendText(chatID, "🛑 <b>Proses aktif berhasil dihentikan!</b>")
			}
		} else {
			if qlen > 0 {
				r.sendQueueCard(chatID, sess)
			} else {
				r.sendText(chatID, "ℹ️ Tidak ada proses yang sedang aktif berjalan.")
			}
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
		r.tryEnqueueOrExecute(chatID, sess, engine.StreamRunOptions{
			UserID:         userID,
			Prompt:         args,
			CWD:            sess.CWD,
			ConversationID: sess.ActiveConversationID,
			PermissionMode: sess.PermissionMode,
			Mode:           "plan",
			Model:          sess.ActiveModel,
			Effort:         sess.ActiveEffort,
			PrintTimeout:   r.cfg.Agy.PrintTimeout,
		}, userMsgIDs...)

	case "/goal":
		if args == "" {
			r.sendText(chatID, "Format: <code>/goal &lt;target autonomous task&gt;</code>")
			return
		}
		r.tryEnqueueOrExecute(chatID, sess, engine.StreamRunOptions{
			UserID:         userID,
			Prompt:         "/goal " + args,
			CWD:            sess.CWD,
			ConversationID: sess.ActiveConversationID,
			PermissionMode: sess.PermissionMode,
			Model:          sess.ActiveModel,
			Effort:         sess.ActiveEffort,
			PrintTimeout:   r.cfg.Agy.PrintTimeout,
			IsGoal:         true,
		}, userMsgIDs...)

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
					totalDel := 0
					for _, c := range cleared {
						if c.BotMsgID > 0 {
							totalDel++
						}
						totalDel += len(c.GetAllUserMsgIDs())
					}
					reply := tgbotapi.NewMessage(chatID, fmt.Sprintf(i18n.T(sess.Language, "autodelete_cleared"), totalDel))
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

	case "/signout", "/logout":
		r.handleSignOut(chatID, userID, sess)

	case "/signin", "/login":
		r.handleStartLogin(chatID, userID, sess)

	case "/accounts", "/account":
		r.handleAccountsMenu(chatID, sess)

	case "/whoami":
		r.handleWhoami(chatID, sess)

	case "/code":
		if args != "" {
			r.handleAuthCodeSubmission(chatID, userID, sess, args)
		} else {
			r.sendText(chatID, i18n.T(sess.Language, "auth_enter_code"))
		}

	default:
		// Forward any other custom slash command or skill to the agent.
		// Queue explicitly when busy so follow-ups are never silently dropped.
		r.tryEnqueueOrExecute(chatID, sess, engine.StreamRunOptions{
			UserID:         userID,
			Prompt:         text,
			CWD:            sess.CWD,
			ConversationID: sess.ActiveConversationID,
			PermissionMode: sess.PermissionMode,
			Model:          sess.ActiveModel,
			Effort:         sess.ActiveEffort,
			PrintTimeout:   r.cfg.Agy.PrintTimeout,
		}, userMsgIDs...)
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
		qlen := r.queueLength(userID)
		if r.cancelActiveTask(userID, chatID) {
			var text string
			if qlen > 0 {
				if sess.Language == "en" {
					text = fmt.Sprintf("🛑 <b>Active task stopped.</b>\n⏭️ <i>%d queued message(s) kept and will auto-run next.</i>", qlen)
				} else {
					text = fmt.Sprintf("🛑 <b>Tugas aktif dihentikan.</b>\n⏭️ <i>%d pesan antrean tetap tersimpan dan akan jalan otomatis.</i>", qlen)
				}
			} else {
				if sess.Language == "en" {
					text = "🛑 <b>Active task stopped.</b> You can send a new instruction now."
				} else {
					text = "🛑 <b>Tugas aktif berhasil dihentikan.</b> Anda dapat mengirim instruksi baru sekarang."
				}
			}
			edit := tgbotapi.NewEditMessageText(chatID, msgID, text)
			edit.ParseMode = "HTML"
			kb := QueueActionKeyboard(sess.Language)
			edit.ReplyMarkup = &kb
			_, _ = r.bot.Send(edit)
			// If the tapped message IS the live card, its content was just replaced —
			// drop tracking so later refreshes don't overwrite this terminal state.
			// Otherwise leave the card alone: the worker will flip it to the next
			// queued item (or finalized stopped state) within moments.
			r.busyMu.Lock()
			if c, ok := r.busyCards[userID]; ok && c != nil && c.ChatID == chatID && c.MsgID == msgID {
				delete(r.busyCards, userID)
			}
			r.busyMu.Unlock()
		} else {
			edit := tgbotapi.NewEditMessageText(chatID, msgID, "ℹ️ Tidak ada proses aktif yang sedang berjalan.")
			edit.ParseMode = "HTML"
			kb := CloseOnlyKeyboard(sess.Language)
			edit.ReplyMarkup = &kb
			_, _ = r.bot.Send(edit)
		}

	case data == "cmd_queue_menu":
		items := r.listQueue(userID)
		task := r.getActiveTask(userID)
		edit := tgbotapi.NewEditMessageText(chatID, msgID, FormatQueueList(sess.Language, task, items))
		edit.ParseMode = "HTML"
		kb := QueueActionKeyboard(sess.Language)
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case data == "cmd_clear_queue":
		removed := r.clearQueue(userID)
		var text string
		if removed > 0 {
			if sess.Language == "en" {
				text = fmt.Sprintf("🧹 <b>Queue cleared:</b> %d pending message(s) dropped. Active task keeps running.", removed)
			} else {
				text = fmt.Sprintf("🧹 <b>Antrean dihapus:</b> %d pesan tertunda dibuang. Tugas aktif tetap berjalan.", removed)
			}
		} else {
			if sess.Language == "en" {
				text = "ℹ️ Queue is already empty — nothing to clear."
			} else {
				text = "ℹ️ Antrean sudah kosong — tidak ada yang dihapus."
			}
		}
		edit := tgbotapi.NewEditMessageText(chatID, msgID, text)
		edit.ParseMode = "HTML"
		kb := QueueActionKeyboard(sess.Language)
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

		// Live card shows the queue without the dropped items from now on.
		// Same-message case: the tapped "cleared" confirmation already IS the
		// current content, so leave it — later ticks/worker events resume live view.
		r.busyMu.Lock()
		same := false
		if c, ok := r.busyCards[userID]; ok && c != nil && c.ChatID == chatID && c.MsgID == msgID {
			c.LastText = text
			same = true
		}
		r.busyMu.Unlock()
		if !same {
			r.refreshBusyCard(userID)
		}

	case data == "cmd_cancel_all":
		removed := r.clearQueue(userID)
		stopped := r.cancelActiveTask(userID, chatID)
		var text string
		if sess.Language == "en" {
			text = fmt.Sprintf("🛑 <b>Full stop:</b> active task stopped=%v, %d queued dropped. Queue is now empty.", stopped, removed)
		} else {
			text = fmt.Sprintf("🛑 <b>Berhenti total:</b> tugas aktif dihentikan=%v, %d antrean dibuang. Antrean kini kosong.", stopped, removed)
		}
		edit := tgbotapi.NewEditMessageText(chatID, msgID, text)
		edit.ParseMode = "HTML"
		kb := CloseOnlyKeyboard(sess.Language)
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)
		// Terminal state lives on the tapped message now. If the live card is a
		// different message, flip it to stopped too; if it's the same one,
		// just drop tracking so nothing overwrites this final text.
		r.busyMu.Lock()
		card, hasCard := r.busyCards[userID]
		sameMsg := hasCard && card != nil && card.ChatID == chatID && card.MsgID == msgID
		if sameMsg {
			delete(r.busyCards, userID)
		}
		r.busyMu.Unlock()
		if hasCard && !sameMsg {
			r.finalizeBusyCard(userID, sess.Language, true)
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
		r.tryEnqueueOrExecute(chatID, sess, engine.StreamRunOptions{
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
		r.tryEnqueueOrExecute(chatID, sess, engine.StreamRunOptions{
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
		r.handleUsageInPlace(chatID, msgID, sess)

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
		queueItems := r.listQueue(userID)
		queueSection := FormatQueueSection(sess.Language, queueItems)
		edit := tgbotapi.NewEditMessageText(chatID, msgID, FormatStatusWithQueue(sess.Language, sess, isRunning, taskDesc, queueSection))
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
			totalDel := 0
			for _, c := range cleared {
				if c.BotMsgID > 0 {
					totalDel++
				}
				totalDel += len(c.GetAllUserMsgIDs())
			}
			toast := tgbotapi.NewCallback(cb.ID, fmt.Sprintf(i18n.T(sess.Language, "autodelete_cleared"), totalDel))
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
		toast := tgbotapi.NewCallback(cb.ID, i18n.T(newLang, "lang_changed"))
		_, _ = r.bot.Request(toast)

		sess = r.sm.GetSession(userID, chatID)
		edit := tgbotapi.NewEditMessageText(chatID, msgID, i18n.GetLanguageMenuText(sess.Language))
		edit.ParseMode = "HTML"
		kb := LanguageSelectionKeyboard(sess.Language)
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case data == "cmd_new":
		r.sm.ResetConversation(userID)
		resetMsg := fmt.Sprintf("🔄 <b>Sesi Percakapan Direset</b>\nWorkspace aktif: <code>%s</code>", sess.CWD)
		if sess.Language == "en" {
			resetMsg = fmt.Sprintf("🔄 <b>Conversation Session Reset</b>\nActive workspace: <code>%s</code>", sess.CWD)
		}
		edit := tgbotapi.NewEditMessageText(chatID, msgID, resetMsg)
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

	case data == "auth_start":
		r.handleStartLogin(chatID, userID, sess)

	case data == "auth_cancel":
		r.authMgr.CancelLogin(userID)
		edit := tgbotapi.NewEditMessageText(chatID, msgID, i18n.T(sess.Language, "auth_cancelled"))
		edit.ParseMode = "HTML"
		kb := CloseOnlyKeyboard(sess.Language)
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case data == "auth_signout":
		r.handleSignOut(chatID, userID, sess)

	case data == "cmd_accounts_menu":
		accounts, _ := auth.ListSavedAccounts()
		activeAcc, _ := auth.GetActiveAccount()
		activeEmail := ""
		if activeAcc != nil {
			activeEmail = activeAcc.Email
		}
		edit := tgbotapi.NewEditMessageText(chatID, msgID, i18n.FormatAccountsList(sess.Language, activeAcc, accounts))
		edit.ParseMode = "HTML"
		kb := AccountsKeyboard(accounts, activeEmail, sess.Language)
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)

	case strings.HasPrefix(data, "auth_detail:"):
		email := strings.TrimPrefix(data, "auth_detail:")
		r.handleAccountDetail(chatID, msgID, sess, email, true)

	case strings.HasPrefix(data, "auth_switch:"):
		email := strings.TrimPrefix(data, "auth_switch:")
		r.handleSwitchAccount(chatID, userID, sess, email)

	case strings.HasPrefix(data, "auth_delete:"):
		email := strings.TrimPrefix(data, "auth_delete:")
		r.handleDeleteAccount(chatID, msgID, sess, email)
	}
}

func (r *Router) deleteTurnMessagesAsync(chatID int64, entries []session.TurnMessageEntry) {
	for _, e := range entries {
		if e.BotMsgID > 0 {
			delBot := tgbotapi.NewDeleteMessage(chatID, e.BotMsgID)
			_, _ = r.bot.Request(delBot)
		}
		for _, uID := range e.GetAllUserMsgIDs() {
			if uID > 0 {
				delUser := tgbotapi.NewDeleteMessage(chatID, uID)
				_, _ = r.bot.Request(delUser)
			}
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

	loadingText := fmt.Sprintf("⏳ Menjalankan <code>%s</code>...", command)
	if lang == "en" {
		loadingText = fmt.Sprintf("⏳ Executing <code>%s</code>...", command)
	}
	loadingMsg, err := r.bot.Send(tgbotapi.NewMessage(chatID, loadingText))
	if err != nil {
		return
	}

	out, err := r.oneShot.Run(ctx, cwd, command)
	lowerOut := strings.ToLower(out)
	if err != nil || strings.Contains(lowerOut, "eligibility check failed") || strings.Contains(lowerOut, "resource_exhausted") {
		rawErr := ""
		if err != nil {
			rawErr = err.Error()
		} else {
			rawErr = out
		}
		pe := i18n.ParseError(rawErr)
		errKb := ErrorActionKeyboard(pe.ExtractedURLs, pe.IsEligibility, pe.IsQuotaLimit, lang)
		edit := tgbotapi.NewEditMessageText(chatID, loadingMsg.MessageID, i18n.FormatErrorCard(lang, pe))
		edit.ParseMode = "HTML"
		edit.ReplyMarkup = &errKb
		_, _ = r.bot.Send(edit)
		return
	}

	if out == "" {
		if lang == "en" {
			out = "(Empty output)"
		} else {
			out = "(Output kosong)"
		}
	}

	kb := CloseOnlyKeyboard(lang)
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

	loadingText := fmt.Sprintf("⏳ Menjalankan <code>%s</code>...", command)
	if lang == "en" {
		loadingText = fmt.Sprintf("⏳ Executing <code>%s</code>...", command)
	}
	loadingEdit := tgbotapi.NewEditMessageText(chatID, messageID, loadingText)
	loadingEdit.ParseMode = "HTML"
	_, _ = r.bot.Send(loadingEdit)

	out, err := r.oneShot.Run(ctx, cwd, command)
	lowerOut := strings.ToLower(out)
	if err != nil || strings.Contains(lowerOut, "eligibility check failed") || strings.Contains(lowerOut, "resource_exhausted") {
		rawErr := ""
		if err != nil {
			rawErr = err.Error()
		} else {
			rawErr = out
		}
		pe := i18n.ParseError(rawErr)
		errKb := ErrorActionKeyboard(pe.ExtractedURLs, pe.IsEligibility, pe.IsQuotaLimit, lang)
		edit := tgbotapi.NewEditMessageText(chatID, messageID, i18n.FormatErrorCard(lang, pe))
		edit.ParseMode = "HTML"
		edit.ReplyMarkup = &errKb
		_, _ = r.bot.Send(edit)
		return
	}

	if out == "" {
		if lang == "en" {
			out = "(Empty output)"
		} else {
			out = "(Output kosong)"
		}
	}

	kb := RefreshAndBackKeyboard(refreshCmd, "cmd_help_menu", lang)
	edit := tgbotapi.NewEditMessageText(chatID, messageID, FormatCodeBlock(title, out))
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &kb
	_, _ = r.bot.Send(edit)
}

// fetchUsage runs `agy -p "/usage"` and classifies failures. Returns raw output
// plus a parsed error card when the call failed (nil on success).
func (r *Router) fetchUsage(cwd string) (string, *i18n.ParsedError) {
	out, err := r.oneShot.Run(context.Background(), cwd, "/usage")
	lowerOut := strings.ToLower(out)
	if err != nil || strings.Contains(lowerOut, "eligibility check failed") || strings.Contains(lowerOut, "resource_exhausted") {
		rawErr := out
		if err != nil {
			rawErr = err.Error()
		}
		pe := i18n.ParseError(rawErr)
		return out, pe
	}
	return out, nil
}

func (r *Router) renderUsageBody(lang, out string, acc ...*auth.AccountInfo) string {
	var a *auth.AccountInfo
	if len(acc) > 0 {
		a = acc[0]
	}
	if entries, ok := ParseUsageOutput(out); ok {
		return FormatUsageCard(lang, entries, time.Now(), a)
	}
	// Fallback: raw output in a code block when the shape is unexpected.
	if strings.TrimSpace(out) == "" {
		if lang == "en" {
			return "(Empty output)"
		}
		return "(Output kosong)"
	}
	var sb strings.Builder
	if a != nil && a.Email != "" {
		tier := a.Tier
		if tier == "" {
			tier = "Free"
		}
		if lang == "en" {
			sb.WriteString(fmt.Sprintf("👤 <b>Account:</b> <code>%s</code>\n", renderer.EscapeHTML(a.Email)))
			sb.WriteString(fmt.Sprintf("%s <b>Plan:</b> %s\n\n", auth.TierIcon(tier), renderer.EscapeHTML(tier)))
		} else {
			sb.WriteString(fmt.Sprintf("👤 <b>Akun:</b> <code>%s</code>\n", renderer.EscapeHTML(a.Email)))
			sb.WriteString(fmt.Sprintf("%s <b>Langganan:</b> %s\n\n", auth.TierIcon(tier), renderer.EscapeHTML(tier)))
		}
	}
	sb.WriteString(FormatCodeBlock("📊 Model Quota & Limit", out))
	return sb.String()
}

// handleUsage renders the beautified /usage quota card (slash-command path).
func (r *Router) handleUsage(chatID int64, sess *session.UserSession) {
	lang := sess.Language
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

	loadingText := "⏳ Menjalankan <code>/usage</code>..."
	if lang == "en" {
		loadingText = "⏳ Executing <code>/usage</code>..."
	}
	loadingMsg, err := r.bot.Send(tgbotapi.NewMessage(chatID, loadingText))
	if err != nil {
		return
	}

	var (
		out       string
		perr      *i18n.ParsedError
		activeAcc *auth.AccountInfo
	)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		out, perr = r.fetchUsage(sess.CWD)
	}()

	go func() {
		defer wg.Done()
		acc, err := auth.GetActiveAccount()
		if err == nil && acc != nil && acc.Email != "" {
			acc.Tier = auth.GetActiveTier(acc.Email)
			activeAcc = acc
		}
	}()

	wg.Wait()

	if perr != nil {
		errKb := ErrorActionKeyboard(perr.ExtractedURLs, perr.IsEligibility, perr.IsQuotaLimit, lang)
		edit := tgbotapi.NewEditMessageText(chatID, loadingMsg.MessageID, i18n.FormatErrorCard(lang, perr))
		edit.ParseMode = "HTML"
		edit.ReplyMarkup = &errKb
		_, _ = r.bot.Send(edit)
		return
	}

	kb := RefreshAndBackKeyboard("cmd_usage", "cmd_help_menu", lang)
	edit := tgbotapi.NewEditMessageText(chatID, loadingMsg.MessageID, r.renderUsageBody(lang, out, activeAcc))
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &kb
	_, _ = r.bot.Send(edit)
}

// handleUsageInPlace renders the beautified /usage quota card into an existing
// message (inline-button refresh path).
func (r *Router) handleUsageInPlace(chatID int64, messageID int, sess *session.UserSession) {
	lang := sess.Language
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

	loadingText := "⏳ Menjalankan <code>/usage</code>..."
	if lang == "en" {
		loadingText = "⏳ Executing <code>/usage</code>..."
	}
	loadingEdit := tgbotapi.NewEditMessageText(chatID, messageID, loadingText)
	loadingEdit.ParseMode = "HTML"
	_, _ = r.bot.Send(loadingEdit)

	var (
		out       string
		perr      *i18n.ParsedError
		activeAcc *auth.AccountInfo
	)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		out, perr = r.fetchUsage(sess.CWD)
	}()

	go func() {
		defer wg.Done()
		acc, err := auth.GetActiveAccount()
		if err == nil && acc != nil && acc.Email != "" {
			acc.Tier = auth.GetActiveTier(acc.Email)
			activeAcc = acc
		}
	}()

	wg.Wait()

	if perr != nil {
		errKb := ErrorActionKeyboard(perr.ExtractedURLs, perr.IsEligibility, perr.IsQuotaLimit, lang)
		edit := tgbotapi.NewEditMessageText(chatID, messageID, i18n.FormatErrorCard(lang, perr))
		edit.ParseMode = "HTML"
		edit.ReplyMarkup = &errKb
		_, _ = r.bot.Send(edit)
		return
	}

	kb := RefreshAndBackKeyboard("cmd_usage", "cmd_help_menu", lang)
	edit := tgbotapi.NewEditMessageText(chatID, messageID, r.renderUsageBody(lang, out, activeAcc))
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &kb
	_, _ = r.bot.Send(edit)
}

func (r *Router) executeAgentTurn(chatID int64, sess *session.UserSession, opts engine.StreamRunOptions, userMsgIDs ...int) {
	r.ensureQueueInit()
	if opts.PrintTimeout == "" {
		opts.PrintTimeout = r.cfg.Agy.PrintTimeout
	}
	if opts.PrintTimeout == "" {
		opts.PrintTimeout = "24h"
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Use closure so reassigned cancel funcs (after /cancel + auto-continue) are still released on exit.
	defer func() { cancel() }()

	r.registerActiveTask(opts.UserID, chatID, opts.Prompt, cancel, opts.IsGoal)
	defer r.unregisterActiveTask(opts.UserID)

	// Live busy card: ticks duration/positions while work is in flight.
	stopBusyTicker := r.startBusyTicker(opts.UserID)
	defer stopBusyTicker()

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

	curOpts := opts
	curUserMsgIDs := append([]int(nil), userMsgIDs...)
	curSess := sess
	queueDone := 0

	for {
		// Keep the active-task card in sync with the item actually running.
		r.taskMu.Lock()
		if t, ok := r.activeTasks[curOpts.UserID]; ok && t != nil {
			t.Prompt = curOpts.Prompt
			// Only reset StartedAt for queued follow-ups, not the first turn.
			if queueDone > 0 {
				t.StartedAt = time.Now()
			}
			t.IsGoal = curOpts.IsGoal
		}
		r.taskMu.Unlock()

		if queueDone > 0 {
			// Dynamic transition: the same live card flips from "queued" to
			// "now running" instead of staying stuck on the old waiting text.
			remaining := r.queueLength(curOpts.UserID)
			hl := runningHighlight(curSess.Language, truncatePreview(curOpts.Prompt, 140), remaining)
			r.upsertBusyCard(chatID, curOpts.UserID, curSess.Language, hl)
		}

		r.runAgentGoalLoop(ctx, chatID, curSess, &curOpts, curUserMsgIDs)

		if ctx.Err() != nil {
			// Current turn was cancelled via /cancel. Start next queued item with a
			// fresh context so cancellation only stops the current turn, not the whole queue.
			// Re-register because cancelActiveTask deleted the map entry.
			next := r.dequeueNext(curOpts.UserID)
			if next == nil {
				break
			}
			queueDone++
			freshSess := r.sm.GetSession(next.UserID, next.ChatID)
			next.ConversationID = freshSess.ActiveConversationID
			if next.CWD == "" {
				next.CWD = freshSess.CWD
			}
			if next.PermissionMode == "" {
				next.PermissionMode = freshSess.PermissionMode
			}
			if next.PrintTimeout == "" {
				next.PrintTimeout = r.cfg.Agy.PrintTimeout
			}
			nextCtx, nextCancel := context.WithCancel(context.Background())
			defer nextCancel()
			ctx = nextCtx
			r.registerActiveTask(next.UserID, next.ChatID, next.Prompt, nextCancel, next.IsGoal)
			curOpts = next.ToStreamOptions()
			curUserMsgIDs = append([]int(nil), next.UserMsgIDs...)
			curSess = freshSess
			chatID = next.ChatID
			continue
		}

		next := r.dequeueNext(curOpts.UserID)
		if next == nil {
			break
		}
		queueDone++
		freshSess := r.sm.GetSession(next.UserID, next.ChatID)
		next.ConversationID = freshSess.ActiveConversationID
		if next.CWD == "" {
			next.CWD = freshSess.CWD
		}
		if next.PermissionMode == "" {
			next.PermissionMode = freshSess.PermissionMode
		}
		if next.PrintTimeout == "" {
			next.PrintTimeout = r.cfg.Agy.PrintTimeout
		}
		curOpts = next.ToStreamOptions()
		curUserMsgIDs = append([]int(nil), next.UserMsgIDs...)
		curSess = freshSess
		chatID = next.ChatID
	}

	// Terminal transition: flip the live card to finished/stopped instead of
	// leaving the last "waiting/running" text frozen in chat.
	r.finalizeBusyCard(curOpts.UserID, curSess.Language, ctx.Err() != nil)
}

// runAgentGoalLoop runs a single agent turn including the /goal multi-turn loop.
func (r *Router) runAgentGoalLoop(ctx context.Context, chatID int64, sess *session.UserSession, opts *engine.StreamRunOptions, userMsgIDs []int) {
	maxTurns := 1
	if opts.IsGoal {
		maxTurns = 15
	}

	iteration := 1
	for iteration <= maxTurns {
		if ctx.Err() != nil {
			break
		}

		var turnUserMsgIDs []int
		if iteration == 1 {
			turnUserMsgIDs = userMsgIDs
		}

		result, err := r.runSingleStreamTurn(ctx, chatID, sess, *opts, iteration, turnUserMsgIDs)
		if ctx.Err() != nil {
			break
		}
		if err != nil || (result != nil && result.Status == "ERROR") {
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

func (r *Router) runSingleStreamTurn(ctx context.Context, chatID int64, sess *session.UserSession, opts engine.StreamRunOptions, iteration int, userMsgIDs []int) (*engine.ResultPayload, error) {
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
	var errorDetails *i18n.ParsedError
	var actionKb *tgbotapi.InlineKeyboardMarkup

	if err != nil {
		if ctx.Err() != nil {
			if sess.Language == "en" {
				finalText = "🛑 <b>Task stopped by user.</b>"
			} else {
				finalText = "🛑 <b>Tugas dihentikan oleh pengguna.</b>"
			}
		} else {
			errorDetails = i18n.ParseError(err.Error())
			finalText = i18n.FormatErrorCard(sess.Language, errorDetails)
			kb := ErrorActionKeyboard(errorDetails.ExtractedURLs, errorDetails.IsEligibility, errorDetails.IsQuotaLimit, sess.Language)
			actionKb = &kb
			if errorDetails.IsQuotaLimit || errorDetails.IsEligibility || errorDetails.IsAuth {
				r.sm.ResetConversation(opts.UserID)
			}
		}
	} else if result != nil {
		lowerResp := strings.ToLower(result.Response)
		isEligibilityInResp := strings.Contains(lowerResp, "eligibility check failed") || strings.Contains(lowerResp, "not eligible")

		if result.Status == "ERROR" || isEligibilityInResp || (result.Error != "" && strings.TrimSpace(result.Response) == "") {
			rawErr := result.Error
			if isEligibilityInResp && (rawErr == "" || !strings.Contains(strings.ToLower(rawErr), "eligibility")) {
				rawErr = result.Response
			} else if rawErr == "" {
				rawErr = result.Response
			}
			if rawErr == "" {
				rawErr = "Unknown execution error"
			}
			errorDetails = i18n.ParseError(rawErr)
			finalText = i18n.FormatErrorCard(sess.Language, errorDetails)
			kb := ErrorActionKeyboard(errorDetails.ExtractedURLs, errorDetails.IsEligibility, errorDetails.IsQuotaLimit, sess.Language)
			actionKb = &kb
			if errorDetails.IsQuotaLimit || errorDetails.IsEligibility || errorDetails.IsAuth {
				r.sm.ResetConversation(opts.UserID)
			}
		} else {
			// Successful response: do not append transient step retry errors
			footer = FormatResultFooter(result)
			finalText = result.Response
			if result.ConversationID != "" {
				lastConvID = result.ConversationID
				r.sm.UpdateConversation(opts.UserID, lastConvID, "")
			}
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
		savedBuffer.FinalizeWithKeyboard(finalText, footer, actions, actionKb)
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
			if actionKb != nil {
				edit.ReplyMarkup = actionKb
			}
			if _, sendErr := r.bot.Send(edit); sendErr == nil {
				edited = true
				finalBotMsgID = trackerMsgID
			} else {
				editPlain := tgbotapi.NewEditMessageText(chatID, trackerMsgID, renderer.StripHTML(formatted))
				if actionKb != nil {
					editPlain.ReplyMarkup = actionKb
				}
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
			if actionKb != nil {
				msg.ReplyMarkup = actionKb
			}
			if sentMsg, sendErr := r.bot.Send(msg); sendErr == nil {
				finalBotMsgID = sentMsg.MessageID
			} else {
				plain := renderer.StripHTML(formatted)
				msgPlain := tgbotapi.NewMessage(chatID, plain)
				if actionKb != nil {
					msgPlain.ReplyMarkup = actionKb
				}
				if sentPlain, errPlain := r.bot.Send(msgPlain); errPlain == nil {
					finalBotMsgID = sentPlain.MessageID
				}
			}
		}
	}

	// Auto-delete turn tracking: record turn and evict expired messages in Telegram
	if finalBotMsgID > 0 {
		toDelete := r.sm.RecordTurnMessages(opts.UserID, userMsgIDs, finalBotMsgID)
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

func (r *Router) handleStartLogin(chatID int64, userID int64, sess *session.UserSession) {
	if r.authMgr.HasActiveSession(userID) {
		r.authMgr.CancelLogin(userID)
	}

	loadingMsg, err := r.bot.Send(tgbotapi.NewMessage(chatID, "⏳ Memulai sesi otorisasi Google OAuth 2.0..."))
	if err != nil {
		return
	}

	loginSess, err := r.authMgr.StartLogin(context.Background(), userID, chatID)
	if err != nil {
		edit := tgbotapi.NewEditMessageText(chatID, loadingMsg.MessageID, i18n.FormatAuthError(sess.Language, err.Error()))
		edit.ParseMode = "HTML"
		kb := AuthErrorKeyboard(sess.Language)
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)
		return
	}

	r.authMgr.SetPromptMsgID(userID, loadingMsg.MessageID)

	edit := tgbotapi.NewEditMessageText(chatID, loadingMsg.MessageID, i18n.FormatLoginPrompt(sess.Language, loginSess.AuthURL))
	edit.ParseMode = "HTML"
	kb := AuthLoginKeyboard(loginSess.AuthURL, sess.Language)
	edit.ReplyMarkup = &kb
	_, _ = r.bot.Send(edit)
}

func (r *Router) handleAuthCodeSubmission(chatID int64, userID int64, sess *session.UserSession, rawInput string) {
	code := auth.ExtractAuthCode(rawInput)
	if code == "" {
		r.sendText(chatID, "⚠️ Format kode tidak valid. Silakan salin authorization code atau link callback URL lengkap dari browser.")
		return
	}

	loadingMsg, err := r.bot.Send(tgbotapi.NewMessage(chatID, "⏳ Memverifikasi authorization code dengan Google..."))
	if err != nil {
		return
	}

	acc, err := r.authMgr.SubmitCode(userID, code)
	if err != nil {
		edit := tgbotapi.NewEditMessageText(chatID, loadingMsg.MessageID, i18n.FormatAuthError(sess.Language, err.Error()))
		edit.ParseMode = "HTML"
		kb := AuthErrorKeyboard(sess.Language)
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)
		return
	}

	if acc == nil {
		acc, _ = auth.GetActiveAccount()
	}
	if acc == nil {
		acc = &auth.AccountInfo{
			Email:      "Google Account",
			Name:       "User",
			AuthMethod: "Google OAuth 2.0 (Consumer)",
			IsActive:   true,
		}
	}

	r.sm.ResetConversation(userID)
	edit := tgbotapi.NewEditMessageText(chatID, loadingMsg.MessageID, i18n.FormatLoginSuccess(sess.Language, acc))
	edit.ParseMode = "HTML"
	kb := CloseOnlyKeyboard(sess.Language)
	edit.ReplyMarkup = &kb
	_, _ = r.bot.Send(edit)
}

func (r *Router) handleSignOut(chatID int64, userID int64, sess *session.UserSession) {
	if r.authMgr.HasActiveSession(userID) {
		r.authMgr.CancelLogin(userID)
	}

	activeAcc, _ := auth.GetActiveAccount()
	activeEmail := ""
	if activeAcc != nil {
		activeEmail = activeAcc.Email
	}

	_, err := auth.SignOut()
	if err != nil {
		r.sendText(chatID, fmt.Sprintf("❌ Gagal sign out: %v", err))
		return
	}

	r.sm.ResetConversation(userID)
	accounts, _ := auth.ListSavedAccounts()
	reply := tgbotapi.NewMessage(chatID, i18n.FormatSignOutSuccess(sess.Language, activeEmail))
	reply.ParseMode = "HTML"
	kb := AccountsKeyboard(accounts, "", sess.Language)
	reply.ReplyMarkup = &kb
	_, _ = r.bot.Send(reply)
}

func (r *Router) handleAccountsMenu(chatID int64, sess *session.UserSession) {
	accounts, _ := auth.ListSavedAccounts()
	activeAcc, _ := auth.GetActiveAccount()
	activeEmail := ""
	if activeAcc != nil {
		activeEmail = activeAcc.Email
	}

	reply := tgbotapi.NewMessage(chatID, i18n.FormatAccountsList(sess.Language, activeAcc, accounts))
	reply.ParseMode = "HTML"
	kb := AccountsKeyboard(accounts, activeEmail, sess.Language)
	reply.ReplyMarkup = &kb
	_, _ = r.bot.Send(reply)
}

func (r *Router) handleAccountDetail(chatID int64, msgID int, sess *session.UserSession, email string, inPlace bool) {
	activeAcc, _ := auth.GetActiveAccount()
	isActive := activeAcc != nil && strings.EqualFold(activeAcc.Email, email)

	var acc *auth.AccountInfo
	if isActive {
		acc = activeAcc
		acc.Tier = auth.GetActiveTier(acc.Email)
	} else {
		saved, err := auth.GetSavedAccount(email)
		if err != nil {
			r.sendText(chatID, fmt.Sprintf("❌ Akun <code>%s</code> tidak ditemukan (%v).", email, err))
			return
		}
		acc = saved
	}

	text := i18n.FormatAccountDetail(sess.Language, acc)
	kb := AccountDetailKeyboard(email, isActive, sess.Language)

	if inPlace && msgID > 0 {
		edit := tgbotapi.NewEditMessageText(chatID, msgID, text)
		edit.ParseMode = "HTML"
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)
	} else {
		reply := tgbotapi.NewMessage(chatID, text)
		reply.ParseMode = "HTML"
		reply.ReplyMarkup = &kb
		_, _ = r.bot.Send(reply)
	}
}

func (r *Router) handleSwitchAccount(chatID int64, userID int64, sess *session.UserSession, email string) {
	_, err := auth.SwitchAccount(email)
	if err != nil {
		r.sendText(chatID, fmt.Sprintf("❌ Gagal beralih ke akun <code>%s</code>: %v", email, err))
		return
	}

	r.sm.ResetConversation(userID)
	msgText := fmt.Sprintf("✅ <b>Berhasil Beralih Akun!</b>\nAkun Google aktif sekarang:\n👤 <code>%s</code>", email)
	if sess.Language == "en" {
		msgText = fmt.Sprintf("✅ <b>Switched Account Successfully!</b>\nActive Google account is now:\n👤 <code>%s</code>", email)
	}

	accounts, _ := auth.ListSavedAccounts()
	reply := tgbotapi.NewMessage(chatID, msgText)
	reply.ParseMode = "HTML"
	kb := AccountsKeyboard(accounts, email, sess.Language)
	reply.ReplyMarkup = &kb
	_, _ = r.bot.Send(reply)
}

func (r *Router) handleDeleteAccount(chatID int64, msgID int, sess *session.UserSession, email string) {
	err := auth.DeleteSavedAccount(email)
	if err != nil {
		r.sendText(chatID, fmt.Sprintf("❌ Gagal menghapus akun <code>%s</code>: %v", email, err))
		return
	}

	if sess.Language == "en" {
		r.sendText(chatID, fmt.Sprintf("🗑️ Account <code>%s</code> deleted from saved accounts.", email))
	} else {
		r.sendText(chatID, fmt.Sprintf("🗑️ Akun <code>%s</code> berhasil dihapus dari daftar tersimpan.", email))
	}

	// Re-render accounts menu
	accounts, _ := auth.ListSavedAccounts()
	activeAcc, _ := auth.GetActiveAccount()
	activeEmail := ""
	if activeAcc != nil {
		activeEmail = activeAcc.Email
	}
	kb := AccountsKeyboard(accounts, activeEmail, sess.Language)

	if msgID > 0 {
		edit := tgbotapi.NewEditMessageText(chatID, msgID, i18n.FormatAccountsList(sess.Language, activeAcc, accounts))
		edit.ParseMode = "HTML"
		edit.ReplyMarkup = &kb
		_, _ = r.bot.Send(edit)
	} else {
		reply := tgbotapi.NewMessage(chatID, i18n.FormatAccountsList(sess.Language, activeAcc, accounts))
		reply.ParseMode = "HTML"
		reply.ReplyMarkup = &kb
		_, _ = r.bot.Send(reply)
	}
}

func (r *Router) handleWhoami(chatID int64, sess *session.UserSession) {
	activeAcc, err := auth.GetActiveAccount()
	if err != nil || activeAcc == nil || activeAcc.Email == "" {
		if sess.Language == "en" {
			r.sendText(chatID, "ℹ️ <b>Not Logged In:</b> No active Google account found.\nUse <code>/login</code> to connect an account.")
		} else {
			r.sendText(chatID, "ℹ️ <b>Belum Login:</b> Tidak ada akun Google yang sedang aktif.\nGunakan <code>/login</code> untuk masuk.")
		}
		return
	}

	activeAcc.Tier = auth.GetActiveTier(activeAcc.Email)
	text := i18n.FormatAccountDetail(sess.Language, activeAcc)
	kb := AccountDetailKeyboard(activeAcc.Email, true, sess.Language)
	reply := tgbotapi.NewMessage(chatID, text)
	reply.ParseMode = "HTML"
	reply.ReplyMarkup = &kb
	_, _ = r.bot.Send(reply)
}

