package bot

import (
	"fmt"
	"strings"
	"time"

	"agy-tele/internal/artifact"
	"agy-tele/internal/engine"
	"agy-tele/internal/i18n"
	"agy-tele/internal/renderer"
	"agy-tele/internal/session"
)

const AppVersion = "v1.0.16"

func FormatHelp(lang string) string {
	return i18n.GetHelpText(lang, AppVersion)
}

func FormatStatus(lang string, sess *session.UserSession, isRunning bool, activeTaskDesc string) string {
	return i18n.GetStatusText(lang, sess, isRunning, activeTaskDesc)
}

// FormatStatusWithQueue appends an explicit FIFO queue section to the status card
// so /status always shows both the active task and pending queue — no ambiguity.
func FormatStatusWithQueue(lang string, sess *session.UserSession, isRunning bool, activeTaskDesc string, queueSection string) string {
	base := i18n.GetStatusText(lang, sess, isRunning, activeTaskDesc)
	if queueSection == "" {
		return base
	}
	return base + "\n\n" + queueSection
}

// FormatQueueSection builds a short queue summary for embedding into /status.
func FormatQueueSection(lang string, items []*QueuedItem) string {
	if len(items) == 0 {
		if lang == "en" {
			return "• <b>Queue</b>: <i>empty (0 pending)</i>"
		}
		return "• <b>Antrean</b>: <i>kosong (0 tertunda)</i>"
	}
	var sb strings.Builder
	if lang == "en" {
		sb.WriteString(fmt.Sprintf("• <b>Queue</b>: <code>%d pending</code> (FIFO, auto-run in order)\n", len(items)))
	} else {
		sb.WriteString(fmt.Sprintf("• <b>Antrean</b>: <code>%d tertunda</code> (FIFO, jalan otomatis berurutan)\n", len(items)))
	}
	for i, it := range items {
		if i >= 5 {
			sb.WriteString(fmt.Sprintf("<i>... and %d more (use /queue for full list)</i>", len(items)-i))
			break
		}
		sb.WriteString(fmt.Sprintf("  %d. <i>\"%s\"</i>\n", i+1, EscapeHTML(truncatePreview(it.Prompt, 70))))
	}
	return strings.TrimRight(sb.String(), "\n")
}

// FormatQueueList builds the full /queue card: active task + pending FIFO items.
func FormatQueueList(lang string, task *ActiveTask, items []*QueuedItem) string {
	var sb strings.Builder
	if lang == "en" {
		sb.WriteString("📋 <b>Message Queue (FIFO)</b>\n\n")
		if task != nil {
			dur := time.Since(task.StartedAt).Round(time.Second)
			sb.WriteString(fmt.Sprintf("🔄 <b>Active:</b> <i>\"%s\"</i> (%s elapsed)\n\n", EscapeHTML(truncatePreview(task.Prompt, 100)), dur.String()))
		} else {
			sb.WriteString("🟢 <b>Status:</b> IDLE — no active task.\n\n")
		}
		if len(items) == 0 {
			sb.WriteString("<i>Queue is empty. Send any message and it will run immediately when idle, or be queued with a position number when busy.</i>")
		} else {
			sb.WriteString(fmt.Sprintf("<b>%d pending (auto-run in order):</b>\n", len(items)))
			for i, it := range items {
				sb.WriteString(fmt.Sprintf("%d. <i>\"%s\"</i>\n", i+1, EscapeHTML(truncatePreview(it.Prompt, 120))))
			}
			sb.WriteString("\n<i>New messages join at the end. /clearqueue drops pending, /cancel stops the active task.</i>")
		}
	} else {
		sb.WriteString("📋 <b>Antrean Pesan (FIFO)</b>\n\n")
		if task != nil {
			dur := time.Since(task.StartedAt).Round(time.Second)
			sb.WriteString(fmt.Sprintf("🔄 <b>Aktif:</b> <i>\"%s\"</i> (durasi %s)\n\n", EscapeHTML(truncatePreview(task.Prompt, 100)), dur.String()))
		} else {
			sb.WriteString("🟢 <b>Status:</b> IDLE — tidak ada tugas aktif.\n\n")
		}
		if len(items) == 0 {
			sb.WriteString("<i>Antrean kosong. Kirim pesan kapan saja: langsung jalan saat idle, atau masuk antrean bernomor saat sibuk.</i>")
		} else {
			sb.WriteString(fmt.Sprintf("<b>%d tertunda (jalan otomatis berurutan):</b>\n", len(items)))
			for i, it := range items {
				sb.WriteString(fmt.Sprintf("%d. <i>\"%s\"</i>\n", i+1, EscapeHTML(truncatePreview(it.Prompt, 120))))
			}
			sb.WriteString("\n<i>Pesan baru masuk di urutan akhir. /clearqueue menghapus antrean, /cancel menghentikan tugas aktif.</i>")
		}
	}
	return sb.String()
}

func FormatSessions(lang string, convs []session.AvailableConversation, activeID string) string {
	return i18n.GetSessionsText(lang, convs, activeID)
}

func FormatArtifacts(lang string, items []artifact.Item) string {
	return i18n.GetArtifactsText(lang, items)
}

func FormatArtifactDetail(lang string, it artifact.Item) string {
	return i18n.GetArtifactDetailText(lang, it)
}

func FormatCodeBlock(title string, content string) string {
	return fmt.Sprintf("<b>%s</b>\n<pre>%s</pre>", title, EscapeHTML(content))
}

func FormatResultFooter(res *engine.ResultPayload) string {
	if res == nil {
		return ""
	}

	var parts []string
	if res.DurationSeconds > 0 {
		parts = append(parts, fmt.Sprintf("⏱ %.1fs", res.DurationSeconds))
	}
	if res.Usage != nil && res.Usage.TotalTokens > 0 {
		parts = append(parts, fmt.Sprintf("🪙 %d tokens", res.Usage.TotalTokens))
	}
	if res.NumTurns > 0 {
		parts = append(parts, fmt.Sprintf("🔄 Turn %d", res.NumTurns))
	}
	if res.ConversationID != "" {
		convID := res.ConversationID
		if len(convID) > 8 {
			convID = convID[:8]
		}
		parts = append(parts, fmt.Sprintf("🆔 %s", convID))
	}

	parts = append(parts, fmt.Sprintf("🏷️ %s", AppVersion))
	return "<i>(" + strings.Join(parts, " • ") + ")</i>"
}

func EscapeHTML(s string) string {
	return renderer.EscapeHTML(s)
}

func FormatMarkdownForTelegram(s string) string {
	return renderer.FormatMarkdownForTelegram(s)
}

func StripHTML(s string) string {
	return renderer.StripHTML(s)
}

func DescribeStepAction(lang string, step *engine.StepUpdatePayload) string {
	return i18n.DescribeStepAction(lang, step)
}

func FormatProgressStatus(lang string, currentAction string, recentHistory []string) string {
	return i18n.FormatProgressStatus(lang, currentAction, recentHistory)
}

func FormatActivityBadge(lang string, step *engine.StepUpdatePayload) string {
	return i18n.FormatActivityBadge(lang, step)
}
