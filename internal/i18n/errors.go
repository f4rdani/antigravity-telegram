package i18n

import (
	"fmt"
	"regexp"
	"strings"

	"agy-tele/internal/renderer"
)

var (
	reURLPattern = regexp.MustCompile(`https?://[^\s<>"'\)\]]+`)
)

// ParsedError holds classified error information and extracted links
type ParsedError struct {
	Raw                  string
	Title                string
	CleanMessage         string
	ExtractedURLs        []string
	IsEligibility        bool
	IsQuotaLimit         bool
	IsAuth               bool
	IsServiceUnavailable bool
}

// ExtractURLs returns all valid URLs found in text, stripping trailing punctuation
func ExtractURLs(text string) []string {
	matches := reURLPattern.FindAllString(text, -1)
	var urls []string
	seen := make(map[string]bool)

	for _, m := range matches {
		cleaned := m
		for len(cleaned) > 0 {
			last := cleaned[len(cleaned)-1]
			if last == '.' || last == ',' || last == ';' || last == ':' || last == '!' || last == '?' {
				cleaned = cleaned[:len(cleaned)-1]
			} else {
				break
			}
		}
		if len(cleaned) > 0 && !seen[cleaned] {
			seen[cleaned] = true
			urls = append(urls, cleaned)
		}
	}
	return urls
}

// ParseError analyzes error strings from agy or internal components
func ParseError(raw string) *ParsedError {
	trimmed := strings.TrimSpace(raw)
	lower := strings.ToLower(trimmed)

	pe := &ParsedError{
		Raw:           trimmed,
		ExtractedURLs: ExtractURLs(trimmed),
	}

	// Detect Eligibility Check Failed
	if strings.Contains(lower, "eligibility") || strings.Contains(lower, "eligible") {
		pe.IsEligibility = true
		pe.Title = "Eligibility Check Failed"
	} else if strings.Contains(lower, "quota") || strings.Contains(lower, "resource_exhausted") ||
		strings.Contains(lower, "usage limit") || strings.Contains(lower, "rate limit") ||
		strings.Contains(lower, "code 429") {
		// True quota/rate-limit errors: quota is actually exhausted
		pe.IsQuotaLimit = true
		pe.Title = "Usage Limit / Quota Exceeded"
	} else if strings.Contains(lower, "unavailable") || strings.Contains(lower, "code 503") ||
		strings.Contains(lower, "capacity available") ||
		strings.Contains(lower, "service is currently unavailable") {
		// 503 UNAVAILABLE = server temporarily down, NOT a quota issue.
		// Session should NOT be reset; user can retry shortly.
		pe.IsServiceUnavailable = true
		pe.Title = "Service Temporarily Unavailable"
	} else if strings.Contains(lower, "invalid_grant") || strings.Contains(lower, "unauthenticated") ||
		strings.Contains(lower, "authentication required") {
		pe.IsAuth = true
		pe.Title = "Authentication Required"
	} else {
		pe.Title = "Error"
	}

	pe.CleanMessage = trimmed
	return pe
}

// FormatErrorCard formats a structured alert message for Telegram HTML
func FormatErrorCard(lang string, pe *ParsedError) string {
	l := NormalizeLang(lang)
	var sb strings.Builder

	if pe.IsEligibility {
		if l == "en" {
			sb.WriteString("⚠️ <b>Eligibility Check Failed:</b>\n")
			sb.WriteString("Your Google account is currently not eligible for Antigravity or requires additional program verification.\n\n")
			if len(pe.ExtractedURLs) > 0 {
				sb.WriteString("👉 <b>Verification Link:</b>\n")
				for _, u := range pe.ExtractedURLs {
					sb.WriteString(fmt.Sprintf("• <a href=\"%s\">%s</a>\n", u, u))
				}
				sb.WriteString("\n")
			}
			sb.WriteString("💡 <i>Click the verification button below to authorize, or switch to another Google account via /accounts.</i>")
		} else {
			sb.WriteString("⚠️ <b>Eligibility Check Failed:</b>\n")
			sb.WriteString("Akun Google Anda saat ini belum memenuhi syarat akses Antigravity atau memerlukan verifikasi program preview.\n\n")
			if len(pe.ExtractedURLs) > 0 {
				sb.WriteString("👉 <b>Link Verifikasi:</b>\n")
				for _, u := range pe.ExtractedURLs {
					sb.WriteString(fmt.Sprintf("• <a href=\"%s\">%s</a>\n", u, u))
				}
				sb.WriteString("\n")
			}
			sb.WriteString("💡 <i>Silakan klik tombol verifikasi di bawah untuk melengkapi izin akun, atau beralih ke akun Google lain via /accounts.</i>")
		}
		return sb.String()
	}

	if pe.IsQuotaLimit {
		if l == "en" {
			sb.WriteString("⚠️ <b>Usage Limit / Quota Reached:</b>\n")
			sb.WriteString(fmt.Sprintf("<code>%s</code>\n\n", renderer.EscapeHTML(pe.CleanMessage)))
			sb.WriteString("🔄 <i>Conversation session has been reset so subsequent messages start fresh.</i>\n")
			sb.WriteString("💡 <i>You can switch to another Google account with /accounts or check your quota with /quota.</i>")
		} else {
			sb.WriteString("⚠️ <b>Batas Kuota / Usage Limit Tercapai:</b>\n")
			sb.WriteString(fmt.Sprintf("<code>%s</code>\n\n", renderer.EscapeHTML(pe.CleanMessage)))
			sb.WriteString("🔄 <i>Sesi percakapan telah direset otomatis agar pesan selanjutnya tidak terkendala sesi lama.</i>\n")
			sb.WriteString("💡 <i>Anda dapat beralih ke akun Google lain via /accounts atau cek sisa kuota via /quota.</i>")
		}
		return sb.String()
	}

	if pe.IsServiceUnavailable {
		if l == "en" {
			sb.WriteString("⏳ <b>Service Temporarily Unavailable:</b>\n")
			sb.WriteString(fmt.Sprintf("<code>%s</code>\n\n", renderer.EscapeHTML(pe.CleanMessage)))
			sb.WriteString("🔄 <i>The Google API server is temporarily unavailable (503). Your session is still intact — please try again in a moment.</i>\n")
			sb.WriteString("💡 <i>If the issue persists, you can switch accounts via /accounts.</i>")
		} else {
			sb.WriteString("⏳ <b>Server Sedang Tidak Tersedia (Sementara):</b>\n")
			sb.WriteString(fmt.Sprintf("<code>%s</code>\n\n", renderer.EscapeHTML(pe.CleanMessage)))
			sb.WriteString("🔄 <i>Server Google API sedang sibuk / sementara tidak tersedia (503). Sesi Anda tetap aman — silakan coba kirim pesan lagi sebentar lagi.</i>\n")
			sb.WriteString("💡 <i>Jika masih bermasalah, coba beralih akun via /accounts.</i>")
		}
		return sb.String()
	}

	// General Error
	escaped := renderer.EscapeHTML(pe.CleanMessage)
	for _, u := range pe.ExtractedURLs {
		escaped = strings.ReplaceAll(escaped, u, fmt.Sprintf("<a href=\"%s\">%s</a>", u, u))
	}

	if l == "en" {
		sb.WriteString("❌ <b>An error occurred:</b>\n")
		sb.WriteString(escaped)
	} else {
		sb.WriteString("❌ <b>Terjadi kesalahan:</b>\n")
		sb.WriteString(escaped)
	}

	return sb.String()
}
