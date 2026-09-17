//go:build !windows

package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/creack/pty"
)

var authURLRegex = regexp.MustCompile(`https://accounts\.google\.com/o/oauth2/auth[^\s\r\n"']+`)

// StartLogin launches agy inside a PTY to trigger Google OAuth login and capture the URL
func (m *AuthManager) StartLogin(ctx context.Context, userID, chatID int64) (*LoginSession, error) {
	m.mu.Lock()
	if old, exists := m.sessions[userID]; exists && old != nil {
		m.mu.Unlock()
		m.CancelLogin(userID)
		m.mu.Lock()
	}

	ctx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	cmd := exec.CommandContext(ctx, m.binaryPath, "-p", "/help")

	home := os.Getenv("HOME")
	if home == "" {
		home = "/root"
	}
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"TERM=xterm-256color",
		"PATH="+os.Getenv("PATH"),
		"USER="+os.Getenv("USER"),
	)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		cancel()
		m.mu.Unlock()
		return nil, fmt.Errorf("gagal memulai PTY login: %w", err)
	}

	sess := &LoginSession{
		UserID:    userID,
		ChatID:    chatID,
		StartedAt: time.Now(),
		ExpiresAt: time.Now().Add(3 * time.Minute),
		Cmd:       cmd,
		PtyFile:   ptmx,
		Cancel:    cancel,
		doneChan:  make(chan struct{}),
	}
	m.sessions[userID] = sess
	m.mu.Unlock()

	urlFoundChan := make(chan string, 1)

	go func() {
		buf := make([]byte, 1024)
		var fullOutput strings.Builder

		for {
			n, readErr := ptmx.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				sess.mu.Lock()
				sess.outputBuf.WriteString(chunk)
				sess.mu.Unlock()

				fullOutput.WriteString(chunk)

				if sess.AuthURL == "" {
					if match := authURLRegex.FindString(fullOutput.String()); match != "" {
						sess.mu.Lock()
						sess.AuthURL = match
						sess.mu.Unlock()
						select {
						case urlFoundChan <- match:
						default:
						}
					}
				}
			}
			if readErr != nil {
				break
			}
		}
		close(sess.doneChan)
	}()

	// Wait up to 10 seconds for agy to output the Google OAuth URL
	select {
	case <-urlFoundChan:
		return sess, nil
	case <-time.After(10 * time.Second):
		sess.mu.Lock()
		outputSoFar := sess.outputBuf.String()
		sess.mu.Unlock()
		m.CancelLogin(userID)

		if strings.Contains(outputSoFar, "Commands available in print mode") {
			return nil, errors.New("sudah login ke akun aktif. Silakan jalankan /signout terlebih dahulu untuk berganti akun")
		}
		return nil, fmt.Errorf("timeout menunggu link login dari Antigravity: %s", outputSoFar)
	case <-ctx.Done():
		m.CancelLogin(userID)
		return nil, ctx.Err()
	}
}
