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

// ConversationHistoryEntry holds a single user prompt from the transcript.
type ConversationHistoryEntry struct {
	Timestamp string
	Text      string
}

// GetConversationHistory reads the transcript.jsonl for the given conversationID
// and returns the last `limit` user prompts. It strips XML-style tags that the
// Telegram bridge wraps user input in (e.g. <USER_REQUEST>…</USER_REQUEST>).
func GetConversationHistory(conversationID string, limit int) []ConversationHistoryEntry {
	if conversationID == "" || limit <= 0 {
		return nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		homeDir = os.Getenv("HOME")
		if homeDir == "" {
			homeDir = "/root"
		}
	}

	transcriptPath := filepath.Join(homeDir, ".gemini", "antigravity-cli", "brain",
		conversationID, ".system_generated", "logs", "transcript.jsonl")

	f, err := os.Open(transcriptPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	// Read all lines first, then pick last N user inputs
	type rawEntry struct {
		Type      string `json:"type"`
		Content   string `json:"content"`
		CreatedAt string `json:"created_at"`
	}

	var all []ConversationHistoryEntry

	buf := make([]byte, 0, 64*1024)
	tmp := make([]byte, 4096)
	var lineStart int
	fullData := []byte{}

	// Read whole file (transcripts can be large; we only need user lines)
	for {
		n, rerr := f.Read(tmp)
		if n > 0 {
			fullData = append(fullData, tmp[:n]...)
		}
		if rerr != nil {
			break
		}
	}
	_ = buf

	lines := strings.Split(string(fullData), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry rawEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry.Type != "USER_INPUT" || entry.Content == "" {
			continue
		}

		text := stripTranscriptTags(entry.Content)
		if text == "" {
			continue
		}

		ts := ""
		if len(entry.CreatedAt) >= 16 {
			// "2026-09-22T11:21:16+08:00" -> "22/09 19:21"
			t, perr := time.Parse(time.RFC3339, entry.CreatedAt)
			if perr == nil {
				ts = t.Local().Format("02/01 15:04")
			} else {
				ts = entry.CreatedAt[:16]
			}
		}

		all = append(all, ConversationHistoryEntry{
			Timestamp: ts,
			Text:      text,
		})
	}
	_ = lineStart

	if len(all) == 0 {
		return nil
	}

	// Return last `limit` entries
	if len(all) > limit {
		all = all[len(all)-limit:]
	}
	return all
}

// stripTranscriptTags removes XML-style wrapper tags the Telegram bridge inserts:
// <USER_REQUEST>…</USER_REQUEST>, <ADDITIONAL_METADATA>…</ADDITIONAL_METADATA>, etc.
// and returns a clean user message.
func stripTranscriptTags(raw string) string {
	// Remove block tags and their content for metadata sections
	blockers := []string{"ADDITIONAL_METADATA", "USER_SETTINGS_CHANGE"}
	for _, tag := range blockers {
		open := "<" + tag + ">"
		close := "</" + tag + ">"
		for {
			start := strings.Index(raw, open)
			if start < 0 {
				break
			}
			end := strings.Index(raw, close)
			if end < 0 {
				raw = raw[:start]
				break
			}
			raw = raw[:start] + raw[end+len(close):]
		}
	}

	// Strip remaining XML-like tags (keep content inside USER_REQUEST)
	raw = strings.ReplaceAll(raw, "<USER_REQUEST>", "")
	raw = strings.ReplaceAll(raw, "</USER_REQUEST>", "")

	// Collapse multiple blank lines
	lines := strings.Split(raw, "\n")
	var out []string
	blank := 0
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			blank++
			if blank <= 1 {
				out = append(out, "")
			}
		} else {
			blank = 0
			out = append(out, trimmed)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
