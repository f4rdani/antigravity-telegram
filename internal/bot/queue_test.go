package bot

import (
	"testing"
	"time"

	"agy-tele/config"
	"agy-tele/internal/session"
)

func newTestRouterForQueue(t *testing.T) *Router {
	t.Helper()
	cfg := config.DefaultConfig()
	sm, err := session.NewSessionManager("/tmp/test_session_queue.json", "/", "auto")
	if err != nil {
		t.Fatalf("Failed to create session manager: %v", err)
	}
	r := NewRouter(cfg, nil, sm)
	r.ensureQueueInit()
	return r
}

func TestQueueEnqueueDequeueFIFO(t *testing.T) {
	r := newTestRouterForQueue(t)
	userID := int64(999001)

	if got := r.queueLength(userID); got != 0 {
		t.Fatalf("expected empty queue, got %d", got)
	}

	pos1 := r.enqueue(userID, &QueuedItem{UserID: userID, ChatID: 1, Prompt: "first"})
	pos2 := r.enqueue(userID, &QueuedItem{UserID: userID, ChatID: 1, Prompt: "second"})
	pos3 := r.enqueue(userID, &QueuedItem{UserID: userID, ChatID: 1, Prompt: "third"})
	if pos1 != 1 || pos2 != 2 || pos3 != 3 {
		t.Fatalf("expected positions 1,2,3 got %d,%d,%d", pos1, pos2, pos3)
	}

	if got := r.queueLength(userID); got != 3 {
		t.Fatalf("expected queue length 3, got %d", got)
	}

	items := r.listQueue(userID)
	if len(items) != 3 || items[0].Prompt != "first" || items[2].Prompt != "third" {
		t.Fatalf("unexpected queue order: %+v", items)
	}

	first := r.dequeueNext(userID)
	if first == nil || first.Prompt != "first" {
		t.Fatalf("expected dequeue 'first', got %+v", first)
	}
	second := r.dequeueNext(userID)
	if second == nil || second.Prompt != "second" {
		t.Fatalf("expected dequeue 'second', got %+v", second)
	}

	if got := r.queueLength(userID); got != 1 {
		t.Fatalf("expected queue length 1 after 2 dequeues, got %d", got)
	}

	// Clear remaining
	if n := r.clearQueue(userID); n != 1 {
		t.Fatalf("expected clear to remove 1, got %d", n)
	}
	if got := r.queueLength(userID); got != 0 {
		t.Fatalf("expected empty after clear, got %d", got)
	}
	if next := r.dequeueNext(userID); next != nil {
		t.Fatalf("expected nil dequeue on empty, got %+v", next)
	}
}

func TestQueueMaxLimit(t *testing.T) {
	r := newTestRouterForQueue(t)
	userID := int64(999002)
	for i := 0; i < MaxQueuePerUser; i++ {
		if pos := r.enqueue(userID, &QueuedItem{UserID: userID, Prompt: "x"}); pos != i+1 {
			t.Fatalf("expected pos %d, got %d", i+1, pos)
		}
	}
	if pos := r.enqueue(userID, &QueuedItem{UserID: userID, Prompt: "overflow"}); pos != -1 {
		t.Fatalf("expected -1 when full, got %d", pos)
	}
}

func TestQueueRemoveAt(t *testing.T) {
	r := newTestRouterForQueue(t)
	userID := int64(999003)
	r.enqueue(userID, &QueuedItem{UserID: userID, Prompt: "a"})
	r.enqueue(userID, &QueuedItem{UserID: userID, Prompt: "b"})
	r.enqueue(userID, &QueuedItem{UserID: userID, Prompt: "c"})

	removed := r.removeQueueAt(userID, 2)
	if removed == nil || removed.Prompt != "b" {
		t.Fatalf("expected removed 'b', got %+v", removed)
	}
	items := r.listQueue(userID)
	if len(items) != 2 || items[0].Prompt != "a" || items[1].Prompt != "c" {
		t.Fatalf("unexpected order after remove: %+v", items)
	}
	if bad := r.removeQueueAt(userID, 9); bad != nil {
		t.Fatalf("expected nil for out-of-range, got %+v", bad)
	}
}

