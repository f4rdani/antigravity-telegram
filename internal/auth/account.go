package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	ErrNotLoggedIn    = errors.New("tidak ada akun yang sedang aktif")
	ErrNoSavedAccount = errors.New("akun tidak ditemukan di daftar tersimpan")
)

// TokenFile matches the JSON structure stored in antigravity-oauth-token
type TokenFile struct {
	Token struct {
		AccessToken  string    `json:"access_token"`
		TokenType    string    `json:"token_type"`
		RefreshToken string    `json:"refresh_token"`
		Expiry       time.Time `json:"expiry"`
	} `json:"token"`
	AuthMethod string `json:"auth_method"`
	IDToken    string `json:"id_token"`
}

// IDTokenPayload matches the decoded JWT payload from id_token
type IDTokenPayload struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Exp           int64  `json:"exp"`
}

// AccountInfo holds public metadata about a Google Antigravity account
type AccountInfo struct {
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	Picture    string    `json:"picture"`
	Expiry     time.Time `json:"expiry"`
	AuthMethod string    `json:"auth_method"`
	IsActive   bool      `json:"is_active"`
	FilePath   string    `json:"file_path"`
	Tier       string    `json:"tier,omitempty"`
}

// GetTokenPath returns the path to the active antigravity-oauth-token
func GetTokenPath() string {
	home := os.Getenv("HOME")
	if home == "" {
		home = "/root"
	}
	return filepath.Join(home, ".gemini", "antigravity-cli", "antigravity-oauth-token")
}

// GetActiveTokenFile returns the parsed TokenFile from the active token path
func GetActiveTokenFile() (*TokenFile, error) {
	tokenPath := GetTokenPath()
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotLoggedIn
		}
		return nil, fmt.Errorf("gagal membaca active token: %w", err)
	}

	var tf TokenFile
	if err := json.Unmarshal(data, &tf); err != nil {
		return nil, fmt.Errorf("gagal unmarshal token JSON: %w", err)
	}
	return &tf, nil
}

// GetAccountsDir returns the directory where saved accounts are stored
func GetAccountsDir() string {
	home := os.Getenv("HOME")
	if home == "" {
		home = "/root"
	}
	return filepath.Join(home, ".gemini", "antigravity-cli", "accounts")
}

// ParseTokenPayload extracts AccountInfo from raw JSON token bytes
func ParseTokenPayload(tokenBytes []byte) (*AccountInfo, error) {
	if len(tokenBytes) == 0 {
		return nil, errors.New("token file kosong")
	}

	var tf TokenFile
	if err := json.Unmarshal(tokenBytes, &tf); err != nil {
		return nil, fmt.Errorf("gagal unmarshal token JSON: %w", err)
	}

	acc := &AccountInfo{
		Expiry:     tf.Token.Expiry,
		AuthMethod: tf.AuthMethod,
	}
	if acc.AuthMethod == "" {
		acc.AuthMethod = "consumer"
	}

	if tf.IDToken != "" {
		parts := strings.Split(tf.IDToken, ".")
		if len(parts) >= 2 {
			payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
			if err != nil {
				// Retry with standard URL encoding if raw failed
				payloadBytes, err = base64.URLEncoding.DecodeString(padBase64(parts[1]))
			}
			if err == nil {
				var payload IDTokenPayload
				if json.Unmarshal(payloadBytes, &payload) == nil {
					acc.Email = payload.Email
					acc.Name = payload.Name
					acc.Picture = payload.Picture
				}
			}
		}
	}

	if acc.Email == "" {
		acc.Email = "unknown-user"
	}
	if acc.Name == "" {
		acc.Name = acc.Email
	}

	return acc, nil
}

func padBase64(s string) string {
	switch len(s) % 4 {
	case 2:
		return s + "=="
	case 3:
		return s + "="
	default:
		return s
	}
}

// IsLoggedIn checks if an active token currently exists on disk
func IsLoggedIn() bool {
	info, err := GetActiveAccount()
	return err == nil && info != nil && info.Email != ""
}

// GetActiveAccount reads and returns metadata of the currently active account
func GetActiveAccount() (*AccountInfo, error) {
	tokenPath := GetTokenPath()
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotLoggedIn
		}
		return nil, fmt.Errorf("gagal membaca active token: %w", err)
	}

	acc, err := ParseTokenPayload(data)
	if err != nil {
		return nil, err
	}
	acc.IsActive = true
	acc.FilePath = tokenPath
	return acc, nil
}

