//go:build windows

package auth

import (
	"context"
	"errors"
)

// StartLogin on Windows returns an informational error as PTY is unix-specific
func (m *AuthManager) StartLogin(ctx context.Context, userID, chatID int64) (*LoginSession, error) {
	return nil, errors.New("login interaktif via Telegram belum didukung di Windows; silakan jalankan agy di terminal langsung")
}