func TestIsQueueableAgentCommand(t *testing.T) {
	queueable := []string{
		"hello world",
		"/plan do something",
		"/goal achieve x",
		"/continue",
		"/customskill do y",
	}
	for _, q := range queueable {
		if !IsQueueableAgentCommand(q) {
			t.Errorf("expected queueable=true for %q", q)
		}
	}
	instant := []string{
		"/cancel", "/stop", "/status", "/queue", "/clearqueue",
		"/model", "/help", "/usage", "/cwd", "/new", "/resume",
	}
	for _, q := range instant {
		if IsQueueableAgentCommand(q) {
			t.Errorf("expected queueable=false for %q", q)
		}
	}
}

func TestFormatQueueListNoAmbiguity(t *testing.T) {
	r := newTestRouterForQueue(t)
	_ = r
	items := []*QueuedItem{
		{Prompt: "second message"},
		{Prompt: "third message"},
	}
	idOut := FormatQueueList("id", nil, items)
	if !stringContains(idOut, "Antrean") || !stringContains(idOut, "second message") {
		t.Fatalf("ID queue card missing expected content:\n%s", idOut)
	}
	enOut := FormatQueueList("en", nil, items)
	if !stringContains(enOut, "Queue") {
		t.Fatalf("EN queue card missing expected content:\n%s", enOut)
	}
	empty := FormatQueueList("id", nil, nil)
	if !stringContains(empty, "kosong") && !stringContains(empty, "Antrean kosong") {
		t.Fatalf("empty queue card should state empty explicitly:\n%s", empty)
	}
}

func TestIsInstantCommandIncludesQueue(t *testing.T) {
	if !IsInstantCommand("/queue") {
		t.Errorf("expected /queue to be instant")
	}
	if !IsInstantCommand("/clearqueue") {
		t.Errorf("expected /clearqueue to be instant")
	}
}

func TestBusyCardTextTransitions(t *testing.T) {
	task := &ActiveTask{UserID: 1, ChatID: 2, Prompt: "setup opencode zen", StartedAt: time.Now()}
	items := []*QueuedItem{{Prompt: "add ke gogate"}}

	// Queued state: ack header + active + pending + live footer, never frozen "harap tunggu".
	queued := buildBusyCardText("id", task, items, queuedHighlight("id", 1, 1, "add ke gogate"))
	for _, want := range []string{"antrean #1", "setup opencode zen", "add ke gogate", "🔄", "otomatis"} {
		if !stringContains(queued, want) {
			t.Errorf("queued card missing %q:\n%s", want, queued)
		}
	}
	for _, banned := range []string{"Harap tunggu", "harap tunggu", "please wait", "Please wait"} {
		if stringContains(queued, banned) {
			t.Errorf("queued card must not contain frozen waiting text %q:\n%s", banned, queued)
		}
	}

	// Running state: text flips to running, no longer "waiting".
	running := buildBusyCardText("id", &ActiveTask{UserID: 1, Prompt: "add ke gogate", StartedAt: time.Now()}, nil,
		runningHighlight("id", "add ke gogate", 0))
	for _, want := range []string{"dijalankan", "add ke gogate", "🔄"} {
		if !stringContains(running, want) {
			t.Errorf("running card missing %q:\n%s", want, running)
		}
	}

	// Done states.
	doneOK := busyDoneText("id", false)
	if !stringContains(doneOK, "selesai") {
		t.Errorf("done card missing 'selesai':\n%s", doneOK)
	}
	doneStop := busyDoneText("id", true)
	if !stringContains(doneStop, "dihentikan") {
		t.Errorf("stopped card missing 'dihentikan':\n%s", doneStop)
	}
	doneEn := busyDoneText("en", false)
	if !stringContains(doneEn, "finished") || !stringContains(doneEn, "Queue") {
		t.Errorf("en done card missing expected content:\n%s", doneEn)
	}
	queuedEn := buildBusyCardText("en", task, items, queuedHighlight("en", 2, 2, "second"))
	if !stringContains(queuedEn, "Queued as #2") || !stringContains(queuedEn, "🔄") {
		t.Errorf("en queued card missing expected content:\n%s", queuedEn)
	}
}
