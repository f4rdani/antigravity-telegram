package bot

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"agy-tele/internal/auth"
)

// wibZone is UTC+7 for user-facing reset times.
var wibZone = time.FixedZone("WIB", 7*3600)

var idMonths = []string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}
var enMonths = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

// UsageEntry is one parsed row of `agy -p "/usage"` output:
// "<scope>\t<limit name>\t<percent>%\t<RFC3339 reset UTC>".
type UsageEntry struct {
	Scope      string
	Kind       string
	Percent    int
	Reset      time.Time
	ResetGiven bool
}

// ParseUsageOutput parses raw /usage TSV output. Returns ok=false when the
// output doesn't match the expected shape so callers can fall back to raw text.
func ParseUsageOutput(out string) ([]UsageEntry, bool) {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	var entries []UsageEntry
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		parts := strings.Split(ln, "\t")
		if len(parts) != 4 {
			return nil, false
		}
		scope := strings.TrimSpace(parts[0])
		kind := strings.TrimSpace(parts[1])
		pctStr := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(parts[2]), "%"))
		pct, err := strconv.Atoi(pctStr)
		if err != nil || scope == "" || kind == "" {
			return nil, false
		}
		if pct < 0 {
			pct = 0
		}
		if pct > 100 {
			pct = 100
		}
		e := UsageEntry{Scope: scope, Kind: kind, Percent: pct}
		if ts, err := time.Parse(time.RFC3339, strings.TrimSpace(parts[3])); err == nil {
			e.Reset = ts
			e.ResetGiven = true
		}
		entries = append(entries, e)
	}
	if len(entries) == 0 {
		return nil, false
	}
	return entries, true
}

func usageScopeLabel(scope, lang string) string {
	lower := strings.ToLower(strings.TrimSpace(scope))
	switch lower {
	case "gemini models":
		return "♊ Gemini Models"
	case "claude and gpt models", "claude and gpt":
		return "🤖 Claude & GPT Models"
	default:
		return "📦 " + EscapeHTML(scope)
	}
}

func usageKindLabel(kind, lang string) string {
	lower := strings.ToLower(kind)
	isEN := lang == "en"
	switch {
	case strings.Contains(lower, "five hour"):
		if isEN {
			return "⏱️ 5-Hour Limit"
		}
		return "⏱️ Limit 5 Jam"
	case strings.Contains(lower, "weekly"):
		if isEN {
			return "📅 Weekly Limit"
		}
		return "📅 Limit Mingguan"
	default:
		return "📌 " + EscapeHTML(kind)
	}
}

func usageDot(pct int) string {
	switch {
	case pct >= 50:
		return "🟢"
	case pct >= 20:
		return "🟡"
	default:
		return "🔴"
	}
}

