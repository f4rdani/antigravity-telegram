package bot

import (
	"strings"
	"testing"
	"time"

	"agy-tele/internal/auth"
)

const sampleUsage = "Gemini Models\tWeekly Limit Remaining\t79%\t2026-09-24T16:01:16Z\n" +
	"Gemini Models\tFive Hour Limit Remaining\t100%\t2026-09-18T07:01:16Z\n" +
	"Claude and GPT models\tWeekly Limit Remaining\t100%\t2026-09-25T02:09:35Z\n" +
	"Claude and GPT models\tFive Hour Limit Remaining\t100%\t2026-09-18T07:09:35Z"

func TestParseUsageOutput(t *testing.T) {
	entries, ok := ParseUsageOutput(sampleUsage)
	if !ok {
		t.Fatal("expected parse ok for sample output")
	}
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}
	if entries[0].Scope != "Gemini Models" || entries[0].Percent != 79 {
		t.Errorf("unexpected first entry: %+v", entries[0])
	}
	if !entries[0].ResetGiven {
		t.Errorf("expected reset time parsed")
	}
	wantReset, _ := time.Parse(time.RFC3339, "2026-09-24T16:01:16Z")
	if !entries[0].Reset.Equal(wantReset) {
		t.Errorf("unexpected reset: %v", entries[0].Reset)
	}
}

func TestParseUsageOutputGarbage(t *testing.T) {
	for _, bad := range []string{"", "random text", "a\tb\tc", "x\ty\tnotanumber\t2026-01-01T00:00:00Z"} {
		if _, ok := ParseUsageOutput(bad); ok {
			t.Errorf("expected parse failure for %q", bad)
		}
	}
}

func TestFormatUsageCardID(t *testing.T) {
	entries, ok := ParseUsageOutput(sampleUsage)
	if !ok {
		t.Fatal("parse failed")
	}
	// Fixed "now": 2026-09-18T10:05:00Z == 17:05 WIB.
	now, _ := time.Parse(time.RFC3339, "2026-09-18T10:05:00Z")
	card := FormatUsageCard("id", entries, now)

	for _, want := range []string{
		"Kuota Model Antigravity",
		"Gemini Models",
		"Claude & GPT",
		"Limit 5 Jam",
		"Limit Mingguan",
		"79%",
		"100%",
		"Reset dalam",
		"WIB",
		"█",
	} {
		if !strings.Contains(card, want) {
			t.Errorf("card missing %q:\n%s", want, card)
		}
	}

	// Five-hour limit (reset 07:01:16Z, now 10:05Z) already passed -> "segera".
	if !strings.Contains(card, "segera") {
		t.Errorf("expected 'segera' for elapsed reset:\n%s", card)
	}
	// Weekly Gemini reset 24 Sep 16:01 UTC = 23:01 WIB, ~6d 5h away.
	if !strings.Contains(card, "6 hari 5 jam") {
		t.Errorf("expected '6 hari 5 jam' countdown:\n%s", card)
	}
	if !strings.Contains(card, "24 Sep 2026, 23:01 WIB") {
		t.Errorf("expected WIB reset time:\n%s", card)
	}
	// Five-hour row should come before weekly row within a scope.
	fiveIdx := strings.Index(card, "Limit 5 Jam")
	weekIdx := strings.Index(card, "Limit Mingguan")
	if fiveIdx < 0 || weekIdx < 0 || fiveIdx > weekIdx {
		t.Errorf("expected 5-hour row before weekly row:\n%s", card)
	}
}

func TestFormatUsageCardEN(t *testing.T) {
	entries, ok := ParseUsageOutput(sampleUsage)
	if !ok {
		t.Fatal("parse failed")
	}
	now, _ := time.Parse(time.RFC3339, "2026-09-18T10:05:00Z")
	card := FormatUsageCard("en", entries, now)
	for _, want := range []string{"Model Quota", "5-Hour Limit", "Weekly Limit", "Resets in", "79%", "WIB"} {
		if !strings.Contains(card, want) {
			t.Errorf("en card missing %q:\n%s", want, card)
		}
	}
}

func TestUsageBar(t *testing.T) {
	if got := usageBar(100); got != "██████████" {
		t.Errorf("100%% bar = %q", got)
	}
	if got := usageBar(0); got != "░░░░░░░░░░" {
		t.Errorf("0%% bar = %q", got)
	}
	if got := usageBar(79); len([]rune(got)) != 10 {
		t.Errorf("bar must be 10 cells, got %q", got)
	}
}

func TestFormatCountdown(t *testing.T) {
	cases := []struct {
		d    time.Duration
		lang string
		want string
	}{
		{6*24*time.Hour + 6*time.Hour, "id", "6 hari 6 jam"},
		{3*time.Hour + 55*time.Minute, "id", "3 jam 55 mnt"},
		{12*time.Minute + 30*time.Second, "id", "12 mnt 30 dtk"},
		{45 * time.Second, "id", "45 dtk"},
		{-time.Minute, "id", "segera"},
		{6*24*time.Hour + 6*time.Hour, "en", "6 days 6 hours"},
		{1*time.Hour + 1*time.Minute, "en", "1 hour 1 min"},
		{-time.Minute, "en", "any moment"},
	}
	for _, c := range cases {
		if got := formatCountdown(c.lang, c.d); !strings.Contains(got, c.want) {
			t.Errorf("formatCountdown(%s, %v) = %q; want containing %q", c.lang, c.d, got, c.want)
		}
	}
}

func TestFormatUsageCardWithAccount(t *testing.T) {
	entries, ok := ParseUsageOutput(sampleUsage)
	if !ok {
		t.Fatal("parse failed")
	}
	now, _ := time.Parse(time.RFC3339, "2026-09-18T10:05:00Z")

	acc := &auth.AccountInfo{
		Email: "dev@example.com",
		Tier:  "Pro",
	}

	// Indonesian
	cardID := FormatUsageCard("id", entries, now, acc)
	if !strings.Contains(cardID, "dev@example.com") {
		t.Errorf("cardID missing email:\n%s", cardID)
	}
	if !strings.Contains(cardID, "<b>Langganan:</b> Pro") {
		t.Errorf("cardID missing Langganan: Pro:\n%s", cardID)
	}
	if !strings.Contains(cardID, "⭐") {
		t.Errorf("cardID missing star icon for Pro:\n%s", cardID)
	}

	// English with Ultra tier
	accUltra := &auth.AccountInfo{
		Email: "ultra@example.com",
		Tier:  "Ultra",
	}
	cardEN := FormatUsageCard("en", entries, now, accUltra)
	if !strings.Contains(cardEN, "ultra@example.com") {
		t.Errorf("cardEN missing email:\n%s", cardEN)
	}
	if !strings.Contains(cardEN, "<b>Plan:</b> Ultra") {
		t.Errorf("cardEN missing Plan: Ultra:\n%s", cardEN)
	}
	if !strings.Contains(cardEN, "💎") {
		t.Errorf("cardEN missing diamond icon for Ultra:\n%s", cardEN)
	}
}
