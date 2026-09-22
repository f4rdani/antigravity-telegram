package bot

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"agy-tele/internal/engine"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// MaxQueuePerUser caps pending queued prompts per user to prevent abuse.
const MaxQueuePerUser = 10

// BusyTickInterval controls how often the live busy card refreshes its
// elapsed duration / queue positions while a task is running.
const BusyTickInterval = 30 * time.Second

// QueuedItem is a single pending agent turn waiting for the active task to finish.
type QueuedItem struct {
	ChatID         int64
	UserID         int64
	Prompt         string
	Mode           string // "plan" or ""
	IsGoal         bool
	CWD            string
	ConversationID string
	PermissionMode string
	Model          string
	Effort         string
	PrintTimeout   string
	UserMsgIDs     []int
	EnqueuedAt     time.Time
}

// ToStreamOptions converts a queued item into engine options.
// ConversationID is refreshed by the caller from the latest session when available.
func (q *QueuedItem) ToStreamOptions() engine.StreamRunOptions {
	return engine.StreamRunOptions{
		UserID:         q.UserID,
		Prompt:         q.Prompt,
		CWD:            q.CWD,
		ConversationID: q.ConversationID,
		PermissionMode: q.PermissionMode,
		Mode:           q.Mode,
		Model:          q.Model,
		Effort:         q.Effort,
		PrintTimeout:   q.PrintTimeout,
		IsGoal:         q.IsGoal,
	}
}

// enqueue adds an item to the user's FIFO queue. Returns position (1-based) or -1 if full.
func (r *Router) enqueue(userID int64, item *QueuedItem) int {
	r.queueMu.Lock()
	defer r.queueMu.Unlock()
	q := r.pendingQueues[userID]
	if len(q) >= MaxQueuePerUser {
		return -1
	}
	item.EnqueuedAt = time.Now()
	r.pendingQueues[userID] = append(q, item)
	return len(r.pendingQueues[userID])
}

// dequeueNext pops the oldest queued item for the user, or nil if empty.
func (r *Router) dequeueNext(userID int64) *QueuedItem {
	r.queueMu.Lock()
	defer r.queueMu.Unlock()
	q := r.pendingQueues[userID]
	if len(q) == 0 {
		return nil
	}
	next := q[0]
	if len(q) == 1 {
		delete(r.pendingQueues, userID)
	} else {
		r.pendingQueues[userID] = q[1:]
	}
	return next
}

// queueLength returns the number of pending items for the user.
func (r *Router) queueLength(userID int64) int {
	r.queueMu.Lock()
	defer r.queueMu.Unlock()
	return len(r.pendingQueues[userID])
}

// listQueue returns a copy of the pending queue for the user.
func (r *Router) listQueue(userID int64) []*QueuedItem {
	r.queueMu.Lock()
	defer r.queueMu.Unlock()
	q := r.pendingQueues[userID]
	out := make([]*QueuedItem, len(q))
	copy(out, q)
	return out
}

// clearQueue drops all pending items and returns the number removed.
func (r *Router) clearQueue(userID int64) int {
	r.queueMu.Lock()
	defer r.queueMu.Unlock()
	n := len(r.pendingQueues[userID])
	delete(r.pendingQueues, userID)
	return n
}

// removeQueueAt removes the 1-based indexed item. Returns removed item or nil.
func (r *Router) removeQueueAt(userID int64, index1 int) *QueuedItem {
	r.queueMu.Lock()
	defer r.queueMu.Unlock()
	q := r.pendingQueues[userID]
	if index1 < 1 || index1 > len(q) {
		return nil
	}
	removed := q[index1-1]
	r.pendingQueues[userID] = append(q[:index1-1], q[index1:]...)
	if len(r.pendingQueues[userID]) == 0 {
		delete(r.pendingQueues, userID)
	}
	return removed
}

// ensure queueMu/pendingQueues exist (defensive for tests constructing Router directly).
func (r *Router) ensureQueueInit() {
	if r.pendingQueues == nil {
		r.pendingQueues = make(map[int64][]*QueuedItem)
	}
}

var _ = sync.Mutex{}

