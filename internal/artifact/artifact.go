package artifact

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Metadata struct {
	Summary         string `json:"summary"`
	UpdatedAt       string `json:"updatedAt"`
	RequestFeedback bool   `json:"requestFeedback"`
	UserFacing      bool   `json:"userFacing"`
}

type Item struct {
	ID              string
	FileName        string
	Path            string
	SizeBytes       int64
	Summary         string
	RequestFeedback bool
	ModTime         time.Time
	TimeLabel       string
	ConversationID  string
}

func GetBrainDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".gemini", "antigravity-cli", "brain"), nil
}

// ListArtifacts returns all artifacts for the conversation (or across recent conversations if convID is empty or yields 0)
func ListArtifacts(convID string) ([]Item, error) {
	brainDir, err := GetBrainDir()
	if err != nil {
		return nil, err
	}

	var targetDirs []string
	if convID != "" {
		targetDirs = append(targetDirs, filepath.Join(brainDir, convID))
	}

	items := scanDirs(targetDirs)
	// If active conversation had no artifacts, look into recent brain directories
	if len(items) == 0 {
		entries, err := os.ReadDir(brainDir)
		if err == nil {
			// Sort directories by mod time
			type dirEntry struct {
				path    string
				modTime time.Time
			}
			var dirs []dirEntry
			for _, e := range entries {
				if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
					p := filepath.Join(brainDir, e.Name())
					if fi, err := e.Info(); err == nil {
						dirs = append(dirs, dirEntry{path: p, modTime: fi.ModTime()})
					}
				}
			}
			sort.Slice(dirs, func(i, j int) bool {
				return dirs[i].modTime.After(dirs[j].modTime)
			})
			var recentDirs []string
			for i, d := range dirs {
				if i >= 5 {
					break
				}
				recentDirs = append(recentDirs, d.path)
			}
			items = scanDirs(recentDirs)
		}
	}

	// Sort newest first
	sort.Slice(items, func(i, j int) bool {
		return items[i].ModTime.After(items[j].ModTime)
	})

	return items, nil
}

func scanDirs(dirs []string) []Item {
	var items []Item
	seen := make(map[string]bool)

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		cID := filepath.Base(dir)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".metadata.json") {
				filePath := filepath.Join(dir, name)
				if seen[filePath] {
					continue
				}
				seen[filePath] = true

				info, err := e.Info()
				if err != nil {
					continue
				}

				baseID := strings.TrimSuffix(name, ".md")
				metaPath := filePath + ".metadata.json"
				var meta Metadata
				if mData, err := os.ReadFile(metaPath); err == nil {
					_ = json.Unmarshal(mData, &meta)
				}

				summary := meta.Summary
				if summary == "" {
					if content, err := os.ReadFile(filePath); err == nil {
						lines := strings.Split(string(content), "\n")
						for _, l := range lines {
							l = strings.TrimSpace(l)
							if l != "" && !strings.HasPrefix(l, "#") {
								summary = l
								if len(summary) > 120 {
									summary = summary[:117] + "..."
								}
								break
							}
						}
					}
				}

				items = append(items, Item{
					ID:              baseID,
					FileName:        name,
					Path:            filePath,
					SizeBytes:       info.Size(),
					Summary:         summary,
					RequestFeedback: meta.RequestFeedback,
					ModTime:         info.ModTime(),
					TimeLabel:       formatFriendlyTime(info.ModTime()),
					ConversationID:  cID,
				})
			}
		}
	}

	return items
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
