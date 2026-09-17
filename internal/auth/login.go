package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var (
	ErrLoginSessionExists = errors.New("sesi login sedang berjalan")
	ErrNoLoginSession     = errors.New("tidak ada sesi login yang sedang aktif")
	ErrLoginTimeout       = errors.New("waktu login habis (timeout)")
)

// LoginSession represents an in-flight Google OAuth interactive CLI session
type LoginSession struct {
	UserID      int64
	ChatID      int64
	AuthURL     string
	StartedAt   time.Time
	ExpiresAt   time.Time
	PromptMsgID int // Message ID of the login prompt in Telegram
	Cmd         *exec.Cmd
	PtyFile     *os.File
	Cancel      context.CancelFunc
	outputBuf   strings.Builder
	doneChan    chan struct{}
	mu          sync.Mutex
	closed      bool
}

type AuthManager struct {
	mu         sync.Mutex
	sessions   map[int64]*LoginSession
	binaryPath string
}

func NewAuthManager(binaryPath string) *AuthManager {
	return &AuthManager{
		sessions:   make(map[int64]*LoginSession),
		binaryPath: binaryPath,
	}
}

// HasActiveSession checks if a user currently has a valid, unexpired login session
func (m *AuthManager) HasActiveSession(userID int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[userID]
	if !ok || s == nil {
		return false
	}
	if time.Now().After(s.ExpiresAt) {
		go m.CancelLogin(userID)
		return false
	}
	return true
}

// GetSession returns the active login session for a user, if any
func (m *AuthManager) GetSession(userID int64) *LoginSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessions[userID]
}

// SetPromptMsgID associates the Telegram prompt message ID with the login session
func (m *AuthManager) SetPromptMsgID(userID int64, msgID int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[userID]; ok && s != nil {
		s.PromptMsgID = msgID
	}
}

// SubmitCode writes the user authorization code into the PTY and waits for token generation
func (m *AuthManager) SubmitCode(userID int64, rawInput string) (*AccountInfo, error) {
	m.mu.Lock()
	sess, exists := m.sessions[userID]
	m.mu.Unlock()

	if !exists || sess == nil {
		return nil, ErrNoLoginSession
	}

	code := ExtractAuthCode(rawInput)
	if code == "" {
		return nil, errors.New("kode otorisasi tidak boleh kosong")
	}

	sess.mu.Lock()
	if sess.closed {
		sess.mu.Unlock()
		return nil, errors.New("sesi login telah ditutup")
	}

	// Send code + newline to PTY
	_, writeErr := sess.PtyFile.Write([]byte(code + "\n"))
	sess.mu.Unlock()

	if writeErr != nil {
		m.CancelLogin(userID)
		return nil, fmt.Errorf("gagal mengirim kode ke proses auth: %w", writeErr)
	}

	// Wait up to 15 seconds for agy to exchange token with Google OAuth
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()

	select {
	case <-sess.doneChan:
		// Process finished
	case <-timer.C:
		m.CancelLogin(userID)
		return nil, errors.New("waktu verifikasi otorisasi habis (timeout 15 detik). Silakan coba lagi")
	}

	sess.mu.Lock()
	output := sess.outputBuf.String()
	sess.mu.Unlock()

	m.CancelLogin(userID)

	// Check if token file was created/updated on disk
	acc, err := GetActiveAccount()
	if err == nil && acc != nil && acc.Email != "" {
		// Success! Save to accounts/<email>.json
		_, _ = SaveCurrentAccount()
		return acc, nil
	}

	// Check for errors in output
	if strings.Contains(output, "Error:") || strings.Contains(output, "error:") {
		lines := strings.Split(output, "\n")
		var errLines []string
		for _, l := range lines {
			trimmed := strings.TrimSpace(l)
			lower := strings.ToLower(trimmed)
			if strings.Contains(lower, "error") || strings.Contains(lower, "failed") || strings.Contains(lower, "invalid_grant") {
				errLines = append(errLines, trimmed)
			}
		}
		errMsg := "Otorisasi gagal atau kode tidak valid."
		if len(errLines) > 0 {
			errMsg = strings.Join(errLines, "\n")
		}
		return nil, fmt.Errorf("%s", errMsg)
	}

	return nil, fmt.Errorf("login gagal: token tidak berhasil dibuat (%s)", strings.TrimSpace(output))
}

// CancelLogin terminates any active login session for the user
func (m *AuthManager) CancelLogin(userID int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	sess, exists := m.sessions[userID]
	if !exists || sess == nil {
		return false
	}
	delete(m.sessions, userID)

	sess.mu.Lock()
	defer sess.mu.Unlock()
	if sess.closed {
		return true
	}
	sess.closed = true

	if sess.Cancel != nil {
		sess.Cancel()
	}
	if sess.PtyFile != nil {
		_ = sess.PtyFile.Close()
	}
	if sess.Cmd != nil && sess.Cmd.Process != nil {
		_ = sess.Cmd.Process.Kill()
	}
	return true
}
