package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"agy-tele/config"
	"agy-tele/internal/bot"
	"agy-tele/internal/session"
)

var (
	version = "1.0.16"
)

func main() {
	configPath := flag.String("config", "config.json", "Path to config JSON file")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("agy-tele version %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
		return
	}

	log.Printf("[agy-tele] Starting agy-tele v%s on %s/%s", version, runtime.GOOS, runtime.GOARCH)

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("[agy-tele] Configuration error: %v", err)
	}

	if cfg.Telegram.BotToken == "" {
		log.Fatalf("[agy-tele] Telegram bot_token is empty! Please set it in %s or via TELEGRAM_BOT_TOKEN environment variable.", *configPath)
	}

	if len(cfg.Telegram.AllowedUserIDs) == 0 {
		log.Printf("[agy-tele] WARNING: No allowed_user_ids configured! For security, all users will be blocked until allowed_user_ids is set.")
	} else {
		log.Printf("[agy-tele] Whitelisted %d user ID(s): %v", len(cfg.Telegram.AllowedUserIDs), cfg.Telegram.AllowedUserIDs)
	}

	log.Printf("[agy-tele] Antigravity CLI binary: %s", cfg.Agy.BinaryPath)
	log.Printf("[agy-tele] Default Workspace Root: %s", cfg.Agy.DefaultWorkspace)
	log.Printf("[agy-tele] Tool Permission Mode: %s", cfg.Agy.PermissionMode)

	// Initialize session manager
	sm, err := session.NewSessionManager(cfg.Storage.SessionFile, cfg.Agy.DefaultWorkspace, cfg.Agy.PermissionMode)
	if err != nil {
		log.Fatalf("[agy-tele] Failed to initialize session manager: %v", err)
	}

	// Create bot server
	server, err := bot.NewBotServer(cfg, sm)
	if err != nil {
		log.Fatalf("[agy-tele] Failed to initialize bot server: %v", err)
	}

	// Setup graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := server.Run(ctx); err != nil {
			log.Fatalf("[agy-tele] Server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Printf("[agy-tele] Shutting down gracefully...")
	if err := sm.SaveAll(); err != nil {
		log.Printf("[agy-tele] Error saving sessions: %v", err)
	}
	log.Printf("[agy-tele] Shutdown complete.")
}
