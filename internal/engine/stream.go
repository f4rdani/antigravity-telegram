package engine

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

type StreamCallbacks struct {
	OnInit       func(conversationID string, init *StreamInitPayload)
	OnDelta      func(delta string)
	OnStepUpdate func(step *StepUpdatePayload)
	OnResult     func(result *ResultPayload)
	OnError      func(err error)
}

type StreamAgentRunner struct {
	binaryPath string
	mu         sync.Mutex
	activeCmds map[int64]*exec.Cmd
}

func NewStreamAgentRunner(binaryPath string) *StreamAgentRunner {
	return &StreamAgentRunner{
		binaryPath: binaryPath,
		activeCmds: make(map[int64]*exec.Cmd),
	}
}

type StreamRunOptions struct {
	UserID         int64
	Prompt         string
	CWD            string
	ConversationID string
	PermissionMode string // "auto" or "ask"
	Mode           string // "plan", "accept-edits", or empty
	Model          string
	Effort         string
}

// CancelActive cancels any running command for the specified user
func (r *StreamAgentRunner) CancelActive(userID int64) bool {
	r.mu.Lock()
	cmd, ok := r.activeCmds[userID]
	r.mu.Unlock()

	if ok && cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		return true
	}
	return false
}

// RunStream spawns agy in stream-json mode and emits callbacks for all events
func (r *StreamAgentRunner) RunStream(ctx context.Context, opts StreamRunOptions, cb StreamCallbacks) (*ResultPayload, error) {
	var args []string
	args = append(args, "--input-format", "stream-json", "--output-format", "stream-json")

	if opts.PermissionMode == "auto" {
		args = append(args, "--dangerously-skip-permissions")
	}

	if opts.ConversationID != "" {
		args = append(args, "--conversation", opts.ConversationID)
	}

	if opts.Mode != "" {
		args = append(args, "--mode", opts.Mode)
	}

	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}

	if opts.Effort != "" {
		args = append(args, "--effort", opts.Effort)
	}

	cmd := exec.CommandContext(ctx, r.binaryPath, args...)
	cmd.Env = GetCommandEnv()
	if opts.CWD != "" {
		cmd.Dir = opts.CWD
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to open stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("failed to open stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, fmt.Errorf("failed to open stderr pipe: %w", err)
	}

	r.mu.Lock()
	r.activeCmds[opts.UserID] = cmd
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		delete(r.activeCmds, opts.UserID)
		r.mu.Unlock()
	}()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start agy process: %w", err)
	}

	// Capture stderr in background for diagnostic errors
	var stderrBuf strings.Builder
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				stderrBuf.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
	}()

	// Send input prompt event
	inputPayload := StreamInputMessage{
		Event: "user",
		Message: &StreamUserMessage{
			Content: opts.Prompt,
		},
	}
	inputBytes, err := json.Marshal(inputPayload)
	if err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("failed to serialize stream input: %w", err)
	}

	// Write newline-delimited JSON and close stdin so agy finishes the turn
	_, err = stdin.Write(append(inputBytes, '\n'))
	_ = stdin.Close()
	if err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("failed to write to stdin: %w", err)
	}

	// Read NDJSON stream from stdout using bufio.Reader (supports unlimited line sizes without ErrTooLong)
	reader := bufio.NewReaderSize(stdout, 64*1024)
	var finalResult *ResultPayload

	for {
		lineBytes, readErr := reader.ReadBytes('\n')
		if len(lineBytes) > 0 {
			line := strings.TrimSpace(string(lineBytes))
			if line != "" {
				var event StreamEvent
				if jsonErr := json.Unmarshal([]byte(line), &event); jsonErr == nil {
					switch event.Event {
					case "init":
						convID := event.ConversationID
						if cb.OnInit != nil {
							cb.OnInit(convID, event.Init)
						}

					case "step_update":
						if event.StepUpdate != nil {
							if event.StepUpdate.TextDelta != "" && cb.OnDelta != nil {
								cb.OnDelta(event.StepUpdate.TextDelta)
							}
							if cb.OnStepUpdate != nil {
								cb.OnStepUpdate(event.StepUpdate)
							}
						}

					case "result":
						if event.Result != nil {
							finalResult = event.Result
							if cb.OnResult != nil {
								cb.OnResult(event.Result)
							}
						}
					}
				}
			}
		}

		if readErr != nil {
			if readErr != io.EOF && cb.OnError != nil {
				cb.OnError(readErr)
			}
			break
		}
	}

	waitErr := cmd.Wait()
	if waitErr != nil && ctx.Err() == nil {
		errOutput := strings.TrimSpace(stderrBuf.String())
		if finalResult != nil && finalResult.Error != "" {
			return finalResult, nil
		}
		if errOutput != "" {
			return nil, fmt.Errorf("agy process error: %s", errOutput)
		}
		return nil, fmt.Errorf("agy process finished with error: %w", waitErr)
	}

	return finalResult, nil
}
