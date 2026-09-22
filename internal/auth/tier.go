package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type cachedTier struct {
	tier      string
	fetchedAt time.Time
}

var (
	tierCacheMu sync.RWMutex
	tierCache   = make(map[string]cachedTier)
	tierTTL     = 10 * time.Minute
)

// InvalidateTierCache purges the cached tier for a given email (or all if email == "").
func InvalidateTierCache(email string) {
	tierCacheMu.Lock()
	defer tierCacheMu.Unlock()
	if email == "" {
		tierCache = make(map[string]cachedTier)
	} else {
		delete(tierCache, strings.ToLower(strings.TrimSpace(email)))
	}
}

// NormalizeTierName standardizes raw tier identifiers or names into clean labels (Free, Plus, Pro, Ultra).
func NormalizeTierName(tierID, tierName string) string {
	lowerID := strings.ToLower(strings.TrimSpace(tierID))
	lowerName := strings.ToLower(strings.TrimSpace(tierName))

	switch {
	case strings.Contains(lowerID, "ultra") || strings.Contains(lowerName, "ultra"):
		return "Ultra"
	case strings.Contains(lowerID, "pro") || strings.Contains(lowerName, "pro"):
		return "Pro"
	case strings.Contains(lowerID, "plus") || strings.Contains(lowerName, "plus"):
		return "Plus"
	case lowerID == "free-tier" || strings.Contains(lowerID, "free") || lowerName == "free":
		return "Free"
	case strings.Contains(lowerID, "standard") || strings.Contains(lowerName, "standard"):
		return "Standard"
	case strings.Contains(lowerID, "enterprise") || strings.Contains(lowerName, "enterprise"):
		return "Enterprise"
	default:
		if tierName != "" && !strings.EqualFold(tierName, "antigravity") {
			return tierName
		}
		return "Free"
	}
}

// TierIcon returns an appropriate emoji icon for a given tier name.
func TierIcon(tier string) string {
	switch strings.ToLower(strings.TrimSpace(tier)) {
	case "ultra":
		return "💎"
	case "pro":
		return "⭐"
	case "plus":
		return "🌟"
	case "standard", "enterprise":
		return "🏢"
	case "free":
		return "🆓"
	default:
		return "⭐"
	}
}

// FormatTierBadge returns a formatted badge string with icon and tier name (e.g. "⭐ Pro", "🆓 Free").
func FormatTierBadge(tier string) string {
	t := strings.TrimSpace(tier)
	if t == "" {
		t = "Free"
	}
	return TierIcon(t) + " " + t
}

type codeAssistTierResp struct {
	CurrentTier *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"currentTier"`
	PaidTier *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"paidTier"`
}

// FetchTier calls the Google Cloud Code Assist loadCodeAssist API to identify the subscription tier.
func FetchTier(ctx context.Context, accessToken string) (string, error) {
	if accessToken == "" {
		return "Free", nil
	}

	endpoints := []string{
		"https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist",
		"https://daily-cloudcode-pa.googleapis.com/v1internal:loadCodeAssist",
	}

	client := &http.Client{Timeout: 3 * time.Second}
	var lastErr error

	for _, ep := range endpoints {
		req, err := http.NewRequestWithContext(ctx, "POST", ep, bytes.NewBufferString("{}"))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "antigravity-cli")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode == http.StatusOK {
			var caResp codeAssistTierResp
			err := json.NewDecoder(resp.Body).Decode(&caResp)
			_ = resp.Body.Close()
			if err == nil {
				// Google One AI subscriptions (Pro, Ultra, Plus) are returned in paidTier
				if caResp.PaidTier != nil && caResp.PaidTier.ID != "" {
					return NormalizeTierName(caResp.PaidTier.ID, caResp.PaidTier.Name), nil
				}
				if caResp.CurrentTier != nil && caResp.CurrentTier.ID != "" {
					return NormalizeTierName(caResp.CurrentTier.ID, caResp.CurrentTier.Name), nil
				}
			}
			return "Free", nil
		}
		_ = resp.Body.Close()
		lastErr = fmt.Errorf("loadCodeAssist API status %d", resp.StatusCode)
	}

	return "Free", lastErr
}

// GetActiveTier returns the subscription tier for the active account, using cache if fresh.
func GetActiveTier(email string) string {
	lowerEmail := strings.ToLower(strings.TrimSpace(email))
	if lowerEmail != "" {
		tierCacheMu.RLock()
		if c, ok := tierCache[lowerEmail]; ok && time.Since(c.fetchedAt) < tierTTL {
			tierCacheMu.RUnlock()
			return c.tier
		}
		tierCacheMu.RUnlock()
	}

	tf, err := GetActiveTokenFile()
	if err != nil || tf == nil || tf.Token.AccessToken == "" {
		return "Free"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	tier, err := FetchTier(ctx, tf.Token.AccessToken)
	if err != nil || tier == "" {
		tier = "Free"
	}

	if lowerEmail != "" {
		tierCacheMu.Lock()
		tierCache[lowerEmail] = cachedTier{
			tier:      tier,
			fetchedAt: time.Now(),
		}
		tierCacheMu.Unlock()
	}

	return tier
}
