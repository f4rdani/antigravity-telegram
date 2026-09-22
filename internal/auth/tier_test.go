package auth

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNormalizeTierName(t *testing.T) {
	cases := []struct {
		id   string
		name string
		want string
	}{
		{"free-tier", "Antigravity", "Free"},
		{"free-tier", "Free Tier", "Free"},
		{"g1-pro-tier", "Google AI Pro", "Pro"},
		{"g1-ultra-tier", "Google AI Ultra", "Ultra"},
		{"plus", "Plus Tier", "Plus"},
		{"standard-tier", "Standard", "Standard"},
		{"enterprise-tier", "Enterprise", "Enterprise"},
		{"unknown-id", "Custom Pro Plan", "Pro"},
		{"unknown-id", "Ultra Fast", "Ultra"},
		{"unknown-id", "Plus Membership", "Plus"},
		{"unknown-id", "Something Else", "Something Else"},
		{"", "", "Free"},
	}

	for _, c := range cases {
		got := NormalizeTierName(c.id, c.name)
		if got != c.want {
			t.Errorf("NormalizeTierName(%q, %q) = %q, want %q", c.id, c.name, got, c.want)
		}
	}
}

func TestFormatTierBadge(t *testing.T) {
	cases := []struct {
		tier string
		want string
	}{
		{"Pro", "⭐ Pro"},
		{"pro", "⭐ pro"},
		{"Ultra", "💎 Ultra"},
		{"Plus", "🌟 Plus"},
		{"Free", "🆓 Free"},
		{"free", "🆓 free"},
		{"Standard", "🏢 Standard"},
		{"Enterprise", "🏢 Enterprise"},
		{"", "🆓 Free"},
	}

	for _, c := range cases {
		got := FormatTierBadge(c.tier)
		if got != c.want {
			t.Errorf("FormatTierBadge(%q) = %q, want %q", c.tier, got, c.want)
		}
	}
}

func TestTierCacheAndInvalidation(t *testing.T) {
	email := "testuser@example.com"
	tierCacheMu.Lock()
	tierCache[email] = cachedTier{tier: "Pro", fetchedAt: time.Now()}
	tierCacheMu.Unlock()

	tier := GetActiveTier(email)
	if tier != "Pro" {
		t.Errorf("expected cached tier Pro, got %s", tier)
	}

	InvalidateTierCache(email)
	tierCacheMu.RLock()
	_, exists := tierCache[email]
	tierCacheMu.RUnlock()
	if exists {
		t.Error("expected cache to be invalidated")
	}
}

func TestCodeAssistPaidTierPriority(t *testing.T) {
	// Simulate user with free currentTier but paidTier g1-pro-tier (Google One AI Pro subscription)
	rawJSON := `{
		"currentTier": {"id": "free-tier", "name": "Antigravity"},
		"paidTier": {"id": "g1-pro-tier", "name": "Google AI Pro"}
	}`

	var resp codeAssistTierResp
	importJSON := []byte(rawJSON)
	if err := json.Unmarshal(importJSON, &resp); err != nil {
		t.Fatal(err)
	}

	var tier string
	if resp.PaidTier != nil && resp.PaidTier.ID != "" {
		tier = NormalizeTierName(resp.PaidTier.ID, resp.PaidTier.Name)
	} else if resp.CurrentTier != nil && resp.CurrentTier.ID != "" {
		tier = NormalizeTierName(resp.CurrentTier.ID, resp.CurrentTier.Name)
	}

	if tier != "Pro" {
		t.Errorf("expected Pro, got %s", tier)
	}
}
