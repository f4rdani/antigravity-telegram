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
