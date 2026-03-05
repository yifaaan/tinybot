package app

import (
	"tinybot/internal/adapters/provider"
	"tinybot/internal/adapters/repository"
	"tinybot/internal/ports"
	"tinybot/internal/usecase/chat"

	"github.com/joho/godotenv"
)

type App struct {
	ChatUseCase *chat.UseCase
	SessionRepo ports.SessionRepository
}

func NewApp(workspace string) (*App, error) {
	if err := godotenv.Load(); err != nil {
		return nil, err
	}

	sessionRepo := repository.NewFileSessionRepository(workspace)
	llm, err := provider.NewQwenClientFromEnv()
	if err != nil {
		return nil, err
	}
	chatUseCase, err := chat.NewUseCase(sessionRepo, llm, 20)
	if err != nil {
		return nil, err
	}

	return &App{
		ChatUseCase: chatUseCase,
		SessionRepo: sessionRepo,
	}, nil
}
