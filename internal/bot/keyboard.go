package bot

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func QuickActionKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
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
			tgbotapi.NewInlineKeyboardButtonData("🔄 Sesi Baru", "cmd_new"),
			tgbotapi.NewInlineKeyboardButtonData("🔒 Atur Permission", "cmd_perm_menu"),
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
			tgbotapi.NewInlineKeyboardButtonData("« Kembali ke Menu", "cmd_help_menu"),
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
			tgbotapi.NewInlineKeyboardButtonData("« Kembali ke Menu", "cmd_help_menu"),
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
			tgbotapi.NewInlineKeyboardButtonData("« Kembali ke Menu", "cmd_help_menu"),
		),
	)
}
