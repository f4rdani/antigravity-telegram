package throttler

import (
	"testing"
	"time"
)

func TestActivityTrackerLifecycle(t *testing.T) {
	// Create mock tracker without active bot API to verify non-blocking calls
	tracker := &ActivityTracker{
		chatID:    123456,
		messageID: 999,
		lastText:  "initial",
		updateCh:  make(chan string, 16),
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}

	// Calling Update should be non-blocking
	tracker.Update("action 1")
	tracker.Update("action 2")

	// Verify MessageID returns the id
	if tracker.MessageID() != 999 {
		t.Errorf("MessageID() = %d, want 999", tracker.MessageID())
	}

	// Close doneCh to simulate worker finish
	close(tracker.doneCh)

	// Calling Delete should be non-blocking
	tracker.Delete()

	// After delete, MessageID should be 0 and closed true
	time.Sleep(50 * time.Millisecond)
	if tracker.MessageID() != 0 {
		t.Errorf("After Delete, MessageID() = %d, want 0", tracker.MessageID())
	}
}
