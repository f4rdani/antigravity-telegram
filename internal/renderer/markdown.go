package renderer

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reFileLink   = regexp.MustCompile(`\[([^\]]+)\]\(file:///[^\)]+\)`)
	reWebLink    = regexp.MustCompile(`\[([^\]]+)\]\((https?://[^\s\)]+)\)`)
	reBold       = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	reItalic     = regexp.MustCompile(`\*([^*]+)\*`)
	reCodeInline = regexp.MustCompile("`([^`]+)`")
	reHTMLTag    = regexp.MustCompile(`<[^>]+>`)
)

func EscapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// FormatMarkdownForTelegram converts raw agent markdown into valid Telegram HTML
func FormatMarkdownForTelegram(raw string) string {
	if raw == "" {
		return ""
	}

	// Separate code blocks (```...```) so formatting inside code is preserved
	parts := strings.Split(raw, "```")
	var sb strings.Builder

	for i, part := range parts {
		if i%2 == 1 {
			// Inside fenced code block
			lang := ""
			codeContent := part
			if idx := strings.IndexByte(part, '\n'); idx != -1 {
				firstLine := strings.TrimSpace(part[:idx])
				if len(firstLine) > 0 && !strings.ContainsAny(firstLine, " \t\r") {
					lang = firstLine
					codeContent = part[idx+1:]
				}
			}
			codeContent = EscapeHTML(codeContent)
			if lang != "" {
				sb.WriteString(fmt.Sprintf("<pre><code class=\"language-%s\">%s</code></pre>", lang, codeContent))
			} else {
				sb.WriteString(fmt.Sprintf("<pre><code>%s</code></pre>", codeContent))
			}
		} else {
			// Outside code block
			text := EscapeHTML(part)

			// Clean file:/// links: [path](file:///...) or [`path`](file:///...) -> <code>path</code>
			text = reFileLink.ReplaceAllStringFunc(text, func(m string) string {
				sub := reFileLink.FindStringSubmatch(m)
				if len(sub) > 1 {
					cleanName := strings.Trim(sub[1], "` \t\r\n")
					return "<code>" + cleanName + "</code>"
				}
				return m
			})

			// Format clickable web links [label](https://...) -> <a href="https://...">label</a>
			text = reWebLink.ReplaceAllString(text, `<a href="$2">$1</a>`)

			// Inline code: `code` -> <code>code</code>
			text = reCodeInline.ReplaceAllString(text, "<code>$1</code>")

			// Bold: **text** -> <b>text</b>
			text = reBold.ReplaceAllString(text, "<b>$1</b>")

			// Italic: *text* -> <i>text</i>
			text = reItalic.ReplaceAllString(text, "<i>$1</i>")

			// Convert headers and bullet points
			lines := strings.Split(text, "\n")
			for j, line := range lines {
				trimmed := strings.TrimLeft(line, " \t")
				indent := line[:len(line)-len(trimmed)]
				if strings.HasPrefix(trimmed, "- ") {
					lines[j] = indent + "• " + trimmed[2:]
				} else if strings.HasPrefix(trimmed, "* ") {
					lines[j] = indent + "• " + trimmed[2:]
				} else if strings.HasPrefix(trimmed, "### ") {
					lines[j] = indent + "<b>" + trimmed[4:] + "</b>"
				} else if strings.HasPrefix(trimmed, "## ") {
					lines[j] = indent + "<b>" + trimmed[3:] + "</b>"
				} else if strings.HasPrefix(trimmed, "# ") {
					lines[j] = indent + "<b>" + trimmed[2:] + "</b>"
				}
			}
			sb.WriteString(strings.Join(lines, "\n"))
		}
	}

	return sb.String()
}

// StripHTML removes all HTML tags as fallback when Telegram HTML parser fails
func StripHTML(s string) string {
	s = reHTMLTag.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	return s
}
