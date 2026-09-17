package bot

import (
	"fmt"
	"strings"

	"agy-tele/internal/artifact"
	"agy-tele/internal/engine"
	"agy-tele/internal/i18n"
	"agy-tele/internal/renderer"
	"agy-tele/internal/session"
)

const AppVersion = "v1.0.12"

func FormatHelp(lang string) string {
	return i18n.GetHelpText(lang, AppVersion)
}

func FormatStatus(lang string, sess *session.UserSession, isRunning bool, activeTaskDesc string) string {
	return i18n.GetStatusText(lang, sess, isRunning, activeTaskDesc)
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
