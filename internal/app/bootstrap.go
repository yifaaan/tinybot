package app

import (
	"tinybot/internal/adapters/provider"
	"tinybot/internal/adapters/repository"
	"tinybot/internal/adapters/tool"
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

	// 初始化工具注册表，并注册工具
	toolRegistry := tool.NewRegistry()
	// exec tool
	execTool := tool.NewExecTool(10, workspace)
	toolRegistry.Register(execTool)

	chatUseCase, err := chat.NewUseCase(sessionRepo, llm, toolRegistry, 20)
	if err != nil {
		return nil, err
	}
	chatUseCase.SetContextBuilder(chat.NewContextBuilder(workspace))

	return &App{
		ChatUseCase: chatUseCase,
		SessionRepo: sessionRepo,
	}, nil
}
