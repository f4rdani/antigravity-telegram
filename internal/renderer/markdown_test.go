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

	// Test web link conversion
	webLinkInput := "Visit [Dashboard](https://status.miniapp.web.id) for details."
	gotWebLink := FormatMarkdownForTelegram(webLinkInput)
	if !strings.Contains(gotWebLink, `<a href="https://status.miniapp.web.id">Dashboard</a>`) {
		t.Errorf("FormatMarkdownForTelegram() failed to format web link: %s", gotWebLink)
	}
}

func TestStripHTML(t *testing.T) {
	input := `<pre><code class="language-go">fmt.Println("hello")</code></pre> and <b>bold</b>`
	expected := `fmt.Println("hello") and bold`
	got := StripHTML(input)
	if got != expected {
		t.Errorf("StripHTML() = %q, want %q", got, expected)
	}
}
