package renderer

import (
	"strings"
	"testing"
)

func TestShouldUseRich_Table(t *testing.T) {
	raw := "Ringkasan:\n\n| No | Komponen | Status |\n| --- | --- | --- |\n| 1 | CPU | Normal |\n"
	if !ShouldUseRich(raw) {
		t.Errorf("ShouldUseRich() = false for table input, want true")
	}
}

func TestShouldUseRich_TaskList(t *testing.T) {
	raw := "Tugas:\n\n- [x] Cek sistem selesai\n- [ ] Buat skrip health check\n"
	if !ShouldUseRich(raw) {
		t.Errorf("ShouldUseRich() = false for task list input, want true")
	}
}

func TestShouldUseRich_Heading(t *testing.T) {
	raw := "# Demo Format Rich Telegram\nHalo, ini demo.\n"
	if !ShouldUseRich(raw) {
		t.Errorf("ShouldUseRich() = false for heading input, want true")
	}
}

func TestShouldUseRich_PlainText(t *testing.T) {
	raw := "Halo, apa kabar hari ini?"
	if ShouldUseRich(raw) {
		t.Errorf("ShouldUseRich() = true for plain text, want false (legacy path is fine)")
	}
}

func TestShouldUseRich_Empty(t *testing.T) {
	if ShouldUseRich("") {
		t.Errorf("ShouldUseRich() = true for empty string, want false")
	}
}

func TestShouldUseRich_Oversize(t *testing.T) {
	raw := strings.Repeat("a", RichMaxChars+1)
	if ShouldUseRich(raw) {
		t.Errorf("ShouldUseRich() = true for oversized input, want false")
	}
}

func TestNormalizeForRich_Passthrough(t *testing.T) {
	raw := "# Judul\n\n| A | B |\n| --- | --- |\n| 1 | 2 |\n\n- [x] selesai\n"
	got := NormalizeForRich(raw)
	for _, want := range []string{"# Judul", "| A | B |", "- [x] selesai"} {
		if !strings.Contains(got, want) {
			t.Errorf("NormalizeForRich() lost %q, got:\n%s", want, got)
		}
	}
}

func TestNormalizeForRich_FileLink(t *testing.T) {
	raw := "Buka [`C:/project/app`](file:///C:/project/app) ya."
	got := NormalizeForRich(raw)
	if strings.Contains(got, "file:///") {
		t.Errorf("NormalizeForRich() still contains file link: %s", got)
	}
	if !strings.Contains(got, "`C:/project/app`") {
		t.Errorf("NormalizeForRich() should keep inline code path, got: %s", got)
	}
}

func TestSplitRichChunks_Small(t *testing.T) {
	chunks := SplitRichChunks("hello", 100)
	if len(chunks) != 1 || chunks[0] != "hello" {
		t.Errorf("SplitRichChunks() = %v, want [hello]", chunks)
	}
}

func TestSplitRichChunks_BlockBoundary(t *testing.T) {
	a := strings.Repeat("a", 60)
	b := strings.Repeat("b", 60)
	rich := a + "\n\n" + b
	chunks := SplitRichChunks(rich, 100)
	if len(chunks) != 2 {
		t.Fatalf("SplitRichChunks() = %d chunks, want 2", len(chunks))
	}
	if !strings.Contains(chunks[0], "a") || !strings.Contains(chunks[1], "b") {
		t.Errorf("SplitRichChunks() tore blocks apart: %v", chunks)
	}
}
