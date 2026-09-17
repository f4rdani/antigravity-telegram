package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func NewSessionManager(filePath string, defaultRoot string, defaultPerm string) (*SessionManager, error) {
	sm := &SessionManager{
		filePath:    filePath,
		sessions:    make(map[int64]*UserSession),
		defaultRoot: defaultRoot,
		defaultPerm: defaultPerm,
	}

	if err := sm.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load session store: %w", err)
	}

	return sm, nil
}

func (sm *SessionManager) load() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	data, err := os.ReadFile(sm.filePath)
	if err != nil {
		return err
	}

	var loaded map[int64]*UserSession
	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}

	sm.sessions = loaded
	for _, sess := range sm.sessions {
		if sess.Language == "" {
			sess.Language = "id"
		}
		if sess.MaxTelegramTurns == 0 {
			sess.MaxTelegramTurns = 50
		}
		if sess.TrackedTurns == nil {
			sess.TrackedTurns = make([]TurnMessageEntry, 0)
		}
	}
	return nil
}

func (sm *SessionManager) SaveAll() error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	dir := filepath.Dir(sm.filePath)
	if dir != "." && dir != "" {
		_ = os.MkdirAll(dir, 0755)
	}

	data, err := json.MarshalIndent(sm.sessions, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(sm.filePath, data, 0644)
}

func (sm *SessionManager) GetSession(userID int64, chatID int64) *UserSession {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sess, ok := sm.sessions[userID]
	if !ok {
		sess = &UserSession{
			UserID:               userID,
			ChatID:               chatID,
			ActiveConversationID: "",
			CWD:                  sm.defaultRoot,
			PermissionMode:       sm.defaultPerm,
			LastActiveTime:       time.Now(),
			RecentConversations:  make([]ConversationEntry, 0),
			Language:             "id",
			MaxTelegramTurns:     50,
			TrackedTurns:         make([]TurnMessageEntry, 0),
		}
		sm.sessions[userID] = sess
		go func() { _ = sm.SaveAll() }()
	} else {
		if chatID != 0 {
			sess.ChatID = chatID
		}
		sess.LastActiveTime = time.Now()
		if sess.CWD == "" {
			sess.CWD = sm.defaultRoot
		}
		if sess.PermissionMode == "" {
			sess.PermissionMode = sm.defaultPerm
		}
		if sess.Language == "" {
			sess.Language = "id"
		}
		if sess.MaxTelegramTurns == 0 {
			sess.MaxTelegramTurns = 50
		}
	}

	return sess
}

func (sm *SessionManager) UpdateCWD(userID int64, newCWD string) {
	sm.mu.Lock()
	if sess, ok := sm.sessions[userID]; ok {
		sess.CWD = filepath.Clean(newCWD)
		sess.LastActiveTime = time.Now()
	}
	sm.mu.Unlock()
	go func() { _ = sm.SaveAll() }()
}

func (sm *SessionManager) SetPermissionMode(userID int64, mode string) {
	sm.mu.Lock()
	if sess, ok := sm.sessions[userID]; ok {
		sess.PermissionMode = mode
		sess.LastActiveTime = time.Now()
	}
	sm.mu.Unlock()
	go func() { _ = sm.SaveAll() }()
}

func (sm *SessionManager) SetModel(userID int64, model string) {
	sm.mu.Lock()
	if sess, ok := sm.sessions[userID]; ok {
		sess.ActiveModel = model
		sess.LastActiveTime = time.Now()
	}
	sm.mu.Unlock()
	go func() { _ = sm.SaveAll() }()
}

func (sm *SessionManager) SetEffort(userID int64, effort string) {
	sm.mu.Lock()
	if sess, ok := sm.sessions[userID]; ok {
		sess.ActiveEffort = effort
		sess.LastActiveTime = time.Now()
	}
	sm.mu.Unlock()
	go func() { _ = sm.SaveAll() }()
}