// ---------------------------------------------------------------------------
// Dynamic live busy card
// ---------------------------------------------------------------------------
// Instead of sending a new static "please wait" card on every follow-up
// (which looks frozen/stuck), the bot keeps ONE live card per user and edits
// it in place: the elapsed duration ticks, queue positions shift, and the
// state transitions queued -> running -> done automatically.

// BusyCard tracks the single dynamic live status message per user.
type BusyCard struct {
	ChatID   int64
	MsgID    int
	LastText string
}

func (r *Router) ensureBusyInit() {
	if r.busyCards == nil {
		r.busyCards = make(map[int64]*BusyCard)
	}
}

// buildBusyCardText renders the dynamic live card: optional highlight header
// (just-queued ack or just-started notice, empty for neutral tick refresh),
// followed by the full active + FIFO queue state and a live timestamp footer
// proving the card is updating rather than stuck.
func buildBusyCardText(lang string, task *ActiveTask, items []*QueuedItem, highlight string) string {
	var sb strings.Builder
	if strings.TrimSpace(highlight) != "" {
		sb.WriteString(highlight + "\n\n")
	}
	sb.WriteString(FormatQueueList(lang, task, items))
	if lang == "en" {
		sb.WriteString(fmt.Sprintf("\n\n<i>🔄 Live status • updated %s • changes automatically, no need to resend</i>", time.Now().Format("15:04:05")))
	} else {
		sb.WriteString(fmt.Sprintf("\n\n<i>🔄 Status live • diperbarui %s • berubah otomatis, tidak perlu kirim ulang</i>", time.Now().Format("15:04:05")))
	}
	return sb.String()
}

func queuedHighlight(lang string, pos, total int, preview string) string {
	if lang == "en" {
		return fmt.Sprintf("✅ <b>Queued as #%d (total in queue: %d).</b>\n• <i>\"%s\"</i>\n▶️ <i>Will run automatically in order. No need to resend.</i>",
			pos, total, EscapeHTML(preview))
	}
	return fmt.Sprintf("✅ <b>Pesan masuk antrean #%d (total antrean: %d).</b>\n• <i>\"%s\"</i>\n▶️ <i>Akan dijalankan otomatis berurutan. Tidak perlu kirim ulang.</i>",
		pos, total, EscapeHTML(preview))
}

func runningHighlight(lang string, preview string, remaining int) string {
	if lang == "en" {
		extra := "<i>(queue empty — this was the last one)</i>"
		if remaining > 0 {
			extra = fmt.Sprintf("<i>(%d more waiting)</i>", remaining)
		}
		return fmt.Sprintf("▶️ <b>Queued prompt now running:</b> <i>\"%s\"</i>\n%s", EscapeHTML(preview), extra)
	}
	extra := "<i>(antrean habis — ini yang terakhir)</i>"
	if remaining > 0 {
		extra = fmt.Sprintf("<i>(%d lagi menunggu)</i>", remaining)
	}
	return fmt.Sprintf("▶️ <b>Prompt antrean berhasil dijalankan:</b> <i>\"%s\"</i>\n%s", EscapeHTML(preview), extra)
}

func busyDoneText(lang string, cancelled bool) string {
	if lang == "en" {
		if cancelled {
			return "🛑 <b>Task stopped.</b>\n<i>Queue is empty.</i>"
		}
		return "✅ <b>All tasks finished.</b>\n<i>Queue is empty. Send a new message anytime — it runs immediately.</i>"
	}
	if cancelled {
		return "🛑 <b>Tugas dihentikan.</b>\n<i>Antrean kosong.</i>"
	}
	return "✅ <b>Semua tugas selesai.</b>\n<i>Antrean kosong. Kirim pesan baru kapan saja — langsung jalan.</i>"
}

