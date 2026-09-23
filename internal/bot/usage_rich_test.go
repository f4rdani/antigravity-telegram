package bot

import (
	"strings"
	"testing"
	"time"
)

func TestFormatUsageRichCardID(t *testing.T) {
	entries, ok := ParseUsageOutput(sampleUsage)
	if !ok {
		t.Fatal("parse failed")
	}
	// Fixed "now": 2026-09-18T10:05:00Z == 17:05 WIB.
	now, _ := time.Parse(time.RFC3339, "2026-09-18T10:05:00Z")
	card := FormatUsageRichCard("id", entries, now)

	for _, want := range []string{
		"# 📊 Kuota Model Antigravity",
		"## ♊ Gemini Models",
		"| Limit | Sisa | Reset dalam |",
		"Limit 5 Jam",
		"Limit Mingguan",
		"**79%**",
		"**100%**",
	} {
		if !strings.Contains(card, want) {
			t.Errorf("FormatUsageRichCard() missing %q, got:\n%s", want, card)
		}
	}
	// No legacy HTML tags may leak into Rich Markdown.
	for _, bad := range []string{"<b>", "<i>", "<code>", "<pre>", "█", "░"} {
		if strings.Contains(card, bad) {
			t.Errorf("FormatUsageRichCard() leaks legacy marker %q, got:\n%s", bad, card)
		}
	}
}

func TestFormatUsageRichCardEN(t *testing.T) {
	entries, ok := ParseUsageOutput(sampleUsage)
	if !ok {
		t.Fatal("parse failed")
	}
	now, _ := time.Parse(time.RFC3339, "2026-09-18T10:05:00Z")
	card := FormatUsageRichCard("en", entries, now)

	for _, want := range []string{
		"# 📊 Antigravity Model Quota",
		"| Limit | Left | Resets in |",
		"5-Hour Limit",
		"Weekly Limit",
	} {
		if !strings.Contains(card, want) {
			t.Errorf("FormatUsageRichCard(en) missing %q, got:\n%s", want, card)
		}
	}
}
