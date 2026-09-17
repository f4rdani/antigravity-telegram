package i18n

import (
	"fmt"
	"strings"

	"agy-tele/internal/artifact"
	"agy-tele/internal/engine"
	"agy-tele/internal/renderer"
	"agy-tele/internal/session"
)

// NormalizeLang ensures language is either "id" or "en", defaulting to "id"
func NormalizeLang(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "en", "english":
		return "en"
	default:
		return "id"
	}
}

// T returns a translated string by key with optional formatting arguments
func T(lang, key string, args ...interface{}) string {
	l := NormalizeLang(lang)
	dict, ok := translations[l]
	if !ok {
		dict = translations["id"]
	}

	msg, ok := dict[key]
	if !ok {
		// Fallback to Indonesian if key not found
		msg = translations["id"][key]
		if msg == "" {
			msg = key
		}
	}

	if len(args) > 0 {
		return fmt.Sprintf(msg, args...)
	}
	return msg
}

var translations = map[string]map[string]string{
	"id": {
		"err_occurred":               "Terjadi kesalahan:",
		"err_busy_wait":              "Sedang ada proses atau tugas yang berjalan. Silakan tunggu hingga selesai atau gunakan /cancel untuk menghentikannya.",
		"task_cancelled":             "Tugas aktif berhasil dihentikan.",
		"no_active_task":             "Tidak ada tugas yang sedang berjalan.",
		"new_session_started":        "Sesi percakapan baru dimulai.",
		"status_refreshed":           "🔄 Status diperbarui!",
		"cwd_changed":                "Direktori kerja diubah ke: %s",
		"autodelete_set":             "✅ Auto-delete diatur ke <b>%d turns</b>.",
		"autodelete_disabled":        "🚫 Auto-delete percakapan Telegram <b>dinonaktifkan</b>.",
		"autodelete_cleared":         "🧹 Berhasil membersihkan <b>%d</b> pesan riwayat Telegram.",
		"autodelete_already_empty":   "ℹ️ Tidak ada riwayat pesan lama yang tersimpan untuk dibersihkan.",
		"lang_changed":               "🌐 Bahasa antarmuka diubah ke: <b>Bahasa Indonesia</b> 🇮🇩",
		"perm_changed":               "Mode permission diubah ke: <b>%s</b>",
		"model_changed":              "Model aktif diubah ke: <code>%s</code>",
		"effort_changed":             "Reasoning effort diubah ke: <b>%s</b>",
		"no_conversation_history":    "📂 Belum ada riwayat sesi percakapan. Mulai percakapan baru dengan mengirim pesan langsung.",
		"no_artifacts_found":         "📑 <b>Tidak ada artifact ditemukan.</b>\nArtifact biasanya berupa rencana kerja (/plan), laporan PRD, atau dokumen terstruktur yang dibuat oleh Antigravity.",
		"btn_resume":                 "📂 Resume Sesi",
		"btn_artifacts":              "📑 Artifacts & Plans",
		"btn_quota":                  "📊 Quota/Usage",
		"btn_credits":                "💰 Credits",
		"btn_model":                  "🧠 Ganti Model",
		"btn_effort":                 "⚡ Set Effort",
		"btn_skills":                 "🧰 Daftar Skills",
		"btn_status":                 "⚙️ Status & CWD",
		"btn_permission":             "🔒 Atur Permission",
		"btn_autodelete":             "🧹 Auto-Delete",
		"btn_language":               "🌐 Bahasa / Lang",
		"btn_new_session":            "🔄 Sesi Baru",
		"btn_close_menu":             "🗑️ Tutup Menu",
		"btn_close":                  "🗑️ Tutup",
		"btn_back_menu":              "« Menu",
		"btn_back":                   "« Kembali",
		"btn_refresh":                "🔄 Refresh",
		"btn_list_files":             "📁 List File (/ls)",
		"btn_clean_now":              "🧹 Bersihkan Chat Sekarang",
		"btn_cancel_task":            "🛑 Hentikan Tugas Aktif",
		"btn_open_file":              "📖 Buka Isi File",
		"btn_download_file":          "📥 Unduh Dokumen",
		"btn_approve":                "✅ Approve & Eksekusi",
		"btn_reject":                 "❌ Reject / Revisi",
	},
	"en": {
		"err_occurred":               "An error occurred:",
		"err_busy_wait":              "A task or process is currently running. Please wait for it to complete or use /cancel to stop it.",
		"task_cancelled":             "Active task was successfully cancelled.",
		"no_active_task":             "No task is currently running.",
		"new_session_started":        "New conversation session started.",
		"status_refreshed":           "🔄 Status refreshed!",
		"cwd_changed":                "Working directory changed to: %s",
		"autodelete_set":             "✅ Auto-delete limit set to <b>%d turns</b>.",
		"autodelete_disabled":        "🚫 Telegram chat auto-delete is now <b>disabled</b>.",
		"autodelete_cleared":         "🧹 Successfully cleaned <b>%d</b> Telegram history messages.",
		"autodelete_already_empty":   "ℹ️ No tracked old messages found to clean.",
		"lang_changed":               "🌐 Interface language changed to: <b>English</b> 🇬🇧",
		"perm_changed":               "Permission mode changed to: <b>%s</b>",
		"model_changed":              "Active model changed to: <code>%s</code>",
		"effort_changed":             "Reasoning effort changed to: <b>%s</b>",
		"no_conversation_history":    "📂 No conversation history found. Start a new conversation by sending a message.",
		"no_artifacts_found":         "📑 <b>No artifacts found.</b>\nArtifacts are structured documents, plans (/plan), or PRD reports created by Antigravity.",
		"btn_resume":                 "📂 Resume Session",
		"btn_artifacts":              "📑 Artifacts & Plans",
		"btn_quota":                  "📊 Quota/Usage",
		"btn_credits":                "💰 Credits",
		"btn_model":                  "🧠 Switch Model",
		"btn_effort":                 "⚡ Set Effort",
		"btn_skills":                 "🧰 List Skills",
		"btn_status":                 "⚙️ Status & CWD",
		"btn_permission":             "🔒 Permission Mode",
		"btn_autodelete":             "🧹 Auto-Delete",
		"btn_language":               "🌐 Language / Bahasa",
		"btn_new_session":            "🔄 New Session",
		"btn_close_menu":             "🗑️ Close Menu",
		"btn_close":                  "🗑️ Close",
		"btn_back_menu":              "« Main Menu",
		"btn_back":                   "« Back",
		"btn_refresh":                "🔄 Refresh",
		"btn_list_files":             "📁 List Files (/ls)",
		"btn_clean_now":              "🧹 Clean Chat History Now",
		"btn_cancel_task":            "🛑 Stop Active Task",
		"btn_open_file":              "📖 View Content",
		"btn_download_file":          "📥 Download Doc",
		"btn_approve":                "✅ Approve & Execute",
		"btn_reject":                 "❌ Reject / Revise",
	},
}

