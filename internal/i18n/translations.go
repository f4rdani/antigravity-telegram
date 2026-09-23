package i18n

import (
	"fmt"
	"strings"

	"agy-tele/internal/artifact"
	"agy-tele/internal/auth"
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
		"err_occurred":             "Terjadi kesalahan:",
		"err_busy_wait":            "Sedang ada proses atau tugas yang berjalan. Silakan tunggu hingga selesai atau gunakan /cancel untuk menghentikannya.",
		"task_cancelled":           "Tugas aktif berhasil dihentikan.",
		"no_active_task":           "Tidak ada tugas yang sedang berjalan.",
		"new_session_started":      "Sesi percakapan baru dimulai.",
		"status_refreshed":         "🔄 Status diperbarui!",
		"cwd_changed":              "Direktori kerja diubah ke: %s",
		"autodelete_set":           "✅ Auto-delete diatur ke <b>%d turns</b>.",
		"autodelete_disabled":      "🚫 Auto-delete percakapan Telegram <b>dinonaktifkan</b>.",
		"autodelete_cleared":       "🧹 Berhasil membersihkan <b>%d</b> pesan riwayat Telegram.",
		"autodelete_already_empty": "ℹ️ Tidak ada riwayat pesan lama yang tersimpan untuk dibersihkan.",
		"lang_changed":             "🌐 Bahasa antarmuka diubah ke: <b>Bahasa Indonesia</b> 🇮🇩",
		"perm_changed":             "Mode permission diubah ke: <b>%s</b>",
		"model_changed":            "Model aktif diubah ke: <code>%s</code>",
		"effort_changed":           "Reasoning effort diubah ke: <b>%s</b>",
		"no_conversation_history":  "📂 Belum ada riwayat sesi percakapan. Mulai percakapan baru dengan mengirim pesan langsung.",
		"no_artifacts_found":       "📑 <b>Tidak ada artifact ditemukan.</b>\nArtifact biasanya berupa rencana kerja (/plan), laporan PRD, atau dokumen terstruktur yang dibuat oleh Antigravity.",
		"btn_resume":               "📂 Resume Sesi",
		"btn_artifacts":            "📑 Artifacts & Plans",
		"btn_quota":                "📊 Quota/Usage",
		"btn_credits":              "💰 Credits",
		"btn_model":                "🧠 Ganti Model",
		"btn_effort":               "⚡ Set Effort",
		"btn_skills":               "🧰 Daftar Skills",
		"btn_status":               "⚙️ Status & CWD",
		"btn_permission":           "🔒 Atur Permission",
		"btn_autodelete":           "🧹 Auto-Delete",
		"btn_language":             "🌐 Bahasa / Lang",
		"btn_new_session":          "🔄 Sesi Baru",
		"btn_close_menu":           "🗑️ Tutup Menu",
		"btn_close":                "🗑️ Tutup",
		"btn_back_menu":            "« Menu",
		"btn_back":                 "« Kembali",
		"btn_refresh":              "🔄 Refresh",
		"btn_list_files":           "📁 List File (/ls)",
		"btn_clean_now":            "🧹 Bersihkan Chat Sekarang",
		"btn_cancel_task":          "🛑 Hentikan Tugas Aktif",
		"btn_open_file":            "📖 Buka Isi File",
		"btn_download_file":        "📥 Unduh Dokumen",
		"btn_approve":              "✅ Approve & Eksekusi",
		"btn_reject":               "❌ Reject / Revisi",
		"btn_accounts":             "👤 Akun Google",
		"btn_add_account":          "➕ Login Akun Baru",
		"btn_signout":              "🚪 Sign Out",
		"btn_switch_account":       "🔄 Beralih ke Akun Ini",
		"btn_delete_account":       "🗑️ Hapus Akun dari Daftar",
		"btn_open_google_login":    "🌐 Buka Link Login Google",
		"btn_cancel_login":         "❌ Batalkan Login",
		"btn_retry_login":          "🔄 Coba Login Baru",
		"btn_saved_accounts":       "👥 Daftar Akun Tersimpan",
		"btn_open_verify_link":     "🌐 Buka Link Verifikasi",
		"btn_open_link":            "🌐 Buka Tautan",
		"btn_view_queue":           "📋 Lihat Antrean",
		"btn_clear_queue":          "🧹 Hapus Antrean",
		"btn_cancel_all":           "🛑 Hentikan Semua",
	},
	"en": {
		"err_occurred":             "An error occurred:",
		"err_busy_wait":            "A task or process is currently running. Please wait for it to complete or use /cancel to stop it.",
		"task_cancelled":           "Active task was successfully cancelled.",
		"no_active_task":           "No task is currently running.",
		"new_session_started":      "New conversation session started.",
		"status_refreshed":         "🔄 Status refreshed!",
		"cwd_changed":              "Working directory changed to: %s",
		"autodelete_set":           "✅ Auto-delete limit set to <b>%d turns</b>.",
		"autodelete_disabled":      "🚫 Telegram chat auto-delete is now <b>disabled</b>.",
		"autodelete_cleared":       "🧹 Successfully cleaned <b>%d</b> Telegram history messages.",
		"autodelete_already_empty": "ℹ️ No tracked old messages found to clean.",
		"lang_changed":             "🌐 Interface language changed to: <b>English</b> 🇬🇧",
		"perm_changed":             "Permission mode changed to: <b>%s</b>",
		"model_changed":            "Active model changed to: <code>%s</code>",
		"effort_changed":           "Reasoning effort changed to: <b>%s</b>",
		"no_conversation_history":  "📂 No conversation history found. Start a new conversation by sending a message.",
		"no_artifacts_found":       "📑 <b>No artifacts found.</b>\nArtifacts are structured documents, plans (/plan), or PRD reports created by Antigravity.",
		"btn_resume":               "📂 Resume Session",
		"btn_artifacts":            "📑 Artifacts & Plans",
		"btn_quota":                "📊 Quota/Usage",
		"btn_credits":              "💰 Credits",
		"btn_model":                "🧠 Switch Model",
		"btn_effort":               "⚡ Set Effort",
		"btn_skills":               "🧰 List Skills",
		"btn_status":               "⚙️ Status & CWD",
		"btn_permission":           "🔒 Permission Mode",
		"btn_autodelete":           "🧹 Auto-Delete",
		"btn_language":             "🌐 Language / Bahasa",
		"btn_new_session":          "🔄 New Session",
		"btn_close_menu":           "🗑️ Close Menu",
		"btn_close":                "🗑️ Close",
		"btn_back_menu":            "« Main Menu",
		"btn_back":                 "« Back",
		"btn_refresh":              "🔄 Refresh",
		"btn_list_files":           "📁 List Files (/ls)",
		"btn_clean_now":            "🧹 Clean Chat History Now",
		"btn_cancel_task":          "🛑 Stop Active Task",
		"btn_open_file":            "📖 Preview File",
		"btn_download_file":        "📥 Download File",
		"btn_approve":              "✅ Approve & Execute",
		"btn_reject":               "❌ Reject / Revision",
		"btn_accounts":             "👤 Google Account",
		"btn_add_account":          "➕ Add New Account",
		"btn_signout":              "🚪 Sign Out",
		"btn_switch_account":       "🔄 Switch to This Account",
		"btn_delete_account":       "🗑️ Remove Account from List",
		"btn_open_google_login":    "🌐 Open Google Login Page",
		"btn_cancel_login":         "❌ Cancel Login",
		"btn_retry_login":          "🔄 Try Login Again",
		"btn_saved_accounts":       "👥 Saved Accounts List",
		"btn_open_verify_link":     "🌐 Open Verification Link",
		"btn_open_link":            "🌐 Open Link",
		"btn_view_queue":           "📋 View Queue",
		"btn_clear_queue":          "🧹 Clear Queue",
		"btn_cancel_all":           "🛑 Stop Everything",
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
		sb.WriteString("• <code>/cancel</code> or <code>/stop</code> — Stop the currently running process\n")
		sb.WriteString("• <code>/queue</code> — View explicit FIFO message queue (auto-run in order)\n")
		sb.WriteString("• <code>/clearqueue</code> — Drop all pending queued messages\n\n")

		sb.WriteString("<b>📊 CLI Status & Quota (Mode A):</b>\n")
		sb.WriteString("• <code>/usage</code> or <code>/quota</code> — Check 5-hour & weekly rate limits\n")
		sb.WriteString("• <code>/credits</code> — Check remaining G1 credits\n")
		sb.WriteString("• <code>/model [name]</code> — View or switch active model\n")
		sb.WriteString("• <code>/effort [level]</code> — Set reasoning effort (low/medium/high)\n")
		sb.WriteString("• <code>/skills</code> — List installed skills\n")
		sb.WriteString("• <code>/agents</code> — List available subagents\n\n")

		sb.WriteString("<b>👤 Google Account Management:</b>\n")
		sb.WriteString("• <code>/accounts</code> — Switch or manage connected Google accounts\n")
		sb.WriteString("• <code>/login</code> — Connect new Google account via OAuth\n")
		sb.WriteString("• <code>/signout</code> — Sign out of current Google account\n")
		sb.WriteString("• <code>/whoami</code> — View currently active account profile\n\n")

		sb.WriteString("<b>📂 Workspace & Session:</b>\n")
		sb.WriteString("• <code>/cwd [path]</code> — Show / change working directory\n")
		sb.WriteString("• <code>/ls [path]</code> — List files and folders on server\n")
		sb.WriteString("• <code>/pwd</code> — Show current active directory path\n")
		sb.WriteString("• <code>/new [path]</code> — Start a new conversation session\n")
		sb.WriteString("• <code>/sessions</code> — View conversation session history\n")
		sb.WriteString("• <code>/switch &lt;id|email&gt;</code> — Switch conversation or account\n")
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
		sb.WriteString("• <code>/cancel</code> atau <code>/stop</code> — Hentikan proses yang sedang berjalan\n")
		sb.WriteString("• <code>/queue</code> — Lihat antrean pesan FIFO (jalan otomatis berurutan)\n")
		sb.WriteString("• <code>/clearqueue</code> — Hapus semua pesan tertunda dalam antrean\n\n")

		sb.WriteString("<b>📊 CLI Status & Kuota (Mode A):</b>\n")
		sb.WriteString("• <code>/usage</code> atau <code>/quota</code> — Cek limit 5 jam & mingguan\n")
		sb.WriteString("• <code>/credits</code> — Cek sisa credit G1\n")
		sb.WriteString("• <code>/model [nama]</code> — Lihat atau ganti model aktif\n")
		sb.WriteString("• <code>/effort [level]</code> — Set reasoning effort (low/medium/high)\n")
		sb.WriteString("• <code>/skills</code> — Daftar skills yang terpasang\n")
		sb.WriteString("• <code>/agents</code> — Daftar subagents yang tersedia\n\n")

		sb.WriteString("<b>👤 Manajemen Akun Google:</b>\n")
		sb.WriteString("• <code>/accounts</code> — Kelola & beralih akun Google yang tersimpan\n")
		sb.WriteString("• <code>/login</code> — Hubungkan akun Google baru via OAuth\n")
		sb.WriteString("• <code>/signout</code> — Keluar dari akun Google aktif\n")
		sb.WriteString("• <code>/whoami</code> — Informasi profil akun Google yang sedang aktif\n\n")

		sb.WriteString("<b>📂 Workspace & Session:</b>\n")
		sb.WriteString("• <code>/cwd [path]</code> — Tampilkan / ganti direktori kerja\n")
		sb.WriteString("• <code>/ls [path]</code> — Tampilkan daftar file/folder di server\n")
		sb.WriteString("• <code>/pwd</code> — Tampilkan path direktori aktif saat ini\n")
		sb.WriteString("• <code>/new [path]</code> — Mulai percakapan sesi baru\n")
		sb.WriteString("• <code>/sessions</code> — Daftar riwayat sesi percakapan\n")
		sb.WriteString("• <code>/switch &lt;id|email&gt;</code> — Beralih sesi percakapan atau akun Google\n")
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

	activeAcc, _ := auth.GetActiveAccount()

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

		accountStr := "❌ <i>Not logged in</i>"
		if activeAcc != nil && activeAcc.Email != "" {
			accountStr = fmt.Sprintf("👤 <code>%s</code> (%s)", renderer.EscapeHTML(activeAcc.Email), renderer.EscapeHTML(activeAcc.Name))
		}

		return fmt.Sprintf(
			"⚙️ <b>Daemon Status</b>\n\n"+
				"• <b>Status</b>: %s\n"+
				"• <b>Google Account</b>: %s\n"+
				"• <b>Workspace (CWD)</b>: <code>%s</code>\n"+
				"• <b>Active Conversation</b>: <code>%s</code>\n"+
				"• <b>Permission Mode</b>: %s\n"+
				"• <b>Active Model</b>: <code>%s</code>\n"+
				"• <b>Reasoning Effort</b>: <code>%s</code>\n"+
				"• <b>Language</b>: %s\n"+
				"• <b>Auto-Delete</b>: %s\n"+
				"• <b>Last Active</b>: %s",
			stateStr,
			accountStr,
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

	accountStr := "❌ <i>Belum login</i>"
	if activeAcc != nil && activeAcc.Email != "" {
		accountStr = fmt.Sprintf("👤 <code>%s</code> (%s)", renderer.EscapeHTML(activeAcc.Email), renderer.EscapeHTML(activeAcc.Name))
	}

	return fmt.Sprintf(
		"⚙️ <b>Status Runtime Daemon</b>\n\n"+
			"• <b>Status</b>: %s\n"+
			"• <b>Akun Google</b>: %s\n"+
			"• <b>Workspace (CWD)</b>: <code>%s</code>\n"+
			"• <b>Active Conversation</b>: <code>%s</code>\n"+
			"• <b>Permission Mode</b>: %s\n"+
			"• <b>Active Model</b>: <code>%s</code>\n"+
			"• <b>Reasoning Effort</b>: <code>%s</code>\n"+
			"• <b>Bahasa / Language</b>: %s\n"+
			"• <b>Auto-Delete Telegram</b>: %s\n"+
			"• <b>Terakhir Aktif</b>: %s",
		stateStr,
		accountStr,
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

	isActive := currentLimit > 0

	if l == "en" {
		var statusLine string
		if isActive {
			statusLine = fmt.Sprintf("🟢 <b>Active</b> — deletes older messages when chat exceeds <b>%d turns</b>", currentLimit)
		} else {
			statusLine = "🔴 <b>Inactive</b> — no messages will be auto-deleted"
		}

		trackedLine := fmt.Sprintf("• <b>Currently Tracked:</b> <code>%d turn(s)</code>", trackedCount)
		if isActive && trackedCount >= currentLimit {
			trackedLine += fmt.Sprintf(" <i>(limit reached — older messages will be cleaned on next turn)</i>")
		} else if isActive {
			remaining := currentLimit - trackedCount
			trackedLine += fmt.Sprintf(" <i>(%d remaining before cleanup)</i>", remaining)
		}

		return fmt.Sprintf(
			"🧹 <b>Telegram Chat Auto-Delete</b>\n\n"+
				"%s\n\n"+
				"• <b>Limit:</b> %s\n"+
				"%s\n\n"+
				"<i>Note: This ONLY deletes messages inside Telegram chat. Session history, code artifacts, and transcripts on the Antigravity server remain completely safe.</i>",
			statusLine,
			func() string {
				if isActive {
					return fmt.Sprintf("<code>%d turns</code>", currentLimit)
				}
				return "<code>Off</code>"
			}(),
			trackedLine,
		)
	}

	// Indonesian
	var statusLine string
	if isActive {
		statusLine = fmt.Sprintf("🟢 <b>Aktif</b> — pesan lama dihapus otomatis jika chat melebihi <b>%d turn</b>", currentLimit)
	} else {
		statusLine = "🔴 <b>Nonaktif</b> — tidak ada pesan yang akan dihapus otomatis"
	}

	trackedLine := fmt.Sprintf("• <b>Pesan Terlacak Saat Ini:</b> <code>%d turn</code>", trackedCount)
	if isActive && trackedCount >= currentLimit {
		trackedLine += " <i>(batas tercapai — pesan lama akan dibersihkan pada giliran berikutnya)</i>"
	} else if isActive {
		remaining := currentLimit - trackedCount
		trackedLine += fmt.Sprintf(" <i>(%d turn lagi baru dibersihkan)</i>", remaining)
	}

	return fmt.Sprintf(
		"🧹 <b>Pengaturan Auto-Delete Telegram</b>\n\n"+
			"%s\n\n"+
			"• <b>Batas:</b> %s\n"+
			"%s\n\n"+
			"<i>Catatan: Fitur ini HANYA menghapus tampilan pesan di chat Telegram. Riwayat sesi, file transcript, dan kode di server Antigravity tetap tersimpan aman.</i>",
		statusLine,
		func() string {
			if isActive {
				return fmt.Sprintf("<code>%d turn</code>", currentLimit)
			}
			return "<code>Nonaktif</code>"
		}(),
		trackedLine,
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

// FormatAccountsList formats the list of saved Google accounts and current active status
func FormatAccountsList(lang string, active *auth.AccountInfo, accounts []auth.AccountInfo) string {
	l := NormalizeLang(lang)
	var sb strings.Builder

	if l == "en" {
		sb.WriteString("👤 <b>Google Accounts Management</b>\n\n")
		if active != nil && active.Email != "" {
			sb.WriteString(fmt.Sprintf("• <b>Active Account:</b> <code>%s</code> (%s)\n", renderer.EscapeHTML(active.Email), renderer.EscapeHTML(active.Name)))
			if !active.Expiry.IsZero() {
				sb.WriteString(fmt.Sprintf("• <b>Token Expiry:</b> %s\n", active.Expiry.Format("15:04:05 02 Jan 2006")))
			}
			sb.WriteString(fmt.Sprintf("• <b>Auth Method:</b> %s\n\n", active.AuthMethod))
		} else {
			sb.WriteString("• <b>Active Account:</b> <i>Not logged in</i> ⚠️\n\n")
		}

		sb.WriteString("<b>Saved Accounts:</b>\n")
		if len(accounts) == 0 {
			sb.WriteString("<i>No saved accounts yet. Click 'Add New Account' to log in.</i>\n")
		} else {
			for _, a := range accounts {
				indicator := "▫️"
				status := ""
				if a.IsActive {
					indicator = "🟢"
					status = " <b>(Active)</b>"
				}
				sb.WriteString(fmt.Sprintf("%s <code>%s</code> (%s)%s\n", indicator, renderer.EscapeHTML(a.Email), renderer.EscapeHTML(a.Name), status))
			}
		}
		sb.WriteString("\n<i>Click an account button below to view details or switch:</i>")
	} else {
		sb.WriteString("👤 <b>Manajemen Akun Google Antigravity</b>\n\n")
		if active != nil && active.Email != "" {
			sb.WriteString(fmt.Sprintf("• <b>Akun Aktif:</b> <code>%s</code> (%s)\n", renderer.EscapeHTML(active.Email), renderer.EscapeHTML(active.Name)))
			if !active.Expiry.IsZero() {
				sb.WriteString(fmt.Sprintf("• <b>Token Kadaluwarsa:</b> %s\n", active.Expiry.Format("15:04:05 02 Jan 2006")))
			}
			sb.WriteString(fmt.Sprintf("• <b>Metode Login:</b> %s\n\n", active.AuthMethod))
		} else {
			sb.WriteString("• <b>Akun Aktif:</b> <i>Belum login</i> ⚠️\n\n")
		}

		sb.WriteString("<b>Daftar Akun Tersimpan:</b>\n")
		if len(accounts) == 0 {
			sb.WriteString("<i>Belum ada akun tersimpan. Klik 'Login Akun Baru' untuk masuk.</i>\n")
		} else {
			for _, a := range accounts {
				indicator := "▫️"
				status := ""
				if a.IsActive {
					indicator = "🟢"
					status = " <b>(Aktif)</b>"
				}
				sb.WriteString(fmt.Sprintf("%s <code>%s</code> (%s)%s\n", indicator, renderer.EscapeHTML(a.Email), renderer.EscapeHTML(a.Name), status))
			}
		}
		sb.WriteString("\n<i>Klik tombol akun di bawah untuk melihat detail atau beralih akun:</i>")
	}

	return sb.String()
}

// FormatAccountDetail formats a single account view
func FormatAccountDetail(lang string, acc *auth.AccountInfo) string {
	l := NormalizeLang(lang)
	var sb strings.Builder

	if l == "en" {
		status := "Saved (Inactive)"
		if acc.IsActive {
			status = "🟢 Active"
		}
		sb.WriteString("👤 <b>Account Details</b>\n\n")
		sb.WriteString(fmt.Sprintf("• <b>Email:</b> <code>%s</code>\n", renderer.EscapeHTML(acc.Email)))
		sb.WriteString(fmt.Sprintf("• <b>Name:</b> %s\n", renderer.EscapeHTML(acc.Name)))
		if acc.Tier != "" {
			sb.WriteString(fmt.Sprintf("• <b>Plan:</b> %s\n", auth.FormatTierBadge(acc.Tier)))
		}
		sb.WriteString(fmt.Sprintf("• <b>Status:</b> %s\n", status))
		sb.WriteString(fmt.Sprintf("• <b>Auth Method:</b> %s\n", acc.AuthMethod))
		if !acc.Expiry.IsZero() {
			sb.WriteString(fmt.Sprintf("• <b>Token Expiry:</b> %s\n", acc.Expiry.Format("15:04:05 02 Jan 2006")))
		}
		sb.WriteString("\nChoose an action below:")
	} else {
		status := "Tersimpan (Tidak Aktif)"
		if acc.IsActive {
			status = "🟢 Aktif"
		}
		sb.WriteString("👤 <b>Detail Akun Google</b>\n\n")
		sb.WriteString(fmt.Sprintf("• <b>Email:</b> <code>%s</code>\n", renderer.EscapeHTML(acc.Email)))
		sb.WriteString(fmt.Sprintf("• <b>Nama:</b> %s\n", renderer.EscapeHTML(acc.Name)))
		if acc.Tier != "" {
			sb.WriteString(fmt.Sprintf("• <b>Langganan:</b> %s\n", auth.FormatTierBadge(acc.Tier)))
		}
		sb.WriteString(fmt.Sprintf("• <b>Status:</b> %s\n", status))
		sb.WriteString(fmt.Sprintf("• <b>Metode Login:</b> %s\n", acc.AuthMethod))
		if !acc.Expiry.IsZero() {
			sb.WriteString(fmt.Sprintf("• <b>Token Kadaluwarsa:</b> %s\n", acc.Expiry.Format("15:04:05 02 Jan 2006")))
		}
		sb.WriteString("\nPilih tindakan di bawah:")
	}

	return sb.String()
}

// FormatLoginPrompt formats the OAuth login link and step-by-step instructions
func FormatLoginPrompt(lang, authURL string) string {
	l := NormalizeLang(lang)
	var sb strings.Builder

	if l == "en" {
		sb.WriteString("🔐 <b>Google Antigravity CLI Login</b>\n\n")
		sb.WriteString("• <b>Method:</b> Google OAuth 2.0 (Consumer)\n")
		sb.WriteString("• <b>Platform:</b> Official Google Accounts Authorization\n\n")
		sb.WriteString(fmt.Sprintf("👉 <a href=\"%s\">Click Here to Authorize Google Account</a>\n\n", authURL))
		sb.WriteString("<b>Instructions:</b>\n")
		sb.WriteString("1. Click the link above or the button below to open the login page in your browser.\n")
		sb.WriteString("2. Select your Google account and grant permissions to Google Antigravity.\n")
		sb.WriteString("3. Copy the <b>Authorization Code</b> (or the entire redirect URL) displayed on screen.\n")
		sb.WriteString("4. Paste and send the code directly into this chat (or type <code>/code &lt;code&gt;</code>).\n\n")
		sb.WriteString("⏳ <i>Waiting for authorization code (timeout 3 minutes)...</i>")
	} else {
		sb.WriteString("🔐 <b>Login Google Antigravity CLI</b>\n\n")
		sb.WriteString("• <b>Metode Login:</b> Google OAuth 2.0 (Consumer)\n")
		sb.WriteString("• <b>Platform:</b> Otorisasi Akun Google Resmi\n\n")
		sb.WriteString(fmt.Sprintf("👉 <a href=\"%s\">Klik di Sini untuk Login ke Akun Google</a>\n\n", authURL))
		sb.WriteString("<b>Langkah-langkah:</b>\n")
		sb.WriteString("1. Klik link di atas atau tombol di bawah untuk membuka halaman login di browser.\n")
		sb.WriteString("2. Pilih akun Google Anda dan setujui izin akses Google Antigravity.\n")
		sb.WriteString("3. Salin <b>Kode Otorisasi</b> (atau seluruh link URL redirect) yang muncul di layar.\n")
		sb.WriteString("4. Kirimkan kode tersebut langsung ke chat bot ini (atau ketik <code>/code &lt;kode&gt;</code>).\n\n")
		sb.WriteString("⏳ <i>Menunggu kode otorisasi (batas waktu 3 menit)...</i>")
	}

	return sb.String()
}

// FormatLoginSuccess formats successful login confirmation
func FormatLoginSuccess(lang string, acc *auth.AccountInfo) string {
	l := NormalizeLang(lang)
	var sb strings.Builder

	if l == "en" {
		sb.WriteString("✅ <b>Login Successful!</b>\n\n")
		sb.WriteString(fmt.Sprintf("• <b>Account:</b> <code>%s</code> (%s)\n", renderer.EscapeHTML(acc.Email), renderer.EscapeHTML(acc.Name)))
		sb.WriteString(fmt.Sprintf("• <b>Auth Method:</b> %s\n", acc.AuthMethod))
		if !acc.Expiry.IsZero() {
			sb.WriteString(fmt.Sprintf("• <b>Token Expiry:</b> %s\n", acc.Expiry.Format("15:04:05 02 Jan 2006")))
		}
		sb.WriteString("\n🚀 <i>Account is active and saved. You can now use Antigravity CLI freely!</i>")
	} else {
		sb.WriteString("✅ <b>Login Berhasil!</b>\n\n")
		sb.WriteString(fmt.Sprintf("• <b>Akun:</b> <code>%s</code> (%s)\n", renderer.EscapeHTML(acc.Email), renderer.EscapeHTML(acc.Name)))
		sb.WriteString(fmt.Sprintf("• <b>Metode:</b> %s\n", acc.AuthMethod))
		if !acc.Expiry.IsZero() {
			sb.WriteString(fmt.Sprintf("• <b>Token Kadaluwarsa:</b> %s\n", acc.Expiry.Format("15:04:05 02 Jan 2006")))
		}
		sb.WriteString("\n🚀 <i>Akun telah aktif dan tersimpan. Anda dapat langsung menggunakan Antigravity CLI!</i>")
	}

	return sb.String()
}

// FormatSignOutSuccess formats signout confirmation
func FormatSignOutSuccess(lang string, email string) string {
	l := NormalizeLang(lang)
	var sb strings.Builder

	if l == "en" {
		sb.WriteString("🚪 <b>Signed Out Successfully!</b>\n\n")
		if email != "" {
			sb.WriteString(fmt.Sprintf("Account <code>%s</code> has been signed out and archived to your saved accounts.\n\n", renderer.EscapeHTML(email)))
		} else {
			sb.WriteString("Active account has been signed out.\n\n")
		}
		sb.WriteString("Choose an action below to switch accounts or sign in:")
	} else {
		sb.WriteString("🚪 <b>Sign Out Berhasil!</b>\n\n")
		if email != "" {
			sb.WriteString(fmt.Sprintf("Akun <code>%s</code> telah keluar dan disimpan ke daftar akun tersimpan.\n\n", renderer.EscapeHTML(email)))
		} else {
			sb.WriteString("Akun aktif telah keluar.\n\n")
		}
		sb.WriteString("Pilih tindakan di bawah untuk beralih akun atau login kembali:")
	}

	return sb.String()
}

// FormatAuthError formats auth error messages
func FormatAuthError(lang, errMsg string) string {
	l := NormalizeLang(lang)
	var sb strings.Builder

	if l == "en" {
		sb.WriteString("❌ <b>Login / Authorization Failed:</b>\n\n")
		sb.WriteString(fmt.Sprintf("<code>%s</code>\n\n", renderer.EscapeHTML(errMsg)))
		sb.WriteString("Please verify your authorization code and try again using the buttons below:")
	} else {
		sb.WriteString("❌ <b>Login / Otorisasi Gagal:</b>\n\n")
		sb.WriteString(fmt.Sprintf("<code>%s</code>\n\n", renderer.EscapeHTML(errMsg)))
		sb.WriteString("Pastikan kode otorisasi benar dan belum kedaluwarsa, lalu coba kembali dengan tombol di bawah:")
	}

	return sb.String()
}

// FormatConversationHistory formats the last N user prompts from a session transcript
// into a readable Telegram HTML card. Used after /resume to show conversation context.
func FormatConversationHistory(lang string, title string, entries []session.ConversationHistoryEntry) string {
	l := NormalizeLang(lang)
	var sb strings.Builder

	if l == "en" {
		sb.WriteString(fmt.Sprintf("📜 <b>Recent History: %s</b>\n", renderer.EscapeHTML(title)))
		sb.WriteString("<i>Last messages you sent in this session:</i>\n\n")
	} else {
		sb.WriteString(fmt.Sprintf("📜 <b>Riwayat Terakhir: %s</b>\n", renderer.EscapeHTML(title)))
		sb.WriteString("<i>Pesan-pesan terakhir yang Anda kirim di sesi ini:</i>\n\n")
	}

	if len(entries) == 0 {
		if l == "en" {
			sb.WriteString("<i>No message history found for this session.</i>")
		} else {
			sb.WriteString("<i>Tidak ada riwayat pesan yang ditemukan untuk sesi ini.</i>")
		}
		return sb.String()
	}

	for i, e := range entries {
		num := fmt.Sprintf("%d.", i+1)
		text := e.Text
		// Truncate long messages to keep card readable
		runes := []rune(text)
		if len(runes) > 200 {
			text = string(runes[:197]) + "..."
		}
		// Collapse newlines for compact preview
		text = strings.Join(strings.Fields(text), " ")

		if e.Timestamp != "" {
			sb.WriteString(fmt.Sprintf("<b>%s</b> <i>[%s]</i>\n", num, renderer.EscapeHTML(e.Timestamp)))
		} else {
			sb.WriteString(fmt.Sprintf("<b>%s</b>\n", num))
		}
		sb.WriteString(fmt.Sprintf("👤 %s\n\n", renderer.EscapeHTML(text)))
	}

	if l == "en" {
		sb.WriteString("<i>💬 Send any message below to continue this conversation.</i>")
	} else {
		sb.WriteString("<i>💬 Kirim pesan apa saja untuk melanjutkan percakapan ini.</i>")
	}

	return sb.String()
}
