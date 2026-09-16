package throttler

import (
	"strings"
	"sync"
	"time"

	"agy-tele/internal/renderer"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type ActivityTracker struct {
	mu        sync.Mutex
	bot       *tgbotapi.BotAPI
	chatID    int64
	messageID int
	lastText  string
	updateCh  chan string
	stopCh    chan struct{}
	doneCh    chan struct{}
	closed    bool
}

func NewActivityTracker(bot *tgbotapi.BotAPI, chatID int64, initialText string) *ActivityTracker {
	msg := tgbotapi.NewMessage(chatID, initialText)
	msg.ParseMode = "HTML"
	sent, err := bot.Send(msg)
	msgID := 0
	if err == nil {
		msgID = sent.MessageID
	}

	a := &ActivityTracker{
		bot:       bot,
		chatID:    chatID,
		messageID: msgID,
		lastText:  initialText,
		updateCh:  make(chan string, 32),
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}

	if msgID != 0 {
		go a.loop()
	} else {
		close(a.doneCh)
	}

	return a
}

func (a *ActivityTracker) loop() {
	defer close(a.doneCh)
	ticker := time.NewTicker(800 * time.Millisecond)
	defer ticker.Stop()

	var pendingText string
	hasPending := false

	for {
		select {
		case <-a.stopCh:
			return

		case text, ok := <-a.updateCh:
			if !ok {
				return
			}
			pendingText = text
			hasPending = true

		case <-ticker.C:
			if hasPending {
				a.mu.Lock()
				msgID := a.messageID
				closed := a.closed
				last := a.lastText
				a.mu.Unlock()

				if closed || msgID == 0 || pendingText == "" || pendingText == last {
					hasPending = false
					continue
				}

				if a.bot != nil {
					edit := tgbotapi.NewEditMessageText(a.chatID, msgID, pendingText)
					edit.ParseMode = "HTML"
					_, err := a.bot.Send(edit)
					if err != nil && !strings.Contains(err.Error(), "message is not modified") {
						// Fallback to plain text if HTML error
						editPlain := tgbotapi.NewEditMessageText(a.chatID, msgID, renderer.StripHTML(pendingText))
						_, _ = a.bot.Send(editPlain)
					}
				}

				a.mu.Lock()
				a.lastText = pendingText
				a.mu.Unlock()
				hasPending = false
			}
		}
	}
}

// Update sends a badge update asynchronously without blocking the caller
func (a *ActivityTracker) Update(text string) {
	a.mu.Lock()
	if a.closed || a.messageID == 0 {
		a.mu.Unlock()
		return
	}
	a.mu.Unlock()

	select {
	case a.updateCh <- text:
	default:
		// Queue full, worker will process latest
	}
}

// Delete stops the update loop and deletes the badge message in the background
func (a *ActivityTracker) Delete() {
	a.mu.Lock()
	if a.closed || a.messageID == 0 {
		a.mu.Unlock()
		return
	}
	a.closed = true
	msgID := a.messageID
	a.messageID = 0
	close(a.stopCh)
	a.mu.Unlock()

	go func() {
		select {
		case <-a.doneCh:
		case <-time.After(1 * time.Second):
		}
		if a.bot != nil {
			del := tgbotapi.NewDeleteMessage(a.chatID, msgID)
			_, _ = a.bot.Request(del)
		}
	}()
}

// MessageID returns the current message ID or 0 if closed/deleted
func (a *ActivityTracker) MessageID() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.messageID
}