// GetHelpText returns localized help documentation
func GetHelpText(lang, version string) string {
	l := NormalizeLang(lang)
	var sb strings.Builder

	if l == "en" {
		sb.WriteString("🤖 <b>Antigravity CLI Remote Bridge (" + version + ")</b>\n\n")
		sb.WriteString("<b>⚡ Agent Execution (Mode B):</b>\n")
		sb.WriteString("• Send plain text for prompt / coding agent\n")
		sb.WriteString("• Send photos, documents, videos, or audio for automatic multimodal analysis\n")
		sb.WriteString("• <code>/artifact</code> — Manage docs/plans: open, download, approve, or request revision\n")
		sb.WriteString("• <code>/resume</code> — Select and continue a previous conversation session\n")
		sb.WriteString("• <code>/plan &lt;task&gt;</code> — Run structured task planning\n")
		sb.WriteString("• <code>/goal &lt;task&gt;</code> — Run long-running autonomous task\n")
		sb.WriteString("• <code>/continue</code> — Continue the last conversation session directly\n")
		sb.WriteString("• <code>/cancel</code> or <code>/stop</code> — Stop the currently running process\n\n")

		sb.WriteString("<b>📊 CLI Status & Quota (Mode A):</b>\n")
		sb.WriteString("• <code>/usage</code> or <code>/quota</code> — Check 5-hour & weekly rate limits\n")
		sb.WriteString("• <code>/credits</code> — Check remaining G1 credits\n")
		sb.WriteString("• <code>/model [name]</code> — View or switch active model\n")
		sb.WriteString("• <code>/effort [level]</code> — Set reasoning effort (low/medium/high)\n")
		sb.WriteString("• <code>/skills</code> — List installed skills\n")
		sb.WriteString("• <code>/agents</code> — List available subagents\n\n")

		sb.WriteString("<b>📂 Workspace & Session:</b>\n")
		sb.WriteString("• <code>/cwd [path]</code> — Show / change working directory\n")
		sb.WriteString("• <code>/ls [path]</code> — List files and folders on server\n")
		sb.WriteString("• <code>/pwd</code> — Show current active directory path\n")
		sb.WriteString("• <code>/new [path]</code> — Start a new conversation session\n")
		sb.WriteString("• <code>/sessions</code> — View conversation session history\n")
		sb.WriteString("• <code>/switch &lt;id&gt;</code> — Switch to a specific conversation ID\n")
		sb.WriteString("• <code>/permission [auto|ask]</code> — Set tool approval mode\n")
		sb.WriteString("• <code>/status</code> — Show daemon runtime status & workspace\n")
		sb.WriteString("• <code>/autodelete [limit]</code> — Configure Telegram auto-delete limit (e.g. 50, 0/off)\n")
		sb.WriteString("• <code>/lang [id|en]</code> — Change bot language\n")
		sb.WriteString("• <code>/file &lt;rel_path&gt;</code> — Download file from server to Telegram\n")
	} else {
		sb.WriteString("🤖 <b>Antigravity CLI Remote Bridge (" + version + ")</b>\n\n")
		sb.WriteString("<b>⚡ Agent Execution (Mode B):</b>\n")
		sb.WriteString("• Kirim teks biasa untuk prompt / coding agent\n")
		sb.WriteString("• Kirim foto, dokumen, video, atau audio untuk dianalisis otomatis\n")
		sb.WriteString("• <code>/artifact</code> — Kelola dokumen/plan: buka, unduh, approve, atau minta revisi\n")
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
		sb.WriteString("• <code>/status</code> — Informasi status runtime daemon & workspace\n")
		sb.WriteString("• <code>/autodelete [limit]</code> — Atur batas auto-delete pesan Telegram (misal 50, 0/off)\n")
		sb.WriteString("• <code>/lang [id|en]</code> — Ganti bahasa tampilan bot\n")
		sb.WriteString("• <code>/file &lt;rel_path&gt;</code> — Unduh file dari server ke Telegram\n")
	}

	return sb.String()
}

