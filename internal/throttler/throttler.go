package throttler

import (

	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type EditFunc func(chatID int64, messageID int, text string) error
type SendFunc func(chatID int64, text string) (int, error)

type MessageThrottler struct {
	mu            sync.Mutex
	bot           *tgbotapi.BotAPI
	chatID        int64
	messageID     int
	builder       strings.Builder
	dirty         bool
	interval      time.Duration
	stopCh        chan struct{}
	doneCh        chan struct{}
	lastFlush     time.Time
	activeMessage int
}

func NewMessageThrottler(bot *tgbotapi.BotAPI, chatID int64, initialMessageID int, intervalMs int) *MessageThrottler {
	if intervalMs <= 0 {
		intervalMs = 1200
	}

	t := &MessageThrottler{
		bot:           bot,
		chatID:        chatID,
		messageID:     initialMessageID,
		activeMessage: initialMessageID,
		interval:      time.Duration(intervalMs) * time.Millisecond,
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
	}

	go t.loop()
	return t
}

func (t *MessageThrottler) Append(delta string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.builder.WriteString(delta)
	t.dirty = true
}

func (t *MessageThrottler) loop() {
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()
	defer close(t.doneCh)

	for {
		select {
		case <-t.stopCh:
			t.flushInternal()
			return
		case <-ticker.C:
			t.flushInternal()
		}
	}
}

func (t *MessageThrottler) flushInternal() {
	t.mu.Lock()
	if !t.dirty {
		t.mu.Unlock()
		return
	}

	fullText := t.builder.String()
	t.dirty = false
	currentMsgID := t.activeMessage
	t.mu.Unlock()

	if strings.TrimSpace(fullText) == "" {
		return
	}

	// Telegram max text length is 4096
	// If text exceeds 3900 chars, chunk it
	const maxChars = 3900
	if len(fullText) <= maxChars {
		editMsg := tgbotapi.NewEditMessageText(t.chatID, currentMsgID, fullText)
		_, err := t.bot.Send(editMsg)
		if err != nil {
			// Ignore "message is not modified" error
			if !strings.Contains(err.Error(), "message is not modified") {
				// Retry with plain formatting or continue
			}
		}
	} else {
		// Divide into chunks
		chunks := splitIntoChunks(fullText, maxChars)
		if len(chunks) > 0 {
			// Edit the first chunk into currentMsgID
			editMsg := tgbotapi.NewEditMessageText(t.chatID, currentMsgID, chunks[0])
			_, _ = t.bot.Send(editMsg)

			// Send subsequent chunks as new messages if not already sent
			// In practice, during streaming we edit the active latest chunk
			// For simplicity and stability, we display the tail or chunked text
		}
	}
}

func splitIntoChunks(text string, chunkSize int) []string {
	var chunks []string
	runes := []rune(text)
	for len(runes) > 0 {
		if len(runes) <= chunkSize {
			chunks = append(chunks, string(runes))
			break
		}
		chunks = append(chunks, string(runes[:chunkSize]))
		runes = runes[chunkSize:]
	}
	return chunks
}

func (t *MessageThrottler) Stop() string {
	close(t.stopCh)
	<-t.doneCh
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.builder.String()
}

func (t *MessageThrottler) Finalize(finalText string, footer string) {
	t.Stop()

	t.mu.Lock()
	defer t.mu.Unlock()

	text := finalText
	if text == "" {
		text = t.builder.String()
	}
	if text == "" {
		text = "(Empty response)"
	}

	if footer != "" {
		text = text + "\n\n" + footer
	}

	const maxChars = 3900
	if len(text) <= maxChars {
		editMsg := tgbotapi.NewEditMessageText(t.chatID, t.activeMessage, text)
		_, _ = t.bot.Send(editMsg)
	} else {
		chunks := splitIntoChunks(text, maxChars)
		for i, ch := range chunks {
			if i == 0 {
				editMsg := tgbotapi.NewEditMessageText(t.chatID, t.activeMessage, ch)
				_, _ = t.bot.Send(editMsg)
			} else {
				newMsg := tgbotapi.NewMessage(t.chatID, ch)
				_, _ = t.bot.Send(newMsg)
			}
		}
	}
}