// editBusyCard edits the tracked live card in place. It never sends a new
// message: if the user deleted the card, tracking is dropped silently instead
// of spamming a replacement on every tick. Returns true if tracking is kept.
func (r *Router) editBusyCard(userID int64, card *BusyCard, lang, text string, done bool) bool {
	if r.bot == nil || card == nil {
		return false
	}
	if card.LastText == text {
		return true
	}
	edit := tgbotapi.NewEditMessageText(card.ChatID, card.MsgID, text)
	edit.ParseMode = "HTML"
	if done {
		kb := CloseOnlyKeyboard(lang)
		edit.ReplyMarkup = &kb
	} else {
		kb := QueueActionKeyboard(lang)
		edit.ReplyMarkup = &kb
	}
	if _, err := r.bot.Send(edit); err != nil {
		msg := err.Error()
		if strings.Contains(msg, "message is not modified") {
			r.busyMu.Lock()
			card.LastText = text
			r.busyMu.Unlock()
			return true
		}
		if strings.Contains(msg, "message to edit not found") ||
			strings.Contains(msg, "message can't be edited") {
			r.forgetBusyCard(userID)
			return false
		}
		return true // transient error: keep tracking, retry on next tick/event
	}
	r.busyMu.Lock()
	card.LastText = text
	r.busyMu.Unlock()
	return true
}

// upsertBusyCard creates the live card on the first queue event, or edits it
// in place afterwards. highlight describes what just happened (queued/running).
func (r *Router) upsertBusyCard(chatID, userID int64, lang, highlight string) {
	if r.bot == nil {
		return
	}
	r.ensureBusyInit()
	task := r.getActiveTask(userID)
	items := r.listQueue(userID)
	text := buildBusyCardText(lang, task, items, highlight)
	r.busyMu.Lock()
	card, ok := r.busyCards[userID]
	r.busyMu.Unlock()
	if ok && card != nil {
		r.editBusyCard(userID, card, lang, text, false)
		return
	}
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	kb := QueueActionKeyboard(lang)
	msg.ReplyMarkup = &kb
	sent, err := r.bot.Send(msg)
	if err != nil {
		return
	}
	r.busyMu.Lock()
	r.busyCards[userID] = &BusyCard{ChatID: chatID, MsgID: sent.MessageID, LastText: text}
	r.busyMu.Unlock()
}

// refreshBusyCard re-renders the live card from current state (neutral view,
// no highlight). Called by the ticker and after queue mutations. It never
// finalizes an idle+empty state — the worker's end-of-drain owns that edit.
func (r *Router) refreshBusyCard(userID int64) bool {
	if r.bot == nil {
		return false
	}
	r.ensureBusyInit()
	r.busyMu.Lock()
	card, ok := r.busyCards[userID]
	r.busyMu.Unlock()
	if !ok || card == nil {
		return false
	}
	sess := r.sm.GetSession(userID, card.ChatID)
	task := r.getActiveTask(userID)
	items := r.listQueue(userID)
	if task == nil && len(items) == 0 {
		return false
	}
	text := buildBusyCardText(sess.Language, task, items, "")
	return r.editBusyCard(userID, card, sess.Language, text, false)
}

// finalizeBusyCard edits the live card into its terminal state
// (finished vs stopped) and drops tracking. No-op when no card exists.
func (r *Router) finalizeBusyCard(userID int64, lang string, cancelled bool) {
	if r.bot == nil {
		return
	}
	r.ensureBusyInit()
	r.busyMu.Lock()
	card, ok := r.busyCards[userID]
	r.busyMu.Unlock()
	if !ok || card == nil {
		return
	}
	r.editBusyCard(userID, card, lang, busyDoneText(lang, cancelled), true)
	r.forgetBusyCard(userID)
}

func (r *Router) forgetBusyCard(userID int64) {
	r.busyMu.Lock()
	defer r.busyMu.Unlock()
	delete(r.busyCards, userID)
}

// startBusyTicker refreshes the live card's elapsed duration / queue positions
// while work is in flight. The returned func stops the ticker (deferred by the
// worker). Ticks never send new messages and never finalize — they only edit
// the existing card, and go quiet when everything is idle.
func (r *Router) startBusyTicker(userID int64) func() {
	stop := make(chan struct{})
	go func() {
		t := time.NewTicker(BusyTickInterval)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				if r.getActiveTask(userID) == nil && r.queueLength(userID) == 0 {
					return
				}
				r.refreshBusyCard(userID)
			}
		}
	}()
	return func() {
		select {
		case <-stop:
		default:
			close(stop)
		}
	}
}