// GetStatusText formats daemon runtime status card
func GetStatusText(lang string, sess *session.UserSession, isRunning bool, activeTaskDesc string) string {
	l := NormalizeLang(lang)

	var stateStr string
	var permStr string
	var langDisplay string
	var autoDeleteStr string

	if l == "en" {
		stateStr = "🟢 IDLE (Ready)"
		if isRunning {
			stateStr = "🔄 PROCESSING TASK..."
			if activeTaskDesc != "" {
				stateStr += fmt.Sprintf("\n  └ <i>\"%s\"</i>", renderer.EscapeHTML(activeTaskDesc))
			}
		}

		permStr = "🟢 Auto-Approve (Hands-free)"
		if sess.PermissionMode == "ask" {
			permStr = "🟡 Ask User (Telegram confirmation)"
		}

		langDisplay = "🇬🇧 English"
		if sess.MaxTelegramTurns <= 0 {
			autoDeleteStr = "Disabled"
		} else {
			autoDeleteStr = fmt.Sprintf("%d turns (currently tracked: %d)", sess.MaxTelegramTurns, len(sess.TrackedTurns))
		}

		convID := sess.ActiveConversationID
		if convID == "" {
			convID = "(No active session / Fresh)"
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
				"• <b>Language</b>: %s\n"+
				"• <b>Auto-Delete</b>: %s\n"+
				"• <b>Last Active</b>: %s",
			stateStr,
			sess.CWD,
			convID,
			permStr,
			modelStr,
			effortStr,
			langDisplay,
			autoDeleteStr,
			sess.LastActiveTime.Format("15:04:05 02 Jan 2006"),
		)
	}

	stateStr = "🟢 IDLE (Siap)"
	if isRunning {
		stateStr = "🔄 SEDANG MEMPROSES TUGAS..."
		if activeTaskDesc != "" {
			stateStr += fmt.Sprintf("\n  └ <i>\"%s\"</i>", renderer.EscapeHTML(activeTaskDesc))
		}
	}

	permStr = "🟢 Auto-Approve (Hands-free)"
	if sess.PermissionMode == "ask" {
		permStr = "🟡 Ask User (Konfirmasi Telegram)"
	}

	langDisplay = "🇮🇩 Bahasa Indonesia"
	if sess.MaxTelegramTurns <= 0 {
		autoDeleteStr = "Nonaktif (Disabled)"
	} else {
		autoDeleteStr = fmt.Sprintf("%d turn (terlacak: %d)", sess.MaxTelegramTurns, len(sess.TrackedTurns))
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
		"⚙️ <b>Status Runtime Daemon</b>\n\n"+
			"• <b>Status</b>: %s\n"+
			"• <b>Workspace (CWD)</b>: <code>%s</code>\n"+
			"• <b>Active Conversation</b>: <code>%s</code>\n"+
			"• <b>Permission Mode</b>: %s\n"+
			"• <b>Active Model</b>: <code>%s</code>\n"+
			"• <b>Reasoning Effort</b>: <code>%s</code>\n"+
			"• <b>Bahasa / Language</b>: %s\n"+
			"• <b>Auto-Delete Telegram</b>: %s\n"+
			"• <b>Terakhir Aktif</b>: %s",
		stateStr,
		sess.CWD,
		convID,
		permStr,
		modelStr,
		effortStr,
		langDisplay,
		autoDeleteStr,
		sess.LastActiveTime.Format("15:04:05 02 Jan 2006"),
	)
}

