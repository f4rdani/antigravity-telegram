package throttler

import (
	"strings"
	"sync"
	"time"

	"agy-tele/internal/renderer"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

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

	formatted := renderer.FormatMarkdownForTelegram(fullText)
	const maxChars = 3900

	if len(formatted) <= maxChars {
		editMsg := tgbotapi.NewEditMessageText(t.chatID, currentMsgID, formatted)
		editMsg.ParseMode = "HTML"
		_, err := t.bot.Send(editMsg)
		if err != nil && !strings.Contains(err.Error(), "message is not modified") {
			// Fallback: send as plain text without HTML parse mode
			plain := renderer.StripHTML(formatted)
			editPlain := tgbotapi.NewEditMessageText(t.chatID, currentMsgID, plain)
			_, _ = t.bot.Send(editPlain)
		}
	} else {
		chunks := splitIntoChunks(formatted, maxChars)
		if len(chunks) > 0 {
			editMsg := tgbotapi.NewEditMessageText(t.chatID, currentMsgID, chunks[0])
			editMsg.ParseMode = "HTML"
			_, err := t.bot.Send(editMsg)
			if err != nil && !strings.Contains(err.Error(), "message is not modified") {
				editPlain := tgbotapi.NewEditMessageText(t.chatID, currentMsgID, renderer.StripHTML(chunks[0]))
				_, _ = t.bot.Send(editPlain)
			}
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

	formatted := renderer.FormatMarkdownForTelegram(text)
	if footer != "" {
		formatted = formatted + "\n\n" + footer
	}

	const maxChars = 3900
	if len(formatted) <= maxChars {
		editMsg := tgbotapi.NewEditMessageText(t.chatID, t.activeMessage, formatted)
		editMsg.ParseMode = "HTML"
		_, err := t.bot.Send(editMsg)
		if err != nil {
			plainText := renderer.StripHTML(formatted)
			editPlain := tgbotapi.NewEditMessageText(t.chatID, t.activeMessage, plainText)
			_, _ = t.bot.Send(editPlain)
		}
	} else {
		chunks := splitIntoChunks(formatted, maxChars)
		for i, ch := range chunks {
			if i == 0 {
				editMsg := tgbotapi.NewEditMessageText(t.chatID, t.activeMessage, ch)
				editMsg.ParseMode = "HTML"
				_, err := t.bot.Send(editMsg)
				if err != nil {
					editPlain := tgbotapi.NewEditMessageText(t.chatID, t.activeMessage, renderer.StripHTML(ch))
					_, _ = t.bot.Send(editPlain)
				}
			} else {
				newMsg := tgbotapi.NewMessage(t.chatID, ch)
				newMsg.ParseMode = "HTML"
				_, err := t.bot.Send(newMsg)
				if err != nil {
					newPlain := tgbotapi.NewMessage(t.chatID, renderer.StripHTML(ch))
					_, _ = t.bot.Send(newPlain)
				}
			}
		}
	}
}
