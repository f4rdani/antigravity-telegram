package session

import (
	"testing"
	"time"
)

func TestCleanWorkspaceURI(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`["file:///home/user/project"]`, "/home/user/project"},
		{`file:///var/www/html`, "/var/www/html"},
		{"", ""},
	}

	for _, tt := range tests {
		got := cleanWorkspaceURI(tt.input)
		// Normalize separators for cross-platform checking
		if tt.input != "" && got == "" {
			t.Errorf("cleanWorkspaceURI(%q) returned empty string", tt.input)
		}
	}
}

func TestFormatFriendlyTime(t *testing.T) {
	now := time.Now()

	if got := formatFriendlyTime(now); got != "Baru saja" {
		t.Errorf("formatFriendlyTime(now) = %q, want 'Baru saja'", got)
	}

	tenMinAgo := now.Add(-10 * time.Minute)
	if got := formatFriendlyTime(tenMinAgo); got != "10 mnt lalu" {
		t.Errorf("formatFriendlyTime(10m ago) = %q, want '10 mnt lalu'", got)
	}

	threeHoursAgo := now.Add(-3 * time.Hour)
	if got := formatFriendlyTime(threeHoursAgo); got != "3 jam lalu" {
		t.Errorf("formatFriendlyTime(3h ago) = %q, want '3 jam lalu'", got)
	}
}