// GetAutoDeleteText returns explanation card for auto-delete settings
func GetAutoDeleteText(lang string, currentLimit int, trackedCount int) string {
	l := NormalizeLang(lang)

	limitStr := fmt.Sprintf("%d turns", currentLimit)
	if currentLimit <= 0 {
		if l == "en" {
			limitStr = "Disabled (0)"
		} else {
			limitStr = "Nonaktif (0)"
		}
	}

	if l == "en" {
		return fmt.Sprintf(
			"🧹 <b>Telegram Chat Auto-Delete Settings</b>\n\n"+
				"This feature automatically cleans older conversation messages in Telegram when the message count exceeds the limit. It keeps your Telegram app smooth and prevents mobile lag.\n\n"+
				"• <b>Current Limit:</b> <code>%s</code>\n"+
				"• <b>Currently Tracked:</b> <code>%d turns</code>\n\n"+
				"<i>Note: This ONLY deletes messages inside Telegram chat. Session history, code artifacts, and transcripts on the Antigravity server remain completely safe and untouched.</i>",
			limitStr,
			trackedCount,
		)
	}

	return fmt.Sprintf(
		"🧹 <b>Pengaturan Auto-Delete Percakapan Telegram</b>\n\n"+
			"Fitur ini secara otomatis menghapus pesan riwayat percakapan lama di Telegram jika sudah melampaui batas turn, sehingga aplikasi Telegram di ponsel Anda tetap ringan dan tidak lag.\n\n"+
			"• <b>Batas Aktif:</b> <code>%s</code>\n"+
			"• <b>Pesan Terlacak Saat Ini:</b> <code>%d turn</code>\n\n"+
			"<i>Catatan: Fitur ini HANYA menghapus tampilan pesan di chat Telegram. Riwayat sesi, file transcript, dan kode di server Antigravity tetap tersimpan aman dan tidak terhapus.</i>",
		limitStr,
		trackedCount,
	)
}

// GetLanguageMenuText returns the language selection card
func GetLanguageMenuText(currentLang string) string {
	l := NormalizeLang(currentLang)
	if l == "en" {
		return "🌐 <b>Language Settings / Pengaturan Bahasa</b>\n\n" +
			"Select your preferred interface language for Antigravity Telegram Bot:\n" +
			"Current: <b>🇬🇧 English</b>\n\n" +
			"<i>All menus, status cards, tool actions, and notifications will adapt to your choice.</i>"
	}

	return "🌐 <b>Pengaturan Bahasa / Language Settings</b>\n\n" +
		"Pilih bahasa antarmuka untuk bot Telegram Antigravity:\n" +
		"Saat ini: <b>🇮🇩 Bahasa Indonesia</b>\n\n" +
		"<i>Semua menu, kartu status, keterangan proses tool, dan notifikasi akan menyesuaikan pilihan Anda.</i>"
}

