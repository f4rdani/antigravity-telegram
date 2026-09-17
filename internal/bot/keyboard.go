package bot

import (
	"fmt"

	"agy-tele/internal/artifact"
	"agy-tele/internal/i18n"
	"agy-tele/internal/session"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func QuickActionKeyboard(lang string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_resume"), "cmd_resume_menu"),
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_artifacts"), "cmd_artifact_menu"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_quota"), "cmd_usage"),
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_credits"), "cmd_credits"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_model"), "cmd_model_menu"),
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_effort"), "cmd_effort_menu"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_skills"), "cmd_skills"),
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_status"), "cmd_status"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_permission"), "cmd_perm_menu"),
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_new_session"), "cmd_new"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_autodelete"), "cmd_autodelete_menu"),
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_language"), "cmd_lang_menu"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_close_menu"), "cmd_delete_msg"),
		),
	)
}

func StatusActionKeyboard(lang string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_list_files"), "cmd_ls_cwd"),
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_refresh"), "cmd_status"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_back_menu"), "cmd_help_menu"),
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_close"), "cmd_delete_msg"),
		),
	)
}

func AutoDeleteKeyboard(lang string, currentLimit int) tgbotapi.InlineKeyboardMarkup {
	t20 := "20 Turns"
	if currentLimit == 20 {
		t20 = "✅ 20 Turns"
	}
	t50 := "50 Turns"
	if currentLimit == 50 {
		t50 = "✅ 50 Turns"
	}
	t100 := "100 Turns"
	if currentLimit == 100 {
		t100 = "✅ 100 Turns"
	}

	tOff := "❌ Nonaktif (Off)"
	if i18n.NormalizeLang(lang) == "en" {
		tOff = "❌ Disabled (Off)"
	}
	if currentLimit <= 0 {
		tOff = "✅ " + tOff
	}

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t20, "set_autodelete:20"),
			tgbotapi.NewInlineKeyboardButtonData(t50, "set_autodelete:50"),
			tgbotapi.NewInlineKeyboardButtonData(t100, "set_autodelete:100"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(tOff, "set_autodelete:0"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_clean_now"), "cmd_clean_chat_now"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_back_menu"), "cmd_help_menu"),
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_close"), "cmd_delete_msg"),
		),
	)
}

func LanguageSelectionKeyboard(currentLang string) tgbotapi.InlineKeyboardMarkup {
	l := i18n.NormalizeLang(currentLang)
	idText := "🇮🇩 Bahasa Indonesia"
	enText := "🇬🇧 English"

	if l == "id" {
		idText = "✅ 🇮🇩 Bahasa Indonesia"
	} else if l == "en" {
		enText = "✅ 🇬🇧 English"
	}

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(idText, "set_lang:id"),
			tgbotapi.NewInlineKeyboardButtonData(enText, "set_lang:en"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(l, "btn_back_menu"), "cmd_help_menu"),
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(l, "btn_close"), "cmd_delete_msg"),
		),
	)
}

func ResumeKeyboard(convs []session.AvailableConversation, activeID string, lang string) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	for i, c := range convs {
		tag := ""
		if c.ID == activeID {
			tag = " 🌟"
		}
		timeInfo := ""
		if c.TimeLabel != "" {
			timeInfo = " (" + c.TimeLabel + ")"
		}
		btnText := fmt.Sprintf("%d. %s%s%s", i+1, c.Title, timeInfo, tag)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(btnText, "resume_id:"+c.ID),
		))
	}

	newSessionText := "➕ Sesi Baru"
	if i18n.NormalizeLang(lang) == "en" {
		newSessionText = "➕ New Session"
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(newSessionText, "cmd_new"),
		tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_back_menu"), "cmd_help_menu"),
		tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_close"), "cmd_delete_msg"),
	))

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func ResumeConfirmedKeyboard(convID string, lang string) tgbotapi.InlineKeyboardMarkup {
	chooseOtherText := "« Pilih Sesi Lain"
	if i18n.NormalizeLang(lang) == "en" {
		chooseOtherText = "« Choose Another Session"
	}

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(chooseOtherText, "cmd_resume_menu"),
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_close"), "cmd_delete_msg"),
		),
	)
}

