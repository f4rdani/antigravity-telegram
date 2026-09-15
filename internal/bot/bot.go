package bot

import (
	"context"
	"fmt"
	"log"

	"agy-tele/config"
	"agy-tele/internal/session"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type BotServer struct {
	cfg    *config.Config
	api    *tgbotapi.BotAPI
	sm     *session.SessionManager
	router *Router
}

func NewBotServer(cfg *config.Config, sm *session.SessionManager) (*BotServer, error) {
	if cfg.Telegram.BotToken == "" {
		return nil, fmt.Errorf("telegram bot_token is required")
	}

	bot, err := tgbotapi.NewBotAPI(cfg.Telegram.BotToken)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize telegram bot api: %w", err)
	}

	log.Printf("[agy-tele] Authorized on Telegram account: @%s (ID: %d)", bot.Self.UserName, bot.Self.ID)

	// Register slash commands with Telegram so typing '/' pops up the command menu!
	commands := []tgbotapi.BotCommand{
		{Command: "help", Description: "Panduan & dashboard interaktif"},
		{Command: "resume", Description: "Pilih dan lanjutkan sesi percakapan"},
		{Command: "plan", Description: "Jalankan mode perencana (planning)"},
		{Command: "goal", Description: "Jalankan autonomous goal"},
		{Command: "continue", Description: "Lanjut obrolan terakhir langsung"},
		{Command: "usage", Description: "Cek kuota limit 5 jam & mingguan"},
		{Command: "credits", Description: "Cek sisa kredit G1"},
		{Command: "model", Description: "Ganti model AI aktif"},
		{Command: "effort", Description: "Atur reasoning effort"},
		{Command: "skills", Description: "Daftar skills yang terpasang"},
		{Command: "status", Description: "Cek status daemon & workspace"},
		{Command: "cwd", Description: "Ubah direktori kerja (workspace)"},
		{Command: "ls", Description: "Tampilkan isi direktori server"},
		{Command: "pwd", Description: "Path direktori saat ini"},
		{Command: "new", Description: "Mulai sesi percakapan baru"},
		{Command: "permission", Description: "Atur mode persetujuan tools"},
		{Command: "cancel", Description: "Hentikan proses yang berjalan"},
		{Command: "file", Description: "Unduh file dari server ke Telegram"},
	}

	setCmds := tgbotapi.NewSetMyCommands(commands...)
	if _, err := bot.Request(setCmds); err != nil {
		log.Printf("[agy-tele] Warning: failed to register slash commands with Telegram: %v", err)
	} else {
		log.Printf("[agy-tele] Successfully registered %d slash commands with Telegram", len(commands))
	}

	router := NewRouter(cfg, bot, sm)

	return &BotServer{
		cfg:    cfg,
		api:    bot,
		sm:     sm,
		router: router,
	}, nil
}

func (s *BotServer) Run(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30

	updates := s.api.GetUpdatesChan(u)

	log.Printf("[agy-tele] Bot service started. Listening for updates...")

	for {
		select {
		case <-ctx.Done():
			log.Printf("[agy-tele] Shutting down bot updates listener...")
			s.api.StopReceivingUpdates()
			return nil

		case update, ok := <-updates:
			if !ok {
				return nil
			}
			go s.router.HandleUpdate(update)
		}
	}
}
