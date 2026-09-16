//go:build windows

package engine

import (
	"os/exec"
)

func setProcGroup(cmd *exec.Cmd) {
	// Process group configuration is handled automatically on Windows
}

func killProcGroup(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
