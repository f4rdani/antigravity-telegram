package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type TelegramConfig struct {
	BotToken             string  `json:"bot_token"`
	AllowedUserIDs       []int64 `json:"allowed_user_ids"`
	StreamEditIntervalMs int     `json:"stream_edit_interval_ms"`
}

type AgyConfig struct {
	BinaryPath       string `json:"binary_path"`
	DefaultWorkspace string `json:"default_workspace"`
	PermissionMode   string `json:"permission_mode"` // "auto" or "ask"
	DefaultModel     string `json:"default_model"`
	DefaultEffort    string `json:"default_effort"`
	PrintTimeout     string `json:"print_timeout"` // e.g. "24h", prevents premature 5m0s timeout
}

type StorageConfig struct {
	SessionFile string `json:"session_file"`
}

type Config struct {
	Telegram TelegramConfig `json:"telegram"`
	Agy      AgyConfig      `json:"agy"`
	Storage  StorageConfig  `json:"storage"`
}

func DefaultRootWorkspace() string {
	if runtime.GOOS == "windows" {
		systemDrive := os.Getenv("SystemDrive")
		if systemDrive == "" {
			systemDrive = "C:"
		}
		return systemDrive + `\`
	}
	return "/"
}

func DefaultConfig() *Config {
	return &Config{
		Telegram: TelegramConfig{
			BotToken:             "",
			AllowedUserIDs:       []int64{},
			StreamEditIntervalMs: 1200,
		},
		Agy: AgyConfig{
			BinaryPath:       "agy",
			DefaultWorkspace: DefaultRootWorkspace(),
			PermissionMode:   "auto", // default auto-approve for hands-free remote usage
			DefaultModel:     "",
			DefaultEffort:    "",
			PrintTimeout:     "24h",
		},
		Storage: StorageConfig{
			SessionFile: "./sessions.json",
		},
	}
}

func LoadConfig(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	if configPath != "" {
		if data, err := os.ReadFile(configPath); err == nil {
			if err := json.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("failed to parse config JSON: %w", err)
			}
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to read config file %q: %w", configPath, err)
		}
	}

	// Environment variable overrides
	if token := os.Getenv("TELEGRAM_BOT_TOKEN"); token != "" {
		cfg.Telegram.BotToken = token
	}
	if allowed := os.Getenv("ALLOWED_USER_IDS"); allowed != "" {
		cfg.Telegram.AllowedUserIDs = nil
		parts := strings.Split(allowed, ",")
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if id, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
				cfg.Telegram.AllowedUserIDs = append(cfg.Telegram.AllowedUserIDs, id)
			}
		}
	}
	if agyBin := os.Getenv("AGY_BINARY_PATH"); agyBin != "" {
		cfg.Agy.BinaryPath = agyBin
	}
	if agyWs := os.Getenv("AGY_DEFAULT_WORKSPACE"); agyWs != "" {
		cfg.Agy.DefaultWorkspace = agyWs
	}
	if perm := os.Getenv("AGY_PERMISSION_MODE"); perm != "" {
		cfg.Agy.PermissionMode = perm
	}
	if timeout := os.Getenv("AGY_PRINT_TIMEOUT"); timeout != "" {
		cfg.Agy.PrintTimeout = timeout
	}
	if cfg.Agy.PrintTimeout == "" {
		cfg.Agy.PrintTimeout = "24h"
	}

	// Validate / normalize workspace
	if cfg.Agy.DefaultWorkspace == "" || cfg.Agy.DefaultWorkspace == "auto" {
		cfg.Agy.DefaultWorkspace = DefaultRootWorkspace()
	}
	cfg.Agy.DefaultWorkspace = filepath.Clean(cfg.Agy.DefaultWorkspace)

	// Resolve agy binary in PATH if using default "agy"
	if cfg.Agy.BinaryPath == "agy" || cfg.Agy.BinaryPath == "agy.exe" {
		if path, err := exec.LookPath(cfg.Agy.BinaryPath); err == nil {
			cfg.Agy.BinaryPath = path
		}
	}

	// Defaults fallback
	if cfg.Telegram.StreamEditIntervalMs <= 0 {
		cfg.Telegram.StreamEditIntervalMs = 1200
	}
	if cfg.Agy.PermissionMode != "ask" {
		cfg.Agy.PermissionMode = "auto"
	}
	if cfg.Storage.SessionFile == "" {
		cfg.Storage.SessionFile = "./sessions.json"
	}

	return cfg, nil
}

func (c *Config) IsUserAllowed(userID int64) bool {
	if len(c.Telegram.AllowedUserIDs) == 0 {
		// If empty, no user is permitted for security!
		return false
	}
	for _, id := range c.Telegram.AllowedUserIDs {
		if id == userID {
			return true
		}
	}
	return false
}
