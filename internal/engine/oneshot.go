package engine

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type OneShotRunner struct {
	binaryPath string
}

func NewOneShotRunner(binaryPath string) *OneShotRunner {
	return &OneShotRunner{
		binaryPath: binaryPath,
	}
}

// Run executes a command in print mode: agy -p "<command>"
func (r *OneShotRunner) Run(ctx context.Context, cwd string, command string) (string, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctxTimeout, r.binaryPath, "-p", command)
	if cwd != "" {
		cmd.Dir = cwd
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	outStr := strings.TrimSpace(stdout.String())
	errStr := strings.TrimSpace(stderr.String())

	if err != nil {
		if ctxTimeout.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("command %q timed out after 45 seconds", command)
		}
		if errStr != "" {
			return "", fmt.Errorf("%s\n%s", err.Error(), errStr)
		}
		return "", fmt.Errorf("execution error: %w", err)
	}

	if outStr == "" && errStr != "" {
		return errStr, nil
	}

	return outStr, nil
}
