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
			sb.WriteString(formatOutsideCode(part))
		}
	}

	return sb.String()
}

func formatOutsideCode(part string) string {
	lines := strings.Split(part, "\n")
	var result []string
	var quoteLines []string
	i := 0

	flushQuotes := func() {
		if len(quoteLines) > 0 {
			result = append(result, "<blockquote>"+strings.Join(quoteLines, "\n")+"</blockquote>")
			quoteLines = nil
		}
	}

	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimLeft(line, " \t")

		// 1. Table Detection
		if i+1 < len(lines) && isTableRow(line) && isTableSeparator(lines[i+1]) {
			flushQuotes()

			rawHeader := splitTableCells(line)
			var header []string
			for _, h := range rawHeader {
				header = append(header, cleanCellContent(h))
			}

			colCount := len(header)
			alignments := parseAlignments(lines[i+1], colCount)

			var rows [][]string
			j := i + 2
			for j < len(lines) && isTableRow(lines[j]) && !isTableSeparator(lines[j]) {
				rawRow := splitTableCells(lines[j])
				var row []string
				for _, r := range rawRow {
					row = append(row, cleanCellContent(r))
				}
				rows = append(rows, row)
				j++
			}

			tableBlock := renderTableBlock(header, rows, alignments)
			result = append(result, tableBlock)
			i = j
			continue
		}

		// 2. Blockquote Detection: > Quote
		if strings.HasPrefix(trimmed, ">") {
			content := strings.TrimPrefix(trimmed, ">")
			content = strings.TrimLeft(content, " ")
			quoteLines = append(quoteLines, formatInlineMarkdown(content))
			i++
			continue
		}

		// Normal line: flush pending quotes first
		flushQuotes()

		// 3. Horizontal Rule
		if trimmed == "---" || trimmed == "***" || trimmed == "___" {
			result = append(result, "──────────────────────────")
			i++
			continue
		}

		// 4. Headers and Bullet Points
		indent := getLineIndent(line)
		if strings.HasPrefix(trimmed, "### ") {
			result = append(result, indent+"<b>"+formatInlineMarkdown(trimmed[4:])+"</b>")
		} else if strings.HasPrefix(trimmed, "## ") {
			result = append(result, indent+"<b>"+formatInlineMarkdown(trimmed[3:])+"</b>")
		} else if strings.HasPrefix(trimmed, "# ") {
			result = append(result, indent+"<b>"+formatInlineMarkdown(trimmed[2:])+"</b>")
		} else if strings.HasPrefix(trimmed, "- ") {
			result = append(result, indent+"• "+formatInlineMarkdown(trimmed[2:]))
		} else if strings.HasPrefix(trimmed, "* ") {
			result = append(result, indent+"• "+formatInlineMarkdown(trimmed[2:]))
		} else {
			result = append(result, indent+formatInlineMarkdown(trimmed))
		}
		i++
	}

	flushQuotes()
	return strings.Join(result, "\n")
}

func getLineIndent(line string) string {
	var sb strings.Builder
	for _, r := range line {
		if r == ' ' || r == '\t' {
			sb.WriteRune(r)
		} else {
			break
		}
	}
	return sb.String()
}

func formatInlineMarkdown(text string) string {
	if text == "" {
		return ""
	}

	text = EscapeHTML(text)

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

	return text
}

// StripHTML removes all HTML tags as fallback when Telegram HTML parser fails
func StripHTML(s string) string {
	s = reHTMLTag.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	return s
}