// DescribeStepAction formats the step progress label
func DescribeStepAction(lang string, step *engine.StepUpdatePayload) string {
	l := NormalizeLang(lang)
	if step == nil {
		if l == "en" {
			return "Processing..."
		}
		return "Sedang memproses..."
	}

	if step.StepType == "agent_response" {
		if step.State == "ACTIVE" && step.TextDelta == "" {
			if l == "en" {
				return "💭 <i>Analyzing instructions & thinking...</i>"
			}
			return "💭 <i>Menganalisis instruksi & berpikir...</i>"
		}
		if l == "en" {
			return "✍️ <i>Writing response...</i>"
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
				if l == "en" {
					return fmt.Sprintf("⚡ <b>Running:</b> <code>%s</code>", renderer.EscapeHTML(cmd))
				}
				return fmt.Sprintf("⚡ <b>Menjalankan:</b> <code>%s</code>", renderer.EscapeHTML(cmd))
			}
			if l == "en" {
				return "⚡ Running shell command..."
			}
			return "⚡ Menjalankan perintah terminal..."

		case "view_file":
			path := ""
			if v, ok := params["AbsolutePath"].(string); ok && v != "" {
				path = v
			}
			if path != "" {
				if l == "en" {
					return fmt.Sprintf("📖 <b>Reading:</b> <code>%s</code>", renderer.EscapeHTML(path))
				}
				return fmt.Sprintf("📖 <b>Membaca file:</b> <code>%s</code>", renderer.EscapeHTML(path))
			}
			if l == "en" {
				return "📖 Reading file..."
			}
			return "📖 Membaca file..."

		case "write_to_file":
			path := ""
			if v, ok := params["TargetFile"].(string); ok && v != "" {
				path = v
			}
			if path != "" {
				if l == "en" {
					return fmt.Sprintf("📝 <b>Writing:</b> <code>%s</code>", renderer.EscapeHTML(path))
				}
				return fmt.Sprintf("📝 <b>Menulis file:</b> <code>%s</code>", renderer.EscapeHTML(path))
			}
			if l == "en" {
				return "📝 Writing new file..."
			}
			return "📝 Menulis file baru..."

		case "replace_file_content":
			path := ""
			if v, ok := params["TargetFile"].(string); ok && v != "" {
				path = v
			}
			if path != "" {
				if l == "en" {
					return fmt.Sprintf("✏️ <b>Editing:</b> <code>%s</code>", renderer.EscapeHTML(path))
				}
				return fmt.Sprintf("✏️ <b>Mengedit file:</b> <code>%s</code>", renderer.EscapeHTML(path))
			}
			if l == "en" {
				return "✏️ Editing file..."
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
				if l == "en" {
					return fmt.Sprintf("🔍 <b>Searching:</b> <code>%s</code>", renderer.EscapeHTML(q))
				}
				return fmt.Sprintf("🔍 <b>Mencari:</b> <code>%s</code>", renderer.EscapeHTML(q))
			}
			if l == "en" {
				return "🔍 Searching code..."
			}
			return "🔍 Mencari teks di kode..."

		case "find_by_name":
			pat := ""
			if v, ok := params["Pattern"].(string); ok && v != "" {
				pat = v
			}
			if pat != "" {
				if l == "en" {
					return fmt.Sprintf("📂 <b>Finding:</b> <code>%s</code>", renderer.EscapeHTML(pat))
				}
				return fmt.Sprintf("📂 <b>Mencari file:</b> <code>%s</code>", renderer.EscapeHTML(pat))
			}
			if l == "en" {
				return "📂 Finding files..."
			}
			return "📂 Mencari file..."

		case "list_dir":
			dir := ""
			if v, ok := params["DirectoryPath"].(string); ok && v != "" {
				dir = v
			}
			if dir != "" {
				if l == "en" {
					return fmt.Sprintf("📁 <b>Viewing directory:</b> <code>%s</code>", renderer.EscapeHTML(dir))
				}
				return fmt.Sprintf("📁 <b>Melihat folder:</b> <code>%s</code>", renderer.EscapeHTML(dir))
			}
			if l == "en" {
				return "📁 Viewing directory..."
			}
			return "📁 Melihat folder..."

		case "search_web":
			q := ""
			if v, ok := params["query"].(string); ok && v != "" {
				q = v
			}
			if q != "" {
				if l == "en" {
					return fmt.Sprintf("🌐 <b>Searching web:</b> <i>%s</i>", renderer.EscapeHTML(q))
				}
				return fmt.Sprintf("🌐 <b>Mencari web:</b> <i>%s</i>", renderer.EscapeHTML(q))
			}
			if l == "en" {
				return "🌐 Searching the web..."
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
				if l == "en" {
					return fmt.Sprintf("🌐 <b>Fetching web:</b> <code>%s</code>", renderer.EscapeHTML(u))
				}
				return fmt.Sprintf("🌐 <b>Mengunduh web:</b> <code>%s</code>", renderer.EscapeHTML(u))
			}
			if l == "en" {
				return "🌐 Reading web page..."
			}
			return "🌐 Membaca halaman web..."

		case "manage_task":
			action := ""
			if v, ok := params["Action"].(string); ok && v != "" {
				action = v
			}
			if action != "" {
				if l == "en" {
					return fmt.Sprintf("📋 <b>Task manager:</b> %s", renderer.EscapeHTML(action))
				}
				return fmt.Sprintf("📋 <b>Task manager:</b> %s", renderer.EscapeHTML(action))
			}
			if l == "en" {
				return "📋 Managing background tasks..."
			}
			return "📋 Mengelola task latar belakang..."

		case "invoke_subagent":
			if l == "en" {
				return "🤖 <b>Invoking subagent...</b>"
			}
			return "🤖 <b>Memanggil subagent...</b>"

		case "schedule":
			dur := ""
			if v, ok := params["DurationSeconds"].(string); ok && v != "" {
				dur = fmt.Sprintf(" (%ss)", v)
			} else if v, ok := params["DurationSeconds"].(float64); ok && v > 0 {
				dur = fmt.Sprintf(" (%.0fs)", v)
			}
			if l == "en" {
				return fmt.Sprintf("⏱️ <b>Waiting for background/timer%s...</b>", dur)
			}
			return fmt.Sprintf("⏱️ <b>Menunggu latar belakang/timer%s...</b>", dur)

		default:
			if v, ok := params["toolAction"].(string); ok && v != "" {
				return fmt.Sprintf("🛠️ <b>%s</b>", renderer.EscapeHTML(v))
			}
			if v, ok := params["toolSummary"].(string); ok && v != "" {
				return fmt.Sprintf("🛠️ <b>%s</b>", renderer.EscapeHTML(v))
			}
			if toolName != "" {
				return fmt.Sprintf("🛠️ <b>Tool:</b> <code>%s</code>", renderer.EscapeHTML(toolName))
			}
			if l == "en" {
				return "⚙️ Executing automated step..."
			}
			return "⚙️ Menjalankan langkah otomatis..."
		}
	}

	if l == "en" {
		return "⏳ Processing..."
	}
	return "⏳ Sedang memproses..."
}

