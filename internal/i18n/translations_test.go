package i18n

import (
	"strings"
	"testing"
	"time"

	"agy-tele/internal/engine"
	"agy-tele/internal/session"
)

func TestI18nTranslations(t *testing.T) {
	// Test T helper
	idText := T("id", "status_refreshed")
	if !strings.Contains(idText, "Status diperbarui") {
		t.Errorf("expected 'Status diperbarui', got %q", idText)
	}

	enText := T("en", "status_refreshed")
	if !strings.Contains(enText, "Status refreshed") {
		t.Errorf("expected 'Status refreshed', got %q", enText)
	}

	// Test GetHelpText
	helpID := GetHelpText("id", "v1.0.10")
	if !strings.Contains(helpID, "Kirim teks biasa") {
		t.Errorf("helpID missing Indonesian content")
	}

	helpEN := GetHelpText("en", "v1.0.10")
	if !strings.Contains(helpEN, "Send plain text") {
		t.Errorf("helpEN missing English content")
	}

	// Test GetStatusText
	sess := &session.UserSession{
		UserID:           1,
		CWD:              "/root",
		PermissionMode:   "auto",
		Language:         "en",
		MaxTelegramTurns: 50,
		LastActiveTime:   time.Now(),
	}
	statusEN := GetStatusText("en", sess, false, "")
	if !strings.Contains(statusEN, "English") || !strings.Contains(statusEN, "50 turns") {
		t.Errorf("statusEN missing expected English fields: %s", statusEN)
	}

	statusID := GetStatusText("id", sess, false, "")
	if !strings.Contains(statusID, "Bahasa Indonesia") || !strings.Contains(statusID, "50 turn") {
		t.Errorf("statusID missing expected Indonesian fields: %s", statusID)
	}

	// Test DescribeStepAction
	step := &engine.StepUpdatePayload{
		StepType: "tool",
		ToolName: "run_command",
		ToolInfo: &engine.ToolInfoPayload{
			Name: "run_command",
			Parameters: map[string]interface{}{
				"CommandLine": "ls -la",
			},
		},
	}
	actionID := DescribeStepAction("id", step)
	if !strings.Contains(actionID, "Menjalankan") {
		t.Errorf("actionID missing 'Menjalankan': %s", actionID)
	}

	actionEN := DescribeStepAction("en", step)
	if !strings.Contains(actionEN, "Running") {
		t.Errorf("actionEN missing 'Running': %s", actionEN)
	}
}
