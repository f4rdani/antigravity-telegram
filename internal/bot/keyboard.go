package bot

import (
	"fmt"

	"agy-tele/internal/session"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func QuickActionKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📂 Resume Sesi", "cmd_resume_menu"),
			tgbotapi.NewInlineKeyboardButtonData("🔄 Sesi Baru", "cmd_new"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 Quota/Usage", "cmd_usage"),
			tgbotapi.NewInlineKeyboardButtonData("💰 Credits", "cmd_credits"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🧠 Ganti Model", "cmd_model_menu"),
			tgbotapi.NewInlineKeyboardButtonData("⚡ Set Effort", "cmd_effort_menu"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🧰 Daftar Skills", "cmd_skills"),
			tgbotapi.NewInlineKeyboardButtonData("⚙️ Status & CWD", "cmd_status"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔒 Atur Permission", "cmd_perm_menu"),
			tgbotapi.NewInlineKeyboardButtonData("🗑️ Tutup Menu", "cmd_delete_msg"),
		),
	)
}

func ResumeKeyboard(convs []session.AvailableConversation, activeID string) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	for i, c := range convs {
		prefix := fmt.Sprintf("💬 %d. ", i+1)
		if c.ID == activeID {
			prefix = fmt.Sprintf("🌟 %d. ", i+1)
		}
		btnText := fmt.Sprintf("%s%s", prefix, c.Title)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(btnText, "resume_id:"+c.ID),
		))
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("➕ Mulai Sesi Baru", "cmd_new"),
		tgbotapi.NewInlineKeyboardButtonData("« Menu", "cmd_help_menu"),
		tgbotapi.NewInlineKeyboardButtonData("🗑️ Tutup", "cmd_delete_msg"),
	))

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func ResumeConfirmedKeyboard(convID string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💬 Lanjut Percakapan Terakhir", "resume_continue:"+convID),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("« Pilih Sesi Lain", "cmd_resume_menu"),
			tgbotapi.NewInlineKeyboardButtonData("🗑️ Tutup", "cmd_delete_msg"),
		),
	)
}

func ModelSelectionKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Gemini 3.8 Flash (High)", "set_model:gemini-3.8-flash-high"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Gemini 3.7 Flash (High)", "set_model:gemini-3.7-flash-high"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Gemini 3.1 Pro (High)", "set_model:gemini-3.1-pro-high"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Claude Sonnet 4.6", "set_model:claude-sonnet-4-6"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Claude Opus 4.6 Thinking", "set_model:claude-opus-4-6-thinking"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("GPT-OSS 120B (Medium)", "set_model:gpt-oss-120b-medium"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("« Kembali", "cmd_help_menu"),
			tgbotapi.NewInlineKeyboardButtonData("🗑️ Tutup", "cmd_delete_msg"),
		),
	)
}

func EffortSelectionKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Low (Cepat)", "set_effort:low"),
			tgbotapi.NewInlineKeyboardButtonData("Medium", "set_effort:medium"),
			tgbotapi.NewInlineKeyboardButtonData("High (Mendalam)", "set_effort:high"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("« Kembali", "cmd_help_menu"),
			tgbotapi.NewInlineKeyboardButtonData("🗑️ Tutup", "cmd_delete_msg"),
		),
	)
}

func PermissionSelectionKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🟢 Auto-Approve (Hands-Free)", "set_perm:auto"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🟡 Ask User (Konfirmasi Tiap Aksi)", "set_perm:ask"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("« Kembali", "cmd_help_menu"),
			tgbotapi.NewInlineKeyboardButtonData("🗑️ Tutup", "cmd_delete_msg"),
		),
	)
}

func BackAndCloseKeyboard(backCmd string) tgbotapi.InlineKeyboardMarkup {
	if backCmd == "" {
		backCmd = "cmd_help_menu"
	}
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("« Kembali ke Menu", backCmd),
			tgbotapi.NewInlineKeyboardButtonData("🗑️ Tutup", "cmd_delete_msg"),
		),
	)
}

func CloseOnlyKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🗑️ Tutup Pesan Ini", "cmd_delete_msg"),
		),
	)
}

func RefreshAndBackKeyboard(refreshCmd string, backCmd string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", refreshCmd),
			tgbotapi.NewInlineKeyboardButtonData("« Kembali", backCmd),
			tgbotapi.NewInlineKeyboardButtonData("🗑️ Tutup", "cmd_delete_msg"),
		),
	)
}
