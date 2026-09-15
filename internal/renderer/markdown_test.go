package renderer

import (
	"strings"
	"testing"
)

func TestEscapeHTML(t *testing.T) {
	input := `<b>Hello & "World" <tag></b>`
	expected := `&lt;b&gt;Hello &amp; "World" &lt;tag&gt;&lt;/b&gt;`
	got := EscapeHTML(input)
	if got != expected {
		t.Errorf("EscapeHTML() = %q, want %q", got, expected)
	}
}

func TestFormatMarkdownForTelegram(t *testing.T) {
	// Test bold, italic, inline code
	input := "Hello **world**, this is *italic* and `code`."
	got := FormatMarkdownForTelegram(input)

	if !strings.Contains(got, "<b>world</b>") {
		t.Errorf("FormatMarkdownForTelegram() missing bold: %s", got)
	}
	if !strings.Contains(got, "<i>italic</i>") {
		t.Errorf("FormatMarkdownForTelegram() missing italic: %s", got)
	}
	if !strings.Contains(got, "<code>code</code>") {
		t.Errorf("FormatMarkdownForTelegram() missing code: %s", got)
	}

	// Test sanitization of file:/// links
	fileLinkInput := "Focused on [`C:/project/app`](file:///C:/project/app)."
	gotFileLink := FormatMarkdownForTelegram(fileLinkInput)
	if !strings.Contains(gotFileLink, "<code>C:/project/app</code>") {
		t.Errorf("FormatMarkdownForTelegram() failed to clean file link: %s", gotFileLink)
	}
	if strings.Contains(gotFileLink, "file:///") {
		t.Errorf("FormatMarkdownForTelegram() still contains raw file link: %s", gotFileLink)
	}
}