// ListSavedAccounts returns all saved accounts in the accounts/ directory
func ListSavedAccounts() ([]AccountInfo, error) {
	dir := GetAccountsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	active, _ := GetActiveAccount()

	var accounts []AccountInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		acc, err := ParseTokenPayload(data)
		if err != nil {
			continue
		}
		acc.FilePath = path
		if active != nil && strings.EqualFold(acc.Email, active.Email) {
			acc.IsActive = true
		}
		accounts = append(accounts, *acc)
	}

	// Sort active first, then alphabetical by email
	sort.Slice(accounts, func(i, j int) bool {
		if accounts[i].IsActive != accounts[j].IsActive {
			return accounts[i].IsActive
		}
		return accounts[i].Email < accounts[j].Email
	})

	return accounts, nil
}

// SaveCurrentAccount archives the active token into accounts/<email>.json
func SaveCurrentAccount() (*AccountInfo, error) {
	tokenPath := GetTokenPath()
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return nil, err
	}

	acc, err := ParseTokenPayload(data)
	if err != nil {
		return nil, err
	}

	dir := GetAccountsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	destPath := filepath.Join(dir, acc.Email+".json")
	if err := os.WriteFile(destPath, data, 0600); err != nil {
		return nil, fmt.Errorf("gagal menyimpan akun ke %s: %w", destPath, err)
	}

	acc.IsActive = true
	acc.FilePath = destPath
	return acc, nil
}

// SwitchAccount replaces active token with a saved account's token
func SwitchAccount(email string) (*AccountInfo, error) {
	// First archive whatever is currently active
	_, _ = SaveCurrentAccount()

	dir := GetAccountsDir()
	srcPath := filepath.Join(dir, email+".json")
	data, err := os.ReadFile(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoSavedAccount
		}
		return nil, fmt.Errorf("gagal membaca file akun tersimpan: %w", err)
	}

	tokenPath := GetTokenPath()
	if err := os.MkdirAll(filepath.Dir(tokenPath), 0755); err != nil {
		return nil, err
	}

	if err := os.WriteFile(tokenPath, data, 0600); err != nil {
		return nil, fmt.Errorf("gagal mengaktifkan token baru: %w", err)
	}

	acc, err := ParseTokenPayload(data)
	if err != nil {
		return nil, err
	}
	acc.IsActive = true
	acc.FilePath = tokenPath
	InvalidateTierCache("")
	return acc, nil
}

// GetSavedAccount retrieves a saved account by email
func GetSavedAccount(email string) (*AccountInfo, error) {
	dir := GetAccountsDir()
	path := filepath.Join(dir, email+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoSavedAccount
		}
		return nil, fmt.Errorf("gagal membaca file akun tersimpan: %w", err)
	}
	acc, err := ParseTokenPayload(data)
	if err != nil {
		return nil, err
	}
	acc.FilePath = path
	active, _ := GetActiveAccount()
	if active != nil && strings.EqualFold(acc.Email, active.Email) {
		acc.IsActive = true
	}
	return acc, nil
}

// SignOut archives the active token to accounts/ and removes the active token file
func SignOut() (*AccountInfo, error) {
	active, err := GetActiveAccount()
	if err == nil && active != nil {
		// Save a backup before removing
		_, _ = SaveCurrentAccount()
	}

	tokenPath := GetTokenPath()
	if err := os.Remove(tokenPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("gagal menghapus token aktif: %w", err)
	}

	InvalidateTierCache("")
	return active, nil
}

// DeleteSavedAccount removes a saved account from accounts/
func DeleteSavedAccount(email string) error {
	dir := GetAccountsDir()
	filePath := filepath.Join(dir, email+".json")
	_ = os.Remove(filePath)

	// If deleting currently active account, also remove active token
	active, err := GetActiveAccount()
	if err == nil && active != nil && strings.EqualFold(active.Email, email) {
		_ = os.Remove(GetTokenPath())
	}
	InvalidateTierCache(email)
	return nil
}

// ExtractAuthCode parses an authorization code from either a raw code string or full redirect URL
func ExtractAuthCode(input string) string {
	s := strings.TrimSpace(input)
	if strings.HasPrefix(strings.ToLower(s), "/code") {
		s = strings.TrimSpace(s[5:])
	}

	// If user pasted full callback URL
	if strings.Contains(s, "code=") {
		if u, err := url.Parse(s); err == nil {
			if qCode := u.Query().Get("code"); qCode != "" {
				return qCode
			}
		}
		re := regexp.MustCompile(`code=([^&\s]+)`)
		if matches := re.FindStringSubmatch(s); len(matches) > 1 {
			if decoded, err := url.QueryUnescape(matches[1]); err == nil {
				return decoded
			}
			return matches[1]
		}
	}

	return s
}
