package artifact

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestArtifactScanDirs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agy_art_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a dummy artifact .md file
	mdPath := filepath.Join(tmpDir, "plan.md")
	mdContent := "# My Plan\nThis is a plan description for testing."
	if err := os.WriteFile(mdPath, []byte(mdContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create dummy metadata
	metaPath := filepath.Join(tmpDir, "plan.md.metadata.json")
	metaContent := `{"summary":"Test Plan Summary","requestFeedback":true,"userFacing":true}`
	if err := os.WriteFile(metaPath, []byte(metaContent), 0644); err != nil {
		t.Fatal(err)
	}

	items := scanDirs([]string{tmpDir})
	if len(items) != 1 {
		t.Fatalf("scanDirs() returned %d items, want 1", len(items))
	}

	item := items[0]
	if item.ID != "plan" {
		t.Errorf("item.ID = %q, want 'plan'", item.ID)
	}
	if item.Summary != "Test Plan Summary" {
		t.Errorf("item.Summary = %q, want 'Test Plan Summary'", item.Summary)
	}
	if !item.RequestFeedback {
		t.Errorf("item.RequestFeedback = false, want true")
	}
}

func TestFormatFriendlyTime(t *testing.T) {
	now := time.Now()
	if got := formatFriendlyTime(now); got != "Baru saja" {
		t.Errorf("formatFriendlyTime(now) = %q, want 'Baru saja'", got)
	}
}
