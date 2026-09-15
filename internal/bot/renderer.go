package bot

import (
	"fmt"
	"strings"

	"agy-tele/internal/engine"
	"agy-tele/internal/session"
)

func FormatHelp() string {
	var sb strings.Builder
	sb.WriteString("🤖 <b>Antigravity CLI Remote Bridge (agy-tele)</b>\n\n")
	sb.WriteString("<b>⚡ Agent Execution (Mode B):</b>\n")
	sb.WriteString("• Kirim teks biasa untuk prompt / coding agent\n")
	sb.WriteString("• <code>/plan &lt;task&gt;</code> — Jalankan perencanaan terstruktur\n")
	sb.WriteString("• <code>/goal &lt;task&gt;</code> — Jalankan task autonomous jangka panjang\n")
	sb.WriteString("• <code>/continue</code> — Lanjutkan sesi obrolan terakhir\n")
	sb.WriteString("• <code>/cancel</code> atau <code>/stop</code> — Hentikan proses yang sedang berjalan\n\n")

	sb.WriteString("<b>📊 CLI Status & Kuota (Mode A):</b>\n")
	sb.WriteString("• <code>/usage</code> atau <code>/quota</code> — Cek limit 5 jam & mingguan\n")
	sb.WriteString("• <code>/credits</code> — Cek sisa credit G1\n")
	sb.WriteString("• <code>/model [nama]</code> — Lihat atau ganti model aktif\n")
	sb.WriteString("• <code>/effort [level]</code> — Set reasoning effort (low/medium/high)\n")
	sb.WriteString("• <code>/skills</code> — Daftar skills yang terpasang\n")
	sb.WriteString("• <code>/agents</code> — Daftar subagents yang tersedia\n\n")

	sb.WriteString("<b>📂 Workspace & Session:</b>\n")
	sb.WriteString("• <code>/cwd [path]</code> — Tampilkan / ganti direktori kerja\n")
	sb.WriteString("• <code>/ls [path]</code> — Tampilkan daftar file/folder di server\n")
	sb.WriteString("• <code>/pwd</code> — Tampilkan path direktori aktif saat ini\n")
	sb.WriteString("• <code>/new [path]</code> — Mulai percakapan sesi baru\n")
	sb.WriteString("• <code>/sessions</code> — Daftar riwayat sesi percakapan\n")
	sb.WriteString("• <code>/switch &lt;id&gt;</code> — Ganti ke ID percakapan tertentu\n")
	sb.WriteString("• <code>/permission [auto|ask]</code> — Atur mode persetujuan tools\n")
	sb.WriteString("• <code>/status</code> — Informasi status runtime daemon\n")
	sb.WriteString("• <code>/file &lt;rel_path&gt;</code> — Unduh file dari server ke Telegram\n")

	return sb.String()
}

func FormatStatus(sess *session.UserSession, isRunning bool) string {
	stateStr := "🟢 IDLE (Siap)"
	if isRunning {
		stateStr = "🔄 SEDANG BERJALAN..."
	}

	permStr := "🟢 Auto-Approve (Hands-free)"
	if sess.PermissionMode == "ask" {
		permStr = "🟡 Ask User (Konfirmasi Telegram)"
	}

	convID := sess.ActiveConversationID
	if convID == "" {
		convID = "(Belum ada sesi aktif / Fresh)"
	}

	modelStr := sess.ActiveModel
	if modelStr == "" {
		modelStr = "(Default agy)"
	}

	effortStr := sess.ActiveEffort
	if effortStr == "" {
		effortStr = "(Default agy)"
	}

	return fmt.Sprintf(
		"⚙️ <b>Daemon Status</b>\n\n"+
			"• <b>Status</b>: %s\n"+
			"• <b>Workspace (CWD)</b>: <code>%s</code>\n"+
			"• <b>Active Conversation</b>: <code>%s</code>\n"+
			"• <b>Permission Mode</b>: %s\n"+
			"• <b>Active Model</b>: <code>%s</code>\n"+
			"• <b>Reasoning Effort</b>: <code>%s</code>\n"+
			"• <b>Terakhir Aktif</b>: %s",
		stateStr,
		sess.CWD,
		convID,
		permStr,
		modelStr,
		effortStr,
		sess.LastActiveTime.Format("15:04:05 02 Jan 2006"),
	)
}

func FormatSessions(recent []session.ConversationEntry, activeID string) string {
	if len(recent) == 0 {
		return "Belum ada riwayat sesi percakapan. Gunakan <code>/new</code> atau langsung kirim pesan untuk memulai."
	}

	var sb strings.Builder
	sb.WriteString("📋 <b>Riwayat Sesi Percakapan:</b>\n\n")

	for i, c := range recent {
		tag := ""
		if c.ID == activeID {
			tag = " 🌟 [AKTIF]"
		}
		sb.WriteString(fmt.Sprintf("%d. <code>%s</code>%s\n", i+1, c.ID, tag))
		sb.WriteString(fmt.Sprintf("   📁 CWD: <code>%s</code>\n", c.CWD))
		sb.WriteString(fmt.Sprintf("   🕒 Waktu: %s\n", c.CreatedAt.Format("15:04 02/01/06")))
		sb.WriteString(fmt.Sprintf("   👉 Switch: <code>/switch %s</code>\n\n", c.ID))
	}

	return sb.String()
}

func FormatCodeBlock(title string, content string) string {
	return fmt.Sprintf("<b>%s</b>\n<pre>%s</pre>", title, EscapeHTML(content))
}

func FormatResultFooter(res *engine.ResultPayload) string {
	if res == nil {
		return ""
	}

	var parts []string
	if res.DurationSeconds > 0 {
		parts = append(parts, fmt.Sprintf("⏱ %.1fs", res.DurationSeconds))
	}
	if res.Usage != nil && res.Usage.TotalTokens > 0 {
		parts = append(parts, fmt.Sprintf("🪙 %d tokens", res.Usage.TotalTokens))
	}
	if res.NumTurns > 0 {
		parts = append(parts, fmt.Sprintf("🔄 Turn %d", res.NumTurns))
	}
	if res.ConversationID != "" {
		parts = append(parts, fmt.Sprintf("🆔 %s", res.ConversationID[:8]))
	}

	if len(parts) == 0 {
		return ""
	}
	return "<i>(" + strings.Join(parts, " • ") + ")</i>"
}

func EscapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