func ModelSelectionKeyboard(lang string) tgbotapi.InlineKeyboardMarkup {
	defaultText := "🔄 Default (Otomatis agy)"
	if i18n.NormalizeLang(lang) == "en" {
		defaultText = "🔄 Default (agy automatic)"
	}

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
			tgbotapi.NewInlineKeyboardButtonData(defaultText, "set_model:default"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_back"), "cmd_help_menu"),
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_close"), "cmd_delete_msg"),
		),
	)
}

func EffortSelectionKeyboard(lang string) tgbotapi.InlineKeyboardMarkup {
	lowText := "Low (Cepat)"
	highText := "High (Mendalam)"
	if i18n.NormalizeLang(lang) == "en" {
		lowText = "Low (Fast)"
		highText = "High (Deep)"
	}

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(lowText, "set_effort:low"),
			tgbotapi.NewInlineKeyboardButtonData("Medium", "set_effort:medium"),
			tgbotapi.NewInlineKeyboardButtonData(highText, "set_effort:high"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_back"), "cmd_help_menu"),
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_close"), "cmd_delete_msg"),
		),
	)
}

func PermissionSelectionKeyboard(lang string) tgbotapi.InlineKeyboardMarkup {
	autoText := "🟢 Auto-Approve (Hands-Free)"
	askText := "🟡 Ask User (Konfirmasi Tiap Aksi)"
	if i18n.NormalizeLang(lang) == "en" {
		askText = "🟡 Ask User (Confirm Every Action)"
	}

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(autoText, "set_perm:auto"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(askText, "set_perm:ask"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_back"), "cmd_help_menu"),
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_close"), "cmd_delete_msg"),
		),
	)
}

func BackAndCloseKeyboard(backCmd string, lang string) tgbotapi.InlineKeyboardMarkup {
	if backCmd == "" {
		backCmd = "cmd_help_menu"
	}
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_back_menu"), backCmd),
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_close"), "cmd_delete_msg"),
		),
	)
}

func CloseOnlyKeyboard(lang string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_close"), "cmd_delete_msg"),
		),
	)
}

func RefreshAndBackKeyboard(refreshCmd string, backCmd string, lang string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_refresh"), refreshCmd),
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_back"), backCmd),
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_close"), "cmd_delete_msg"),
		),
	)
}

func ArtifactListKeyboard(items []artifact.Item, lang string) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	for i, item := range items {
		status := "📄"
		if item.RequestFeedback {
			status = "🔔"
		}
		timeStr := ""
		if item.TimeLabel != "" {
			timeStr = " (" + item.TimeLabel + ")"
		}
		btnText := fmt.Sprintf("%s %d. %s%s", status, i+1, item.ID, timeStr)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(btnText, "art_select:"+item.ID),
		))
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_refresh"), "cmd_artifact_menu"),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_back_menu"), "cmd_help_menu"),
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_close"), "cmd_delete_msg"),
		)[0],
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_back_menu"), "cmd_help_menu"),
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_close"), "cmd_delete_msg"),
		)[1],
	))

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func ArtifactDetailKeyboard(item artifact.Item, lang string) tgbotapi.InlineKeyboardMarkup {
	backListText := "« Kembali ke Daftar"
	if i18n.NormalizeLang(lang) == "en" {
		backListText = "« Back to List"
	}

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_open_file"), "art_open:"+item.ID),
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_download_file"), "art_download:"+item.ID),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_approve"), "art_approve:"+item.ID),
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_reject"), "art_reject:"+item.ID),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(backListText, "cmd_artifact_menu"),
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_close"), "cmd_delete_msg"),
		),
	)
}

func ActiveTaskKeyboard(lang string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_cancel_task"), "cmd_cancel_active_task"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(i18n.T(lang, "btn_close"), "cmd_delete_msg"),
		),
	)
}
