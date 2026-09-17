package bot

import (
	"strings"
	"sync"
	"time"

	"agy-tele/internal/session"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// AccumulatedBatch represents a group of messages or split chunks from a single user.
type AccumulatedBatch struct {
	UserID    int64
	ChatID    int64
	Chunks    []string
	MsgIDs    []int
	FirstSeen time.Time
	LastSeen  time.Time
	Timer     *time.Timer
	Session   *session.UserSession
	LastMsg   *tgbotapi.Message
}

// FlushCallback is called when the debounce window expires.
type FlushCallback func(batch *AccumulatedBatch, combinedText string)

// MessageAccumulator debounces and merges split message chunks sent by Telegram clients.
type MessageAccumulator struct {
	mu      sync.Mutex
	batches map[int64]*AccumulatedBatch
}

func NewMessageAccumulator() *MessageAccumulator {
	return &MessageAccumulator{
		batches: make(map[int64]*AccumulatedBatch),
	}
}

// JoinSplitMessages merges chunk strings together.
// If a previous chunk has len >= 3800 characters, it was sliced by Telegram's 4096-char client limit,
// so it is concatenated directly without inserting a newline (which would break code or JSON).
// If the previous chunk is shorter, it is treated as a separate message and separated with a newline.
func JoinSplitMessages(chunks []string) string {
	if len(chunks) == 0 {
		return ""
	}
	if len(chunks) == 1 {
		return chunks[0]
	}

	var sb strings.Builder
	for i, chunk := range chunks {
		if i == 0 {
			sb.WriteString(chunk)
			continue
		}
		prev := chunks[i-1]
		if len(prev) >= 3800 {
			sb.WriteString(chunk)
		} else {
			if !strings.HasSuffix(prev, "\n") {
				sb.WriteString("\n")
			}
			sb.WriteString(chunk)
		}
	}
	return sb.String()
}

// IsInstantCommand checks if a command should bypass the accumulator and run immediately.
func IsInstantCommand(text string) bool {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "/") {
		return false
	}
	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return false
	}
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/cancel", "/stop", "/status", "/start", "/help",
		"/model", "/effort", "/perm", "/cwd", "/ls", "/clear",
		"/sessions", "/resume", "/new", "/lang", "/usage",
		"/quota", "/credits", "/skills", "/agents", "/changelog",
		"/artifact", "/artifacts":
		return len(trimmed) < 1000
	}
	return false
}

// Add adds a message to the user's batch and starts or extends the debounce timer.
func (a *MessageAccumulator) Add(msg *tgbotapi.Message, sess *session.UserSession, onFlush FlushCallback) {
	a.mu.Lock()
	defer a.mu.Unlock()

	userID := msg.From.ID
	text := msg.Text

	// Calculate debounce window:
	// If message is long (>= 3000 chars), subsequent chunks usually arrive within 100-300ms.
	// We give 850ms window. Otherwise 550ms.
	debounceDuration := 550 * time.Millisecond
	if len(text) >= 3000 {
		debounceDuration = 850 * time.Millisecond
	}

	batch, exists := a.batches[userID]
	if !exists {
		batch = &AccumulatedBatch{
			UserID:    userID,
			ChatID:    msg.Chat.ID,
			Chunks:    []string{text},
			MsgIDs:    []int{msg.MessageID},
			FirstSeen: time.Now(),
			LastSeen:  time.Now(),
			Session:   sess,
			LastMsg:   msg,
		}
		a.batches[userID] = batch

		batch.Timer = time.AfterFunc(debounceDuration, func() {
			a.flushBatch(userID, onFlush)
		})
		return
	}

	// Existing batch: stop previous timer and append chunk
	if batch.Timer != nil {
		batch.Timer.Stop()
	}
	batch.Chunks = append(batch.Chunks, text)
	batch.MsgIDs = append(batch.MsgIDs, msg.MessageID)
	batch.LastSeen = time.Now()
	batch.Session = sess
	batch.LastMsg = msg

	// Max ceiling check: if chunks have been arriving for over 4 seconds, flush now
	if time.Since(batch.FirstSeen) >= 4*time.Second {
		delete(a.batches, userID)
		// Release lock before invoking callback
		go func(b *AccumulatedBatch) {
			combined := JoinSplitMessages(b.Chunks)
			onFlush(b, combined)
		}(batch)
		return
	}

	// Re-arm timer
	batch.Timer = time.AfterFunc(debounceDuration, func() {
		a.flushBatch(userID, onFlush)
	})
}

// flushBatch handles timer expiry and executes the flush callback.
func (a *MessageAccumulator) flushBatch(userID int64, onFlush FlushCallback) {
	a.mu.Lock()
	batch, exists := a.batches[userID]
	if !exists {
		a.mu.Unlock()
		return
	}
	delete(a.batches, userID)
	a.mu.Unlock()

	combined := JoinSplitMessages(batch.Chunks)
	onFlush(batch, combined)
}

// Cancel cancels and removes any pending batch for the user.
func (a *MessageAccumulator) Cancel(userID int64) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	batch, exists := a.batches[userID]
	if exists && batch != nil {
		if batch.Timer != nil {
			batch.Timer.Stop()
		}
		delete(a.batches, userID)
		return true
	}
	return false
}

// FlushNow immediately flushes any pending batch for the user if present.
func (a *MessageAccumulator) FlushNow(userID int64, onFlush FlushCallback) bool {
	a.mu.Lock()
	batch, exists := a.batches[userID]
	if !exists || batch == nil {
		a.mu.Unlock()
		return false
	}
	if batch.Timer != nil {
		batch.Timer.Stop()
	}
	delete(a.batches, userID)
	a.mu.Unlock()

	combined := JoinSplitMessages(batch.Chunks)
	onFlush(batch, combined)
	return true
}
