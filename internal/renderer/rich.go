package renderer

import (
	"strings"
)

// Rich message limits (Telegram Bot API 10.1+):
// - Up to 32768 UTF-8 characters in rich message text
// - Up to 500 blocks, 16 nesting levels, 20 table columns
const (
	RichMaxChars   = 32768
	RichMaxBlocks  = 500
	RichMaxColumns = 20
)

// ShouldUseRich reports whether a raw agent markdown response benefits from
// native Rich Messages (sendRichMessage with the markdown field) instead of
// the legacy HTML path. Legacy HTML degrades tables (monospace box drawing),
// task lists, <details> blocks, and math — Rich renders them natively.
func ShouldUseRich(raw string) bool {
	if raw == "" {
		return false
	}
	if len(raw) > RichMaxChars {
		return false
	}
	lower := strings.ToLower(raw)
	// Fenced code with language, tables, task lists, details, math
	if strings.Contains(raw, "```") {
		return true
	}
	lines := strings.Split(raw, "\n")
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if isTableRow(ln) && i+1 < len(lines) && isTableSeparator(lines[i+1]) {
			return true
		}
		if strings.HasPrefix(t, "- [ ]") || strings.HasPrefix(t, "- [x]") ||
			strings.HasPrefix(t, "- [X]") || strings.HasPrefix(t, "* [ ]") {
			return true
		}
	}
	if strings.Contains(lower, "<details") || strings.Contains(lower, "<summary") {
		return true
	}
	if strings.Contains(raw, "$$") || strings.Contains(lower, "<tg-math") || strings.Contains(lower, "```math") {
		return true
	}
	if strings.HasPrefix(strings.TrimSpace(raw), "#") {
		return true
	}
	return false
}

// NormalizeForRich converts raw agent markdown into Rich Markdown
// (Telegram Bot API 10.1 "Rich Markdown style").
//
// Rich Markdown is GitHub-Flavored-Markdown compatible and additionally
// supports arbitrary HTML tags, so most agent output passes through
// unchanged. We only fix the constructs our legacy HTML renderer used to
// rewrite:
//
//  1. file:/// links  -> plain inline code (no dead local links in chat)
//  2. Horizontal rules "---" stay as-is (native divider support)
//  3. "# Heading" lines stay as-is (native heading support — the legacy
//     renderer flattened them to <b>, losing hierarchy)
//
// Everything else (tables, task lists, bold/italic/code, blockquotes, code
// fences, details, math) is already valid Rich Markdown and is preserved
// verbatim so the server renders it natively.
func NormalizeForRich(raw string) string {
	if raw == "" {
		return ""
	}
	text := raw
	// Clean file:/// links: [path](file:///...) or [`path`](file:///...) -> `path`
	text = reFileLink.ReplaceAllStringFunc(text, func(m string) string {
		sub := reFileLink.FindStringSubmatch(m)
		if len(sub) > 1 {
			cleanName := strings.Trim(sub[1], "` \t\r\n")
			return "`" + cleanName + "`"
		}
		return m
	})
	return strings.TrimSpace(text) + "\n"
}

// SplitRichChunks splits a rich markdown payload into chunks that fit the
// rich text limit, preferring block boundaries (blank lines) so tables and
// code fences are not torn apart.
func SplitRichChunks(rich string, maxChars int) []string {
	if maxChars <= 0 {
		maxChars = RichMaxChars
	}
	if len(rich) <= maxChars {
		return []string{rich}
	}
	var chunks []string
	var cur strings.Builder
	curLen := 0
	flush := func() {
		if curLen > 0 {
			chunks = append(chunks, cur.String())
			cur.Reset()
			curLen = 0
		}
	}
	for _, para := range strings.Split(rich, "\n\n") {
		if curLen+len(para)+2 > maxChars {
			flush()
			// Single oversized paragraph: hard-split by runes.
			if len(para) > maxChars {
				runes := []rune(para)
				for len(runes) > 0 {
					n := maxChars
					if n > len(runes) {
						n = len(runes)
					}
					chunks = append(chunks, string(runes[:n]))
					runes = runes[n:]
				}
				continue
			}
		}
		if curLen > 0 {
			cur.WriteString("\n\n")
			curLen += 2
		}
		cur.WriteString(para)
		curLen += len(para)
	}
	flush()
	return chunks
}
