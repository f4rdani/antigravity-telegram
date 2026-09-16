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
