package session

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type HistoryEntry struct {
	Display        string `json:"display"`
	Timestamp      int64  `json:"timestamp"`
	Workspace      string `json:"workspace"`
	ConversationID string `json:"conversationId"`
	Type           string `json:"type"`
}

type AvailableConversation struct {
	ID        string
	Title     string
	Workspace string
	Time      time.Time
}

func (sm *SessionManager) GetAvailableConversations(userID int64) []AvailableConversation {
	sess := sm.GetSession(userID, 0)

	var result []AvailableConversation
	seen := make(map[string]bool)

	// 1. From active bot session
	for _, c := range sess.RecentConversations {
		if c.ID != "" && !seen[c.ID] {
			seen[c.ID] = true
			result = append(result, AvailableConversation{
				ID:        c.ID,
				Title:     c.Title,
				Workspace: c.CWD,
				Time:      c.CreatedAt,
			})
		}
	}

	// 2. From agy history.jsonl (works on both Linux ~/.gemini/ and Windows C:\Users\~/.gemini/)
	homeDir, err := os.UserHomeDir()
	if err == nil {
		historyPath := filepath.Join(homeDir, ".gemini", "antigravity-cli", "history.jsonl")
		if f, err := os.Open(historyPath); err == nil {
			defer f.Close()

			var lines []string
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				lines = append(lines, scanner.Text())
			}

			// Traverse lines in reverse (newest first)
			for i := len(lines) - 1; i >= 0; i-- {
				var entry HistoryEntry
				if err := json.Unmarshal([]byte(lines[i]), &entry); err == nil {
					if entry.ConversationID != "" && !seen[entry.ConversationID] {
						seen[entry.ConversationID] = true

						title := strings.TrimSpace(entry.Display)
						if strings.HasPrefix(title, "/") || title == "" {
							title = "Sesi " + entry.ConversationID[:8]
						}
						runes := []rune(title)
						if len(runes) > 30 {
							title = string(runes[:30]) + "..."
						}

						t := time.UnixMilli(entry.Timestamp)
						if entry.Timestamp <= 0 {
							t = time.Now()
						}

						result = append(result, AvailableConversation{
							ID:        entry.ConversationID,
							Title:     title,
							Workspace: entry.Workspace,
							Time:      t,
						})

						if len(result) >= 8 {
							break
						}
					}
				}
			}
		}
	}

	return result
}
