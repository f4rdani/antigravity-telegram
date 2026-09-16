package renderer

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

type TableAlignment int

const (
	AlignLeft TableAlignment = iota
	AlignCenter
	AlignRight
)

var (
	reInlineMD = regexp.MustCompile(`[*_~` + "`" + `]`)
)

// cleanCellContent removes inline markdown markers and unescapes text for display
func cleanCellContent(s string) string {
	s = strings.TrimSpace(s)
	s = StripHTML(s)
	// Remove bold/italic/code markers: **text** -> text, `code` -> code
	s = reInlineMD.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

func isTableRow(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.Contains(trimmed, "|") {
		return false
	}
	cells := splitTableCells(trimmed)
	return len(cells) >= 2
}

func isTableSeparator(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.Contains(trimmed, "|") || !strings.Contains(trimmed, "-") {
		return false
	}
	cells := splitTableCells(trimmed)
	if len(cells) < 2 {
		return false
	}
	for _, cell := range cells {
		c := strings.TrimSpace(cell)
		if len(c) == 0 {
			return false
		}
		for _, r := range c {
			if r != '-' && r != ':' && r != ' ' {
				return false
			}
		}
		if !strings.ContainsRune(c, '-') {
			return false
		}
	}
	return true
}

func splitTableCells(line string) []string {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "|") {
		trimmed = trimmed[1:]
	}
	if strings.HasSuffix(trimmed, "|") {
		trimmed = trimmed[:len(trimmed)-1]
	}
	parts := strings.Split(trimmed, "|")
	var cells []string
	for _, p := range parts {
		cells = append(cells, strings.TrimSpace(p))
	}
	return cells
}

func parseAlignments(sepLine string, colCount int) []TableAlignment {
	cells := splitTableCells(sepLine)
	alignments := make([]TableAlignment, colCount)
	for i := 0; i < colCount; i++ {
		alignments[i] = AlignLeft
		if i < len(cells) {
			c := strings.TrimSpace(cells[i])
			hasLeftColon := strings.HasPrefix(c, ":")
			hasRightColon := strings.HasSuffix(c, ":")
			if hasLeftColon && hasRightColon {
				alignments[i] = AlignCenter
			} else if hasRightColon {
				alignments[i] = AlignRight
			}
		}
	}
	return alignments
}

func padCell(s string, width int, align TableAlignment) string {
	runeCount := utf8.RuneCountInString(s)
	if runeCount >= width {
		return EscapeHTML(s)
	}
	diff := width - runeCount
	escaped := EscapeHTML(s)
	switch align {
	case AlignRight:
		return strings.Repeat(" ", diff) + escaped
	case AlignCenter:
		left := diff / 2
		right := diff - left
		return strings.Repeat(" ", left) + escaped + strings.Repeat(" ", right)
	default:
		return escaped + strings.Repeat(" ", diff)
	}
}

// FormatMarkdownTables finds markdown tables in lines and replaces them with beautiful monospace box tables
func FormatMarkdownTables(text string) string {
	lines := strings.Split(text, "\n")
	var result []string
	i := 0

	for i < len(lines) {
		if i+1 < len(lines) && isTableRow(lines[i]) && isTableSeparator(lines[i+1]) {
			// Found table start!
			rawHeader := splitTableCells(lines[i])
			var header []string
			for _, h := range rawHeader {
				header = append(header, cleanCellContent(h))
			}

			colCount := len(header)
			alignments := parseAlignments(lines[i+1], colCount)

			// Collect data rows
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

			// Calculate maximum column widths
			colWidths := make([]int, colCount)
			for col := 0; col < colCount; col++ {
				colWidths[col] = utf8.RuneCountInString(header[col])
			}

			for _, row := range rows {
				for col := 0; col < colCount; col++ {
					if col < len(row) {
						w := utf8.RuneCountInString(row[col])
						if w > colWidths[col] {
							colWidths[col] = w
						}
					}
				}
			}

			tableBlock := renderTableBlock(header, rows, alignments)
			result = append(result, tableBlock)
			i = j
		} else {
			result = append(result, lines[i])
			i++
		}
	}

	return strings.Join(result, "\n")
}

func renderTableBlock(header []string, rows [][]string, alignments []TableAlignment) string {
	colCount := len(header)
	colWidths := make([]int, colCount)
	for col := 0; col < colCount; col++ {
		colWidths[col] = utf8.RuneCountInString(header[col])
	}

	for _, row := range rows {
		for col := 0; col < colCount; col++ {
			if col < len(row) {
				w := utf8.RuneCountInString(row[col])
				if w > colWidths[col] {
					colWidths[col] = w
				}
			}
		}
	}

	var sb strings.Builder
	sb.WriteString("<pre>")

	// Top border: ┌───┬───┐
	sb.WriteString("┌")
	for col, w := range colWidths {
		if col > 0 {
			sb.WriteString("┬")
		}
		sb.WriteString(strings.Repeat("─", w+2))
	}
	sb.WriteString("┐\n")

	// Header row: │ Col1 │ Col2 │
	sb.WriteString("│")
	for col, h := range header {
		sb.WriteString(" ")
		sb.WriteString(padCell(h, colWidths[col], alignments[col]))
		sb.WriteString(" │")
	}
	sb.WriteString("\n")

	// Header separator: ├───┼───┤
	sb.WriteString("├")
	for col, w := range colWidths {
		if col > 0 {
			sb.WriteString("┼")
		}
		sb.WriteString(strings.Repeat("─", w+2))
	}
	sb.WriteString("┤\n")

	// Data rows
	for _, row := range rows {
		sb.WriteString("│")
		for col := 0; col < colCount; col++ {
			cellVal := ""
			if col < len(row) {
				cellVal = row[col]
			}
			sb.WriteString(" ")
			sb.WriteString(padCell(cellVal, colWidths[col], alignments[col]))
			sb.WriteString(" │")
		}
		sb.WriteString("\n")
	}

	// Bottom border: └───┴───┘
	sb.WriteString("└")
	for col, w := range colWidths {
		if col > 0 {
			sb.WriteString("┴")
		}
		sb.WriteString(strings.Repeat("─", w+2))
	}
	sb.WriteString("┘</pre>")

	return sb.String()
}