// FormatProgressStatus formats status message when stream is in progress
func FormatProgressStatus(lang string, currentAction string, recentHistory []string) string {
	l := NormalizeLang(lang)
	var sb strings.Builder

	if l == "en" {
		sb.WriteString("⏳ <b>Antigravity is working...</b>\n\n")
		if len(recentHistory) > 0 {
			sb.WriteString("<i>Activity:</i>\n")
			for _, h := range recentHistory {
				sb.WriteString(fmt.Sprintf("• %s\n", h))
			}
			sb.WriteString("\n")
		}
		if currentAction != "" {
			sb.WriteString(fmt.Sprintf("🔄 <b>Currently:</b>\n%s\n\n", currentAction))
		}
		sb.WriteString("<i>(Final response will update automatically)</i>")
	} else {
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
	}

	return sb.String()
}

// FormatActivityBadge formats inline streaming badge
func FormatActivityBadge(lang string, step *engine.StepUpdatePayload) string {
	l := NormalizeLang(lang)
	if step == nil {
		if l == "en" {
			return "💭 <i>Thinking...</i>"
		}
		return "💭 <i>Sedang berpikir...</i>"
	}

	if step.StepType == "agent_response" {
		if step.State == "ACTIVE" && step.TextDelta == "" {
			if l == "en" {
				return "💭 <i>Analyzing instructions & thinking...</i>"
			}
			return "💭 <i>Menganalisis instruksi & berpikir...</i>"
		}
		if l == "en" {
			return "✍️ <i>Writing response...</i>"
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
				return fmt.Sprintf("⚡ <code>Bash(%s)</code>", renderer.EscapeHTML(cmd))
			}
			return "⚡ <code>Bash(...)</code>"

		case "view_file":
			path := ""
			if v, ok := params["AbsolutePath"].(string); ok && v != "" {
				path = v
			}
			if path != "" {
				return fmt.Sprintf("📖 <code>View(%s)</code>", renderer.EscapeHTML(path))
			}
			return "📖 <code>View(...)</code>"

		case "write_to_file":
			path := ""
			if v, ok := params["TargetFile"].(string); ok && v != "" {
				path = v
			}
			if path != "" {
				return fmt.Sprintf("📝 <code>Write(%s)</code>", renderer.EscapeHTML(path))
			}
			return "📝 <code>Write(...)</code>"

		case "replace_file_content":
			path := ""
			if v, ok := params["TargetFile"].(string); ok && v != "" {
				path = v
			}
			if path != "" {
				return fmt.Sprintf("✏️ <code>Edit(%s)</code>", renderer.EscapeHTML(path))
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
				return fmt.Sprintf("🔍 <code>Search(%s)</code>", renderer.EscapeHTML(q))
			}
			return "🔍 <code>Search(...)</code>"

		case "find_by_name":
			pat := ""
			if v, ok := params["Pattern"].(string); ok && v != "" {
				pat = v
			}
			if pat != "" {
				return fmt.Sprintf("📂 <code>Find(%s)</code>", renderer.EscapeHTML(pat))
			}
			return "📂 <code>Find(...)</code>"

		default:
			if v, ok := params["toolAction"].(string); ok && v != "" {
				return fmt.Sprintf("🛠️ <code>%s</code>", renderer.EscapeHTML(v))
			}
			if toolName != "" {
				return fmt.Sprintf("🛠️ <code>Tool(%s)</code>", renderer.EscapeHTML(toolName))
			}
			if l == "en" {
				return "⚙️ <i>Working...</i>"
			}
			return "⚙️ <i>Sedang bekerja...</i>"
		}
	}

	if l == "en" {
		return "⏳ <i>Processing...</i>"
	}
	return "⏳ <i>Sedang memproses...</i>"
}

