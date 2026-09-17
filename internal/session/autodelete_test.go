package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSessionAutoDeleteAndLanguage(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "agy-session-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sessionFile := filepath.Join(tempDir, "sessions.json")
	sm, err := NewSessionManager(sessionFile, "/tmp", "auto")
	if err != nil {
		t.Fatalf("failed to init session manager: %v", err)
	}

	userID := int64(12345)
	sess := sm.GetSession(userID, 67890)
	if sess.Language != "id" {
		t.Errorf("default Language = %q, want 'id'", sess.Language)
	}
	if sess.MaxTelegramTurns != 50 {
		t.Errorf("default MaxTelegramTurns = %d, want 50", sess.MaxTelegramTurns)
	}

	// Test Language change
	sm.SetLanguage(userID, "en")
	sess = sm.GetSession(userID, 67890)
	if sess.Language != "en" {
		t.Errorf("expected Language = 'en', got %q", sess.Language)
	}

	// Set limit to 3 turns
	sm.SetMaxTelegramTurns(userID, 3)

	// Turn 1 (with split messages, e.g. 2 chunks)
	toDel := sm.RecordTurnMessages(userID, []int{101, 1012}, 201)
	if len(toDel) != 0 {
		t.Fatalf("expected 0 deletions on turn 1, got %d", len(toDel))
	}

	// Turn 2
	toDel = sm.RecordTurnMessages(userID, []int{102}, 202)
	if len(toDel) != 0 {
		t.Fatalf("expected 0 deletions on turn 2, got %d", len(toDel))
	}

	// Turn 3
	toDel = sm.RecordTurnMessages(userID, []int{103}, 203)
	if len(toDel) != 0 {
		t.Fatalf("expected 0 deletions on turn 3, got %d", len(toDel))
	}

	// Turn 4 -> should evict turn 1
	toDel = sm.RecordTurnMessages(userID, []int{104}, 204)
	if len(toDel) != 1 {
		t.Fatalf("expected 1 deletion on turn 4, got %d", len(toDel))
	}
	if toDel[0].UserMsgID != 101 || toDel[0].BotMsgID != 201 {
		t.Errorf("expected evicted turn (101, 201), got (%d, %d)", toDel[0].UserMsgID, toDel[0].BotMsgID)
	}
	allIDs := toDel[0].GetAllUserMsgIDs()
	if len(allIDs) != 2 || allIDs[0] != 101 || allIDs[1] != 1012 {
		t.Errorf("expected evicted all IDs [101, 1012], got %v", allIDs)
	}

	sess = sm.GetSession(userID, 67890)
	if len(sess.TrackedTurns) != 3 {
		t.Errorf("expected 3 tracked turns remaining, got %d", len(sess.TrackedTurns))
	}

	// Test ClearAllTrackedTurns
	cleared := sm.ClearAllTrackedTurns(userID)
	if len(cleared) != 3 {
		t.Errorf("expected 3 cleared turns, got %d", len(cleared))
	}
	sess = sm.GetSession(userID, 67890)
	if len(sess.TrackedTurns) != 0 {
		t.Errorf("expected 0 tracked turns after clear, got %d", len(sess.TrackedTurns))
	}

	// Test disabled limit (-1)
	sm.SetMaxTelegramTurns(userID, -1)
	for i := 1; i <= 5; i++ {
		toDel = sm.RecordTurnMessages(userID, []int{100 + i}, 200+i)
		if len(toDel) != 0 {
			t.Errorf("disabled limit should not evict, got %d items", len(toDel))
		}
	}
	sess = sm.GetSession(userID, 67890)
	if len(sess.TrackedTurns) != 5 {
		t.Errorf("expected 5 turns tracked when disabled, got %d", len(sess.TrackedTurns))
	}
}
