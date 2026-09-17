package auth

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractAuthCode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Raw code",
			input:    "4/0AbCdEf123456",
			expected: "4/0AbCdEf123456",
		},
		{
			name:     "Code with /code command prefix",
			input:    "/code 4/0AbCdEf123456",
			expected: "4/0AbCdEf123456",
		},
		{
			name:     "Full callback URL with URL encoded code",
			input:    "https://antigravity.google/oauth-callback?code=4%2F0AbCdEf123456&state=xyz",
			expected: "4/0AbCdEf123456",
		},
		{
			name:     "Callback URL with raw code",
			input:    "https://antigravity.google/oauth-callback?state=xyz&code=4/0AbCdEf123456",
			expected: "4/0AbCdEf123456",
		},
		{
			name:     "Whitespace and newlines",
			input:    "  4/0AbCdEf123456 \n\n",
			expected: "4/0AbCdEf123456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractAuthCode(tt.input)
			if got != tt.expected {
				t.Errorf("ExtractAuthCode(%q) = %q; want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestAccountLifecycle(t *testing.T) {
	tempHome, err := os.MkdirTemp("", "agy-auth-test-*")
	if err != nil {
		t.Fatalf("failed to create temp home: %v", err)
	}
	defer os.RemoveAll(tempHome)

	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)
	os.Setenv("HOME", tempHome)

	// Ensure no active account initially
	if IsLoggedIn() {
		t.Errorf("expected not logged in initially")
	}

	// Create dummy token 1 (account A)
	// JWT header: {"alg":"none"} -> eyJhbGciOiJub25lIn0
	// JWT payload: {"email":"userA@gmail.com","name":"User A"} -> eyJlbWFpbCI6InVzZXJBQGdtYWlsLmNvbSIsIm5hbWUiOiJVc2VyIEEifQ
	tokenA := `{
		"token": {
			"access_token": "ya29.testA",
			"token_type": "Bearer",
			"refresh_token": "1//testA",
			"expiry": "2026-12-31T23:59:59Z"
		},
		"auth_method": "consumer",
		"id_token": "eyJhbGciOiJub25lIn0.eyJlbWFpbCI6InVzZXJBQGdtYWlsLmNvbSIsIm5hbWUiOiJVc2VyIEEifQ."
	}`

	tokenPath := GetTokenPath()
	if err := os.MkdirAll(filepath.Dir(tokenPath), 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(tokenPath, []byte(tokenA), 0600); err != nil {
		t.Fatalf("failed to write tokenA: %v", err)
	}

	if !IsLoggedIn() {
		t.Errorf("expected logged in after writing tokenA")
	}

	active, err := GetActiveAccount()
	if err != nil {
		t.Fatalf("GetActiveAccount failed: %v", err)
	}
	if active.Email != "userA@gmail.com" || active.Name != "User A" {
		t.Errorf("unexpected active account: %+v", active)
	}

	// Archive Account A
	saved, err := SaveCurrentAccount()
	if err != nil {
		t.Fatalf("SaveCurrentAccount failed: %v", err)
	}
	if saved.Email != "userA@gmail.com" {
		t.Errorf("expected saved email userA@gmail.com, got %s", saved.Email)
	}

	// Create Account B in accounts/
	tokenB := `{
		"token": {
			"access_token": "ya29.testB",
			"token_type": "Bearer",
			"refresh_token": "1//testB",
			"expiry": "2026-12-31T23:59:59Z"
		},
		"auth_method": "consumer",
		"id_token": "eyJhbGciOiJub25lIn0.eyJlbWFpbCI6InVzZXJCQGdtYWlsLmNvbSIsIm5hbWUiOiJVc2VyIEIifQ."
	}`
	destB := filepath.Join(GetAccountsDir(), "userB@gmail.com.json")
	if err := os.WriteFile(destB, []byte(tokenB), 0600); err != nil {
		t.Fatalf("failed to write tokenB: %v", err)
	}

	// List saved accounts
	accounts, err := ListSavedAccounts()
	if err != nil {
		t.Fatalf("ListSavedAccounts failed: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected 2 saved accounts, got %d", len(accounts))
	}
	// Active account A should be first
	if !accounts[0].IsActive || accounts[0].Email != "userA@gmail.com" {
		t.Errorf("expected active account A first, got %+v", accounts[0])
	}

	// Switch to Account B
	switched, err := SwitchAccount("userB@gmail.com")
	if err != nil {
		t.Fatalf("SwitchAccount failed: %v", err)
	}
	if switched.Email != "userB@gmail.com" || !switched.IsActive {
		t.Errorf("unexpected switched account: %+v", switched)
	}

	// Verify active account is now B
	activeNow, err := GetActiveAccount()
	if err != nil {
		t.Fatalf("GetActiveAccount after switch failed: %v", err)
	}
	if activeNow.Email != "userB@gmail.com" {
		t.Errorf("expected active account B, got %s", activeNow.Email)
	}

	// Test SignOut
	former, err := SignOut()
	if err != nil {
		t.Fatalf("SignOut failed: %v", err)
	}
	if former.Email != "userB@gmail.com" {
		t.Errorf("expected former account B, got %s", former.Email)
	}
	if IsLoggedIn() {
		t.Errorf("expected logged out after SignOut")
	}

	// Verify accounts still exist in saved list
	accountsAfter, err := ListSavedAccounts()
	if err != nil {
		t.Fatalf("ListSavedAccounts failed: %v", err)
	}
	if len(accountsAfter) != 2 {
		t.Fatalf("expected 2 accounts still in storage, got %d", len(accountsAfter))
	}

	// Test GetSavedAccount
	accB, err := GetSavedAccount("userB@gmail.com")
	if err != nil || accB.Email != "userB@gmail.com" {
		t.Errorf("expected to get saved account B, got %+v, err: %v", accB, err)
	}
	_, err = GetSavedAccount("nonexistent@gmail.com")
	if !errors.Is(err, ErrNoSavedAccount) {
		t.Errorf("expected ErrNoSavedAccount, got %v", err)
	}

	// Test Delete Account A
	if err := DeleteSavedAccount("userA@gmail.com"); err != nil {
		t.Fatalf("DeleteSavedAccount failed: %v", err)
	}
	accountsFinal, _ := ListSavedAccounts()
	if len(accountsFinal) != 1 || accountsFinal[0].Email != "userB@gmail.com" {
		t.Errorf("expected only account B left, got %v", accountsFinal)
	}
}
