package i18n

import (
	"strings"
	"testing"
)

func TestExtractURLs(t *testing.T) {
	text := `An error occurred: Eligibility Check Failed. Please visit https://cloud.google.com/earlyaccess/antigravity and verify. Also see https://goo.gle/preview-join, thanks!`
	urls := ExtractURLs(text)
	if len(urls) != 2 {
		t.Fatalf("expected 2 URLs, got %d: %#v", len(urls), urls)
	}

	if urls[0] != "https://cloud.google.com/earlyaccess/antigravity" {
		t.Errorf("expected clean URL without trailing dot, got %q", urls[0])
	}
	if urls[1] != "https://goo.gle/preview-join" {
		t.Errorf("expected clean URL without trailing comma, got %q", urls[1])
	}
}

func TestParseErrorEligibility(t *testing.T) {
	raw := "API error (attempt 1): UNAVAILABLE: Eligibility Check Failed. Please enroll at https://goo.gle/antigravity-preview"
	pe := ParseError(raw)

	if !pe.IsEligibility {
		t.Errorf("expected IsEligibility = true")
	}
	if len(pe.ExtractedURLs) != 1 || pe.ExtractedURLs[0] != "https://goo.gle/antigravity-preview" {
		t.Errorf("unexpected extracted URLs: %#v", pe.ExtractedURLs)
	}

	cardID := FormatErrorCard("id", pe)
	if !strings.Contains(cardID, "Eligibility Check Failed") || !strings.Contains(cardID, "href=\"https://goo.gle/antigravity-preview\"") {
		t.Errorf("formatted card missing required HTML elements: %s", cardID)
	}

	cardEN := FormatErrorCard("en", pe)
	if !strings.Contains(cardEN, "Eligibility Check Failed") || !strings.Contains(cardEN, "not eligible") {
		t.Errorf("formatted English card missing required content: %s", cardEN)
	}
}

func TestParseErrorQuota(t *testing.T) {
	raw := "RESOURCE_EXHAUSTED (code 429): Quota limit reached for model gemini-3.8-flash-high"
	pe := ParseError(raw)

	if !pe.IsQuotaLimit {
		t.Errorf("expected IsQuotaLimit = true")
	}

	card := FormatErrorCard("id", pe)
	if !strings.Contains(card, "Batas Kuota") {
		t.Errorf("card missing quota title: %s", card)
	}
}