func (sm *SessionManager) UpdateConversation(userID int64, convID string, title string) {
	sm.mu.Lock()
	if sess, ok := sm.sessions[userID]; ok {
		sess.ActiveConversationID = convID
		sess.LastActiveTime = time.Now()

		// Add to recent if not already at the top
		var filtered []ConversationEntry
		for _, c := range sess.RecentConversations {
			if c.ID != convID {
				filtered = append(filtered, c)
			}
		}

		if title == "" {
			if len(convID) >= 8 {
				title = "Conversation " + convID[:8]
			} else {
				title = "Conversation " + convID
			}
		}

		newEntry := ConversationEntry{
			ID:        convID,
			Title:     title,
			CreatedAt: time.Now(),
			CWD:       sess.CWD,
		}

		// Keep up to 20 recent conversations
		sess.RecentConversations = append([]ConversationEntry{newEntry}, filtered...)
		if len(sess.RecentConversations) > 20 {
			sess.RecentConversations = sess.RecentConversations[:20]
		}
	}
	sm.mu.Unlock()
	go func() { _ = sm.SaveAll() }()
}

func (sm *SessionManager) ResetConversation(userID int64) {
	sm.mu.Lock()
	if sess, ok := sm.sessions[userID]; ok {
		sess.ActiveConversationID = ""
		sess.LastActiveTime = time.Now()
	}
	sm.mu.Unlock()
	go func() { _ = sm.SaveAll() }()
}

func (sm *SessionManager) SetLanguage(userID int64, lang string) {
	sm.mu.Lock()
	if sess, ok := sm.sessions[userID]; ok {
		sess.Language = lang
		sess.LastActiveTime = time.Now()
	}
	sm.mu.Unlock()
	go func() { _ = sm.SaveAll() }()
}

func (sm *SessionManager) SetMaxTelegramTurns(userID int64, maxTurns int) {
	sm.mu.Lock()
	if sess, ok := sm.sessions[userID]; ok {
		sess.MaxTelegramTurns = maxTurns
		sess.LastActiveTime = time.Now()
	}
	sm.mu.Unlock()
	go func() { _ = sm.SaveAll() }()
}

// RecordTurnMessages appends a completed turn (userMsgID + botMsgID).
// If tracked turns exceed MaxTelegramTurns (when limit > 0), excess turns are evicted
// and returned so the caller can delete them from Telegram.
func (sm *SessionManager) RecordTurnMessages(userID int64, userMsgID, botMsgID int) []TurnMessageEntry {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sess, ok := sm.sessions[userID]
	if !ok {
		return nil
	}

	sess.TrackedTurns = append(sess.TrackedTurns, TurnMessageEntry{
		UserMsgID: userMsgID,
		BotMsgID:  botMsgID,
		Timestamp: time.Now(),
	})
	sess.LastActiveTime = time.Now()

	limit := sess.MaxTelegramTurns
	if limit <= 0 {
		// Limit <= 0 means disabled (e.g. -1 or 0)
		go func() { _ = sm.SaveAll() }()
		return nil
	}

	var toDelete []TurnMessageEntry
	if len(sess.TrackedTurns) > limit {
		excess := len(sess.TrackedTurns) - limit
		toDelete = make([]TurnMessageEntry, excess)
		copy(toDelete, sess.TrackedTurns[:excess])
		sess.TrackedTurns = sess.TrackedTurns[excess:]
	}

	go func() { _ = sm.SaveAll() }()
	return toDelete
}

// ClearAllTrackedTurns empties the tracked turns slice and returns all previously tracked turns
func (sm *SessionManager) ClearAllTrackedTurns(userID int64) []TurnMessageEntry {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sess, ok := sm.sessions[userID]
	if !ok {
		return nil
	}

	all := make([]TurnMessageEntry, len(sess.TrackedTurns))
	copy(all, sess.TrackedTurns)
	sess.TrackedTurns = make([]TurnMessageEntry, 0)
	sess.LastActiveTime = time.Now()

	go func() { _ = sm.SaveAll() }()
	return all
}

