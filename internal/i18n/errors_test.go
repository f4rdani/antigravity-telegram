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
	if pe.IsServiceUnavailable {
		t.Errorf("expected IsServiceUnavailable = false for 429 quota error")
	}

	card := FormatErrorCard("id", pe)
	if !strings.Contains(card, "Batas Kuota") {
		t.Errorf("card missing quota title: %s", card)
	}
}

func TestParseError503Unavailable(t *testing.T) {
	// 503 UNAVAILABLE should NOT be classified as quota limit
	raw := "API error (attempt 1): UNAVAILABLE (code 503): The service is currently unavailable."
	pe := ParseError(raw)

	if pe.IsQuotaLimit {
		t.Errorf("expected IsQuotaLimit = false for 503 error, got true — 503 is NOT a quota issue")
	}
	if !pe.IsServiceUnavailable {
		t.Errorf("expected IsServiceUnavailable = true for 503 error")
	}

	cardID := FormatErrorCard("id", pe)
	if strings.Contains(cardID, "Batas Kuota") {
		t.Errorf("503 error card should NOT contain 'Batas Kuota': %s", cardID)
	}
	if !strings.Contains(cardID, "Sementara") {
		t.Errorf("503 error card should contain temporary unavailable message: %s", cardID)
	}

	cardEN := FormatErrorCard("en", pe)
	if !strings.Contains(cardEN, "Temporarily Unavailable") {
		t.Errorf("503 English card should contain 'Temporarily Unavailable': %s", cardEN)
	}
}