// GetSessionsText formats session list
func GetSessionsText(lang string, convs []session.AvailableConversation, activeID string) string {
	l := NormalizeLang(lang)
	if len(convs) == 0 {
		return T(l, "no_conversation_history")
	}

	var sb strings.Builder
	if l == "en" {
		sb.WriteString("📋 <b>Conversation History:</b>\n\n")
	} else {
		sb.WriteString("📋 <b>Daftar Riwayat Percakapan:</b>\n\n")
	}

	for i, c := range convs {
		tag := ""
		if c.ID == activeID {
			if l == "en" {
				tag = " 🌟 <i>(Active)</i>"
			} else {
				tag = " 🌟 <i>(Aktif)</i>"
			}
		}
		timeInfo := ""
		if c.TimeLabel != "" {
			timeInfo = fmt.Sprintf(" • <i>%s</i>", c.TimeLabel)
		}
		shortID := c.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		sb.WriteString(fmt.Sprintf("<b>%d. %s</b>%s%s\n", i+1, renderer.EscapeHTML(c.Title), timeInfo, tag))
		if c.Workspace != "" {
			sb.WriteString(fmt.Sprintf("   📁 <code>%s</code>\n", c.Workspace))
		}
		sb.WriteString(fmt.Sprintf("   👉 Resume: <code>/resume %s</code>\n\n", shortID))
	}

	return sb.String()
}

