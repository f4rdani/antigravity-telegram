package session

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "modernc.org/sqlite"
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
	TimeLabel string
}

func (sm *SessionManager) GetAvailableConversations(userID int64) []AvailableConversation {
	var result []AvailableConversation
	seen := make(map[string]bool)

	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		homeDir = os.Getenv("HOME")
		if homeDir == "" {
			homeDir = "/root"
		}
	}
	if homeDir != "" {
		// 1. Primary Source: conversation_summaries.db from Antigravity CLI
		dbPath := filepath.Join(homeDir, ".gemini", "antigravity-cli", "conversation_summaries.db")
		if _, err := os.Stat(dbPath); err == nil {
			db, err := sql.Open("sqlite", dbPath+"?mode=ro")
			if err == nil {
				defer db.Close()

				query := `SELECT conversation_id, preview, workspace_uris, last_modified_time 
				          FROM conversation_summaries 
				          ORDER BY last_modified_time DESC LIMIT 10;`
				rows, err := db.Query(query)
				if err == nil {
					defer rows.Close()
					for rows.Next() {
						var id, preview, wsJSON, modTimeStr string
						if err := rows.Scan(&id, &preview, &wsJSON, &modTimeStr); err == nil {
							if id != "" && !seen[id] {
								seen[id] = true

								// Parse title
								title := strings.TrimSpace(preview)
								if title == "" {
									if len(id) >= 8 {
										title = "Percakapan " + id[:8]
									} else {
										title = "Percakapan " + id
									}
								}
								runes := []rune(title)
								if len(runes) > 32 {
									title = string(runes[:32]) + "..."
								}

								// Parse workspace URI
								ws := cleanWorkspaceURI(wsJSON)

								// Parse time
								modTime, err := time.Parse(time.RFC3339Nano, modTimeStr)
								if err != nil {
									modTime, _ = time.Parse("2006-01-02 15:04:05.999999999-07:00", modTimeStr)
								}
								if modTime.IsZero() {
									modTime = time.Now()
								}

								result = append(result, AvailableConversation{
									ID:        id,
									Title:     title,
									Workspace: ws,
									Time:      modTime,
									TimeLabel: formatFriendlyTime(modTime),
								})

								if len(result) >= 7 {
									break
								}
							}
						}
					}
				}
			}
		}
	}

	// 2. Secondary Source: bot session store if not already added
	sess := sm.GetSession(userID, 0)
	for _, c := range sess.RecentConversations {
		if c.ID != "" && !seen[c.ID] {
			seen[c.ID] = true
			result = append(result, AvailableConversation{
				ID:        c.ID,
				Title:     c.Title,
				Workspace: c.CWD,
				Time:      c.CreatedAt,
				TimeLabel: formatFriendlyTime(c.CreatedAt),
			})
			if len(result) >= 7 {
				break
			}
		}
	}

	return result
}

func cleanWorkspaceURI(raw string) string {
	if raw == "" {
		return ""
	}
	var uris []string
	if err := json.Unmarshal([]byte(raw), &uris); err == nil && len(uris) > 0 {
		raw = uris[0]
	}
	if strings.HasPrefix(raw, "file://") {
		raw = strings.TrimPrefix(raw, "file://")
		if runtime.GOOS != "windows" && !strings.HasPrefix(raw, "/") {
			raw = "/" + raw
		}
	}

	// Decode URL-encoded characters (like %20 -> space)
	if decoded, err := url.PathUnescape(raw); err == nil {
		raw = decoded
	}

	return filepath.Clean(raw)
}

func formatFriendlyTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	diff := time.Since(t)
	if diff < 2*time.Minute {
		return "Baru saja"
	}
	if diff < time.Hour {
		return fmt.Sprintf("%d mnt lalu", int(diff.Minutes()))
	}
	if diff < 24*time.Hour {
		return fmt.Sprintf("%d jam lalu", int(diff.Hours()))
	}
	if diff < 48*time.Hour {
		return "Kemarin"
	}
	return t.Format("02/01 15:04")
}