// usageBar renders a 10-cell progress bar, e.g. ████████░░.
func usageBar(pct int) string {
	filled := (pct + 5) / 10
	if filled < 0 {
		filled = 0
	}
	if filled > 10 {
		filled = 10
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", 10-filled)
}

func usageMonthName(t time.Time, lang string) string {
	m := int(t.Month()) - 1
	if m < 0 || m > 11 {
		return ""
	}
	if lang == "en" {
		return enMonths[m]
	}
	return idMonths[m]
}

// formatWIB renders "24 Sep 2026, 23:01 WIB".
func formatWIB(t time.Time, lang string) string {
	w := t.In(wibZone)
	return fmt.Sprintf("%d %s %d, %02d:%02d WIB", w.Day(), usageMonthName(w, lang), w.Year(), w.Hour(), w.Minute())
}

// formatCountdown renders "5 hari 22 jam" / "3 jam 55 mnt" / "12 mnt 30 dtk" / "45 dtk"
// (top two non-zero units), or a "refresh soon" note when already passed.
func formatCountdown(lang string, d time.Duration) string {
	isEN := lang == "en"
	if d <= 0 {
		if isEN {
			return "any moment now"
		}
		return "segera (menunggu refresh)"
	}
	totalSecs := int64(d.Seconds())
	days := totalSecs / 86400
	hours := (totalSecs % 86400) / 3600
	mins := (totalSecs % 3600) / 60
	secs := totalSecs % 60

	var uDay, uHour, uMin, uSec string
	if isEN {
		uDay, uHour, uMin, uSec = "days", "hours", "mins", "secs"
	} else {
		uDay, uHour, uMin, uSec = "hari", "jam", "mnt", "dtk"
	}
	one := func(n int64, u string) string {
		if isEN && n == 1 {
			u = strings.TrimSuffix(u, "s")
		}
		return fmt.Sprintf("%d %s", n, u)
	}

	var parts []string
	if days > 0 {
		parts = append(parts, one(days, uDay))
		parts = append(parts, one(hours, uHour))
	} else if hours > 0 {
		parts = append(parts, one(hours, uHour))
		parts = append(parts, one(mins, uMin))
	} else if mins > 0 {
		parts = append(parts, one(mins, uMin))
		parts = append(parts, one(secs, uSec))
	} else {
		parts = append(parts, one(secs, uSec))
	}
	// Top two units only.
	if len(parts) > 2 {
		parts = parts[:2]
	}
	return strings.Join(parts, " ")
}

// FormatUsageCard renders the beautified /usage card with per-limit remaining,
// progress bars, WIB reset times, reset countdowns, and active account/plan info.
func FormatUsageCard(lang string, entries []UsageEntry, now time.Time, acc ...*auth.AccountInfo) string {
	isEN := lang == "en"

	// Group by scope (first-seen order), five-hour row before weekly row.
	type group struct {
		scope string
		rows  []UsageEntry
	}
	var order []string
	groups := map[string]*group{}
	for _, e := range entries {
		g, ok := groups[e.Scope]
		if !ok {
			g = &group{scope: e.Scope}
			groups[e.Scope] = g
			order = append(order, e.Scope)
		}
		g.rows = append(g.rows, e)
	}
	fiveFirst := func(rows []UsageEntry) {
		for i := 0; i < len(rows); i++ {
			for j := i + 1; j < len(rows); j++ {
				iw := strings.Contains(strings.ToLower(rows[i].Kind), "weekly")
				jw := strings.Contains(strings.ToLower(rows[j].Kind), "weekly")
				if iw && !jw {
					rows[i], rows[j] = rows[j], rows[i]
				}
			}
		}
	}

	var sb strings.Builder
	if isEN {
		sb.WriteString("📊 <b>Antigravity Model Quota</b>\n")
		if len(acc) > 0 && acc[0] != nil && acc[0].Email != "" {
			sb.WriteString(fmt.Sprintf("👤 <b>Account:</b> <code>%s</code>\n", EscapeHTML(acc[0].Email)))
			tier := acc[0].Tier
			if tier == "" {
				tier = "Free"
			}
			sb.WriteString(fmt.Sprintf("%s <b>Plan:</b> %s\n", auth.TierIcon(tier), EscapeHTML(tier)))
		}
		sb.WriteString(fmt.Sprintf("🕒 <i>Updated %s</i>\n", formatWIB(now, lang)))
	} else {
		sb.WriteString("📊 <b>Kuota Model Antigravity</b>\n")
		if len(acc) > 0 && acc[0] != nil && acc[0].Email != "" {
			sb.WriteString(fmt.Sprintf("👤 <b>Akun:</b> <code>%s</code>\n", EscapeHTML(acc[0].Email)))
			tier := acc[0].Tier
			if tier == "" {
				tier = "Free"
			}
			sb.WriteString(fmt.Sprintf("%s <b>Langganan:</b> %s\n", auth.TierIcon(tier), EscapeHTML(tier)))
		}
		sb.WriteString(fmt.Sprintf("🕒 <i>Diperbarui %s</i>\n", formatWIB(now, lang)))
	}

	for _, key := range order {
		g := groups[key]
		fiveFirst(g.rows)
		sb.WriteString(fmt.Sprintf("\n<b>%s</b>\n", usageScopeLabel(g.scope, lang)))
		for _, r := range g.rows {
			remainWord := "tersisa"
			resetWord := "Reset dalam"
			atWord := "•"
			if isEN {
				remainWord = "left"
				resetWord = "Resets in"
			}
			sb.WriteString(fmt.Sprintf("• %s — %s <b>%d%%</b> %s\n", usageKindLabel(r.Kind, lang), usageDot(r.Percent), r.Percent, remainWord))
			sb.WriteString(fmt.Sprintf("  <code>%s</code>\n", usageBar(r.Percent)))
			if r.ResetGiven {
				sb.WriteString(fmt.Sprintf("  └ %s <b>%s</b> %s %s\n", resetWord, formatCountdown(lang, r.Reset.Sub(now)), atWord, formatWIB(r.Reset, lang)))
			}
		}
	}

	if isEN {
		sb.WriteString("\n<i>5-hour limits refill every 5 hours, weekly ones every 7 days. When one scope runs low, switch to the other.</i>")
	} else {
		sb.WriteString("\n<i>Limit 5-jam terisi ulang tiap 5 jam, mingguan tiap 7 hari. Kalau satu scope menipis, pindah ke scope lain.</i>")
	}
	return sb.String()
}
