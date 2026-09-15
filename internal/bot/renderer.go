package bot

import (
	"fmt"
	"strings"

	"agy-tele/internal/engine"
	"agy-tele/internal/renderer"
	"agy-tele/internal/session"
)

const AppVersion = "v1.0.5"

func FormatHelp() string {
	var sb strings.Builder
	sb.WriteString("🤖 <b>Antigravity CLI Remote Bridge (" + AppVersion + ")</b>\n\n")
	sb.WriteString("<b>⚡ Agent Execution (Mode B):</b>\n")
	sb.WriteString("• Kirim teks biasa untuk prompt / coding agent\n")
	sb.WriteString("• <code>/resume</code> — Pilih dan lanjutkan sesi percakapan sebelumnya\n")
	sb.WriteString("• <code>/plan &lt;task&gt;</code> — Jalankan perencanaan terstruktur\n")
	sb.WriteString("• <code>/goal &lt;task&gt;</code> — Jalankan task autonomous jangka panjang\n")
	sb.WriteString("• <code>/continue</code> — Lanjutkan sesi obrolan terakhir langsung\n")
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

func FormatSessions(convs []session.AvailableConversation, activeID string) string {
	if len(convs) == 0 {
		return "📂 Belum ada riwayat sesi percakapan. Mulai percakapan baru dengan mengirim pesan langsung."
	}

	var sb strings.Builder
	sb.WriteString("📋 <b>Daftar Riwayat Percakapan:</b>\n\n")

	for i, c := range convs {
		tag := ""
		if c.ID == activeID {
			tag = " 🌟 <i>(Aktif)</i>"
		}
		timeInfo := ""
		if c.TimeLabel != "" {
			timeInfo = fmt.Sprintf(" • <i>%s</i>", c.TimeLabel)
		}
		shortID := c.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		sb.WriteString(fmt.Sprintf("<b>%d. %s</b>%s%s\n", i+1, EscapeHTML(c.Title), timeInfo, tag))
		if c.Workspace != "" {
			sb.WriteString(fmt.Sprintf("   📁 <code>%s</code>\n", c.Workspace))
		}
		sb.WriteString(fmt.Sprintf("   👉 Resume: <code>/resume %s</code>\n\n", shortID))
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

	parts = append(parts, fmt.Sprintf("🏷️ %s", AppVersion))
	return "<i>(" + strings.Join(parts, " • ") + ")</i>"
}

func EscapeHTML(s string) string {
	return renderer.EscapeHTML(s)
}

func FormatMarkdownForTelegram(s string) string {
	return renderer.FormatMarkdownForTelegram(s)
}

func StripHTML(s string) string {
	return renderer.StripHTML(s)
}



func DescribeStepAction(step *engine.StepUpdatePayload) string {
	if step == nil {
		return "Sedang memproses..."
	}

	if step.StepType == "agent_response" {
		if step.State == "ACTIVE" && step.TextDelta == "" {
			return "💭 <i>Menganalisis instruksi & berpikir...</i>"
		}
		return "✍️ <i>Menulis balasan...</i>"
	}

	if step.StepType == "tool" {
		toolName := step.ToolName
		if toolName == "" && step.ToolInfo != nil {
			toolName = step.ToolInfo.Name
		}

		params := make(map[string]interface{})
		if step.ToolInfo != nil && step.ToolInfo.Parameters != nil {
			params = step.ToolInfo.Parameters
		}

		switch toolName {
		case "run_command":
			cmd := ""
			if v, ok := params["CommandLine"].(string); ok && v != "" {
				cmd = v
			}
			if len(cmd) > 80 {
				cmd = cmd[:77] + "..."
			}
			if cmd != "" {
				return fmt.Sprintf("⚡ <b>Menjalankan:</b> <code>%s</code>", EscapeHTML(cmd))
			}
			return "⚡ Menjalankan perintah terminal..."

		case "view_file":
			path := ""
			if v, ok := params["AbsolutePath"].(string); ok && v != "" {
				path = v
			}
			if path != "" {
				return fmt.Sprintf("📖 <b>Membaca file:</b> <code>%s</code>", EscapeHTML(path))
			}
			return "📖 Membaca file..."

		case "write_to_file":
			path := ""
			if v, ok := params["TargetFile"].(string); ok && v != "" {
				path = v
			}
			if path != "" {
				return fmt.Sprintf("📝 <b>Menulis file:</b> <code>%s</code>", EscapeHTML(path))
			}
			return "📝 Menulis file baru..."

		case "replace_file_content":
			path := ""
			if v, ok := params["TargetFile"].(string); ok && v != "" {
				path = v
			}
			if path != "" {
				return fmt.Sprintf("✏️ <b>Mengedit file:</b> <code>%s</code>", EscapeHTML(path))
			}
			return "✏️ Mengedit file..."

		case "grep_search":
			q := ""
			if v, ok := params["Query"].(string); ok && v != "" {
				q = v
			}
			if len(q) > 60 {
				q = q[:57] + "..."
			}
			if q != "" {
				return fmt.Sprintf("🔍 <b>Mencari:</b> <code>%s</code>", EscapeHTML(q))
			}
			return "🔍 Mencari teks di kode..."

		case "find_by_name":
			pat := ""
			if v, ok := params["Pattern"].(string); ok && v != "" {
				pat = v
			}
			if pat != "" {
				return fmt.Sprintf("📂 <b>Mencari file:</b> <code>%s</code>", EscapeHTML(pat))
			}
			return "📂 Mencari file..."

		case "list_dir":
			dir := ""
			if v, ok := params["DirectoryPath"].(string); ok && v != "" {
				dir = v
			}
			if dir != "" {
				return fmt.Sprintf("📁 <b>Melihat folder:</b> <code>%s</code>", EscapeHTML(dir))
			}
			return "📁 Melihat folder..."

		case "search_web":
			q := ""
			if v, ok := params["query"].(string); ok && v != "" {
				q = v
			}
			if q != "" {
				return fmt.Sprintf("🌐 <b>Mencari web:</b> <i>%s</i>", EscapeHTML(q))
			}
			return "🌐 Mencari web..."

		case "read_url_content":
			u := ""
			if v, ok := params["Url"].(string); ok && v != "" {
				u = v
			}
			if len(u) > 60 {
				u = u[:57] + "..."
			}
			if u != "" {
				return fmt.Sprintf("🌐 <b>Mengunduh web:</b> <code>%s</code>", EscapeHTML(u))
			}
			return "🌐 Membaca halaman web..."

		case "manage_task":
			action := ""
			if v, ok := params["Action"].(string); ok && v != "" {
				action = v
			}
			if action != "" {
				return fmt.Sprintf("📋 <b>Task manager:</b> %s", EscapeHTML(action))
			}
			return "📋 Mengelola task latar belakang..."

		case "invoke_subagent":
			return "🤖 <b>Memanggil subagent...</b>"

		default:
			if v, ok := params["toolAction"].(string); ok && v != "" {
				return fmt.Sprintf("🛠️ <b>%s</b>", EscapeHTML(v))
			}
			if v, ok := params["toolSummary"].(string); ok && v != "" {
				return fmt.Sprintf("🛠️ <b>%s</b>", EscapeHTML(v))
			}
			if toolName != "" {
				return fmt.Sprintf("🛠️ <b>Tool:</b> <code>%s</code>", EscapeHTML(toolName))
			}
			return "⚙️ Menjalankan langkah otomatis..."
		}
	}

	return "⏳ Sedang memproses..."
}

func FormatProgressStatus(currentAction string, recentHistory []string) string {
	var sb strings.Builder
	sb.WriteString("⏳ <b>Antigravity sedang bekerja...</b>\n\n")

	if len(recentHistory) > 0 {
		sb.WriteString("<i>Aktivitas:</i>\n")
		for _, h := range recentHistory {
			sb.WriteString(fmt.Sprintf("• %s\n", h))
		}
		sb.WriteString("\n")
	}

	if currentAction != "" {
		sb.WriteString(fmt.Sprintf("🔄 <b>Saat ini:</b>\n%s\n\n", currentAction))
	}

	sb.WriteString("<i>(Hasil akhir akan tampil otomatis setelah selesai)</i>")
	return sb.String()
}

func FormatActivityBadge(step *engine.StepUpdatePayload) string {
	if step == nil {
		return "💭 <i>Sedang berpikir...</i>"
	}

	if step.StepType == "agent_response" {
		if step.State == "ACTIVE" && step.TextDelta == "" {
			return "💭 <i>Menganalisis instruksi & berpikir...</i>"
		}
		return "✍️ <i>Menulis balasan...</i>"
	}

	if step.StepType == "tool" {
		toolName := step.ToolName
		if toolName == "" && step.ToolInfo != nil {
			toolName = step.ToolInfo.Name
		}

		params := make(map[string]interface{})
		if step.ToolInfo != nil && step.ToolInfo.Parameters != nil {
			params = step.ToolInfo.Parameters
		}

		switch toolName {
		case "run_command":
			cmd := ""
			if v, ok := params["CommandLine"].(string); ok && v != "" {
				cmd = v
			}
			if len(cmd) > 100 {
				cmd = cmd[:97] + "..."
			}
			if cmd != "" {
				return fmt.Sprintf("⚡ <code>Bash(%s)</code>", EscapeHTML(cmd))
			}
			return "⚡ <code>Bash(...)</code>"

		case "view_file":
			path := ""
			if v, ok := params["AbsolutePath"].(string); ok && v != "" {
				path = v
			}
			if path != "" {
				return fmt.Sprintf("📖 <code>View(%s)</code>", EscapeHTML(path))
			}
			return "📖 <code>View(...)</code>"

		case "write_to_file":
			path := ""
			if v, ok := params["TargetFile"].(string); ok && v != "" {
				path = v
			}
			if path != "" {
				return fmt.Sprintf("📝 <code>Write(%s)</code>", EscapeHTML(path))
			}
			return "📝 <code>Write(...)</code>"

		case "replace_file_content":
			path := ""
			if v, ok := params["TargetFile"].(string); ok && v != "" {
				path = v
			}
			if path != "" {
				return fmt.Sprintf("✏️ <code>Edit(%s)</code>", EscapeHTML(path))
			}
			return "✏️ <code>Edit(...)</code>"

		case "grep_search":
			q := ""
			if v, ok := params["Query"].(string); ok && v != "" {
				q = v
			}
			if len(q) > 80 {
				q = q[:77] + "..."
			}
			if q != "" {
				return fmt.Sprintf("🔍 <code>Search(%s)</code>", EscapeHTML(q))
			}
			return "🔍 <code>Search(...)</code>"

		case "find_by_name":
			pat := ""
			if v, ok := params["Pattern"].(string); ok && v != "" {
				pat = v
			}
			if pat != "" {
				return fmt.Sprintf("📂 <code>Find(%s)</code>", EscapeHTML(pat))
			}
			return "📂 <code>Find(...)</code>"

		default:
			if v, ok := params["toolAction"].(string); ok && v != "" {
				return fmt.Sprintf("🛠️ <code>%s</code>", EscapeHTML(v))
			}
			if toolName != "" {
				return fmt.Sprintf("🛠️ <code>Tool(%s)</code>", EscapeHTML(toolName))
			}
			return "⚙️ <i>Sedang bekerja...</i>"
		}
	}

	return "⏳ <i>Sedang memproses...</i>"
}
