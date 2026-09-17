package bot

import (
	"strings"
	"sync"
	"testing"
	"time"

	"agy-tele/internal/session"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestJoinSplitMessages(t *testing.T) {
	// Case 1: Empty chunks
	if got := JoinSplitMessages(nil); got != "" {
		t.Errorf("expected empty string for nil chunks, got %q", got)
	}

	// Case 2: Single chunk
	if got := JoinSplitMessages([]string{"Hello World"}); got != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", got)
	}

	// Case 3: Telegram client split simulation (prev chunk >= 3800 chars, split in middle of code)
	part1 := strings.Repeat("A", 4000) + "fmt.Println(\"Hello "
	part2 := "World\")"
	joined := JoinSplitMessages([]string{part1, part2})
	expected := strings.Repeat("A", 4000) + "fmt.Println(\"Hello World\")"
	if joined != expected {
		t.Errorf("expected direct concatenation without newline, got diff: len %d vs %d", len(joined), len(expected))
	}

	// Case 4: Multiple small messages (sent as separate rapid messages)
	short1 := "First prompt line"
	short2 := "Second prompt line"
	joinedShort := JoinSplitMessages([]string{short1, short2})
	if joinedShort != "First prompt line\nSecond prompt line" {
		t.Errorf("expected newline separation for short chunks, got %q", joinedShort)
	}

	// Case 5: Short message already ending with newline
	withNL := "First line\n"
	joinedNL := JoinSplitMessages([]string{withNL, short2})
	if joinedNL != "First line\nSecond prompt line" {
		t.Errorf("expected no double newline, got %q", joinedNL)
	}
}

func TestIsInstantCommand(t *testing.T) {
	tests := []struct {
		text     string
		expected bool
	}{
		{"/cancel", true},
		{"/stop", true},
		{"/status", true},
		{"/help", true},
		{"/start", true},
		{"/quota", true},
		{"/model gemini-2.5-flash", true},
		{"/plan Do something", false},
		{"/goal Achieve target", false},
		{"Regular user prompt text", false},
		{"/cancel " + strings.Repeat("x", 1200), false},
	}

	for _, tt := range tests {
		got := IsInstantCommand(tt.text)
		if got != tt.expected {
			t.Errorf("IsInstantCommand(%q) = %v; want %v", tt.text, got, tt.expected)
		}
	}
}

func TestMessageAccumulator_DebounceAndMerge(t *testing.T) {
	acc := NewMessageAccumulator()
	sess := &session.UserSession{UserID: 12345, ChatID: 999}

	var mu sync.Mutex
	var flushedBatches []*AccumulatedBatch
	var flushedTexts []string
	flushedChan := make(chan struct{}, 1)

	callback := func(batch *AccumulatedBatch, combinedText string) {
		mu.Lock()
		flushedBatches = append(flushedBatches, batch)
		flushedTexts = append(flushedTexts, combinedText)
		mu.Unlock()
		select {
		case flushedChan <- struct{}{}:
		default:
		}
	}

	// Send chunk 1 (split part 1)
	part1 := strings.Repeat("X", 3950) + "function foo() {"
	msg1 := &tgbotapi.Message{
		MessageID: 101,
		From:      &tgbotapi.User{ID: 12345},
		Chat:      &tgbotapi.Chat{ID: 999},
		Text:      part1,
	}
	acc.Add(msg1, sess, callback)

	// Send chunk 2 50ms later (split part 2)
	time.Sleep(50 * time.Millisecond)
	part2 := "\n  return 42;\n}"
	msg2 := &tgbotapi.Message{
		MessageID: 102,
		From:      &tgbotapi.User{ID: 12345},
		Chat:      &tgbotapi.Chat{ID: 999},
		Text:      part2,
	}
	acc.Add(msg2, sess, callback)

	// Wait for accumulator timer to fire
	select {
	case <-flushedChan:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for accumulator to flush")
	}

	mu.Lock()
	defer mu.Unlock()

	if len(flushedBatches) != 1 {
		t.Fatalf("expected 1 flushed batch, got %d", len(flushedBatches))
	}

	batch := flushedBatches[0]
	if len(batch.Chunks) != 2 {
		t.Errorf("expected 2 chunks in batch, got %d", len(batch.Chunks))
	}
	if len(batch.MsgIDs) != 2 || batch.MsgIDs[0] != 101 || batch.MsgIDs[1] != 102 {
		t.Errorf("expected msg IDs [101, 102], got %v", batch.MsgIDs)
	}

	expectedCombined := part1 + part2
	if flushedTexts[0] != expectedCombined {
		t.Errorf("flushed text does not match expected combined text")
	}
}

func TestMessageAccumulator_Cancel(t *testing.T) {
	acc := NewMessageAccumulator()
	sess := &session.UserSession{UserID: 88888, ChatID: 777}

	called := false
	callback := func(batch *AccumulatedBatch, combinedText string) {
		called = true
	}

	msg := &tgbotapi.Message{
		MessageID: 201,
		From:      &tgbotapi.User{ID: 88888},
		Chat:      &tgbotapi.Chat{ID: 777},
		Text:      "Cancel me please",
	}
	acc.Add(msg, sess, callback)

	// Cancel before timer expires
	ok := acc.Cancel(88888)
	if !ok {
		t.Errorf("expected Cancel to return true")
	}

	time.Sleep(700 * time.Millisecond)
	if called {
		t.Errorf("callback was called despite being cancelled")
	}
}