// GetArtifactsText formats artifact list
func GetArtifactsText(lang string, items []artifact.Item) string {
	l := NormalizeLang(lang)
	if len(items) == 0 {
		return T(l, "no_artifacts_found")
	}

	var sb strings.Builder
	if l == "en" {
		sb.WriteString("📑 <b>Artifacts & Plans List:</b>\n\n")
	} else {
		sb.WriteString("📑 <b>Daftar Artifact & Plans:</b>\n\n")
	}

	for i, it := range items {
		status := "📄"
		if it.RequestFeedback {
			if l == "en" {
				status = "🔔 [Review Required]"
			} else {
				status = "🔔 [Menunggu Review]"
			}
		}
		timeInfo := ""
		if it.TimeLabel != "" {
			timeInfo = fmt.Sprintf(" • <i>%s</i>", it.TimeLabel)
		}
		sizeKB := float64(it.SizeBytes) / 1024.0

		sb.WriteString(fmt.Sprintf("<b>%d. %s</b> %s%s\n", i+1, renderer.EscapeHTML(it.ID), status, timeInfo))
		if it.Summary != "" {
			sb.WriteString(fmt.Sprintf("   📝 <i>%s</i>\n", renderer.EscapeHTML(it.Summary)))
		}
		if l == "en" {
			sb.WriteString(fmt.Sprintf("   📦 Size: <code>%.1f KB</code>\n\n", sizeKB))
		} else {
			sb.WriteString(fmt.Sprintf("   📦 Ukuran: <code>%.1f KB</code>\n\n", sizeKB))
		}
	}

	if l == "en" {
		sb.WriteString("<i>Click any artifact below to open details or give approval.</i>")
	} else {
		sb.WriteString("<i>Silakan klik salah satu artifact di bawah untuk membuka detail atau memberikan approval.</i>")
	}
	return sb.String()
}

// GetArtifactDetailText formats single artifact detail card
func GetArtifactDetailText(lang string, it artifact.Item) string {
	l := NormalizeLang(lang)
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("📑 <b>Artifact:</b> <code>%s</code>\n\n", renderer.EscapeHTML(it.FileName)))

	if l == "en" {
		if it.RequestFeedback {
			sb.WriteString("🔔 <b>Status:</b> 🟡 <i>Waiting for Review & Approval (Feedback Required)</i>\n")
		} else {
			sb.WriteString("📄 <b>Status:</b> 🟢 <i>Saved</i>\n")
		}

		if it.TimeLabel != "" {
			sb.WriteString(fmt.Sprintf("🕒 <b>Time:</b> %s\n", it.TimeLabel))
		}
		sb.WriteString(fmt.Sprintf("📦 <b>Size:</b> %.1f KB\n", float64(it.SizeBytes)/1024.0))
		sb.WriteString(fmt.Sprintf("📁 <b>Path:</b> <code>%s</code>\n\n", renderer.EscapeHTML(it.Path)))

		if it.Summary != "" {
			sb.WriteString(fmt.Sprintf("📝 <b>Summary:</b>\n%s\n\n", renderer.EscapeHTML(it.Summary)))
		}

		sb.WriteString("Choose an action below:\n")
		sb.WriteString("• <b>View Content</b>: Preview file markdown in chat\n")
		sb.WriteString("• <b>Download Doc</b>: Send raw file to Telegram\n")
		sb.WriteString("• <b>Approve</b>: Approve plan & proceed execution\n")
		sb.WriteString("• <b>Reject</b>: Request revisions on this artifact")
	} else {
		if it.RequestFeedback {
			sb.WriteString("🔔 <b>Status:</b> 🟡 <i>Menunggu Review & Persetujuan (Feedback Required)</i>\n")
		} else {
			sb.WriteString("📄 <b>Status:</b> 🟢 <i>Tersimpan</i>\n")
		}

		if it.TimeLabel != "" {
			sb.WriteString(fmt.Sprintf("🕒 <b>Waktu:</b> %s\n", it.TimeLabel))
		}
		sb.WriteString(fmt.Sprintf("📦 <b>Ukuran:</b> %.1f KB\n", float64(it.SizeBytes)/1024.0))
		sb.WriteString(fmt.Sprintf("📁 <b>Path:</b> <code>%s</code>\n\n", renderer.EscapeHTML(it.Path)))

		if it.Summary != "" {
			sb.WriteString(fmt.Sprintf("📝 <b>Ringkasan:</b>\n%s\n\n", renderer.EscapeHTML(it.Summary)))
		}

		sb.WriteString("Pilih tindakan di bawah:\n")
		sb.WriteString("• <b>Buka Isi File</b>: Tampilkan preview isi dokumen di chat\n")
		sb.WriteString("• <b>Unduh Dokumen</b>: Kirim file markdown asli ke Telegram\n")
		sb.WriteString("• <b>Approve</b>: Setujui rencana & instruksikan bot untuk lanjut eksekusi\n")
		sb.WriteString("• <b>Reject</b>: Minta bot untuk merevisi artifact/rencana ini")
	}

	return sb.String()
}
