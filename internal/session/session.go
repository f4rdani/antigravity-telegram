package session

import (
	"sync"
	"time"
)

type ConversationEntry struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	CWD       string    `json:"cwd"`
}

type TurnMessageEntry struct {
	UserMsgID int       `json:"user_msg_id"`
	BotMsgID  int       `json:"bot_msg_id"`
	Timestamp time.Time `json:"timestamp"`
}

type UserSession struct {
	UserID               int64               `json:"user_id"`
	ChatID               int64               `json:"chat_id"`
	ActiveConversationID string              `json:"active_conversation_id"`
	CWD                  string              `json:"cwd"`
	PermissionMode       string              `json:"permission_mode"` // "auto" or "ask"
	ActiveModel          string              `json:"active_model"`
	ActiveEffort         string              `json:"active_effort"`
	LastActiveTime       time.Time           `json:"last_active_time"`
	RecentConversations  []ConversationEntry `json:"recent_conversations"`
	Language             string              `json:"language"`           // "id" or "en"
	MaxTelegramTurns     int                 `json:"max_telegram_turns"` // default 50 (0 = disabled)
	TrackedTurns         []TurnMessageEntry  `json:"tracked_turns"`      // turn message pairs
}

type SessionManager struct {
	mu          sync.RWMutex
	filePath    string
	sessions    map[int64]*UserSession
	defaultRoot string
	defaultPerm string
}
