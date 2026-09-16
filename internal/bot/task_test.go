package bot

import (
	"testing"
	"time"

	"agy-tele/config"
	"agy-tele/internal/session"
)

func TestActiveTaskManagement(t *testing.T) {
	cfg := config.DefaultConfig()
	sm, err := session.NewSessionManager("/tmp/test_session_task.json", "/", "auto")
	if err != nil {
		t.Fatalf("Failed to create session manager: %v", err)
	}

	r := NewRouter(cfg, nil, sm)

	userID := int64(12345)
	chatID := int64(67890)

	// Initially no task
	if task := r.getActiveTask(userID); task != nil {
		t.Fatalf("Expected nil task initially, got %+v", task)
	}

	cancelled := false
	cancelFunc := func() {
		cancelled = true
	}

	// Register task
	task := r.registerActiveTask(userID, chatID, "Test prompt", cancelFunc, false)
	if task == nil {
		t.Fatalf("Expected registered task, got nil")
	}

	// Verify retrieval
	retrieved := r.getActiveTask(userID)
	if retrieved == nil || retrieved.Prompt != "Test prompt" {
		t.Fatalf("Expected retrieved task with 'Test prompt', got %+v", retrieved)
	}

	// Cancel task
	ok := r.cancelActiveTask(userID, chatID)
	if !ok {
		t.Fatalf("Expected cancelActiveTask to return true")
	}
	if !cancelled {
		t.Fatalf("Expected cancelFunc to be called")
	}

	// Verify task cleared
	if taskAfter := r.getActiveTask(userID); taskAfter != nil {
		t.Fatalf("Expected nil task after cancel, got %+v", taskAfter)
	}
}

func TestFormatStatusRunning(t *testing.T) {
	sess := &session.UserSession{
		UserID:               123,
		ChatID:               456,
		ActiveConversationID: "abc12345",
		CWD:                  "/root",
		PermissionMode:       "auto",
		ActiveModel:          "gemini-2.5-flash",
		ActiveEffort:         "high",
		LastActiveTime:       time.Now(),
	}

	idleStatus := FormatStatus(sess, false, "")
	if !containsString(idleStatus, "🟢 IDLE") {
		t.Fatalf("Expected idle status to contain '🟢 IDLE', got:\n%s", idleStatus)
	}

	runningStatus := FormatStatus(sess, true, "Running complex task (15s)")
	if !containsString(runningStatus, "SEDANG MEMPROSES TUGAS") || !containsString(runningStatus, "Running complex task") {
		t.Fatalf("Expected running status to contain running info, got:\n%s", runningStatus)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(substr) > 0 && len(s) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
