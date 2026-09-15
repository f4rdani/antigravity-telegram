package throttler

import (
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type ActivityTracker struct {
	mu           sync.Mutex
	bot          *tgbotapi.BotAPI
	chatID       int64
	messageID    int
	lastText     string
	lastEditTime time.Time
	closed       bool
}

func NewActivityTracker(bot *tgbotapi.BotAPI, chatID int64, initialText string) *ActivityTracker {
	msg := tgbotapi.NewMessage(chatID, initialText)
	msg.ParseMode = "HTML"
	sent, err := bot.Send(msg)
	msgID := 0
	if err == nil {
		msgID = sent.MessageID
	}
	return &ActivityTracker{
		bot:          bot,
		chatID:       chatID,
		messageID:    msgID,
		lastText:     initialText,
		lastEditTime: time.Now(),
	}
}

func (a *ActivityTracker) Update(text string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed || a.messageID == 0 || text == "" || text == a.lastText {
		return
	}
	now := time.Now()
	if elapsed := now.Sub(a.lastEditTime); elapsed < 700*time.Millisecond {
		time.Sleep(700*time.Millisecond - elapsed)
	}
	edit := tgbotapi.NewEditMessageText(a.chatID, a.messageID, text)
	edit.ParseMode = "HTML"
	_, err := a.bot.Send(edit)
	if err == nil {
		a.lastText = text
		a.lastEditTime = time.Now()
	}
}

func (a *ActivityTracker) Delete() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed || a.messageID == 0 {
		return
	}
	a.closed = true
	del := tgbotapi.NewDeleteMessage(a.chatID, a.messageID)
	_, _ = a.bot.Request(del)
	a.messageID = 0
}
