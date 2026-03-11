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
	Config      *Config
}

func NewApp(workspace string) (*App, error) {
	workspace = ResolveWorkspacePath(workspace)

	if err := godotenv.Load(); err != nil {
		return nil, err
	}

	cfg := DefaultConfig()
	cfg, err := LoadConfig(workspace)
	if err != nil {
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
	execTool := tool.NewExecTool(cfg.Tools.Exec.TimeoutSeconds, workspace)
	toolRegistry.Register(execTool)

	// read file tool
	readFileTool := tool.NewReadFileTool(workspace)
	toolRegistry.Register(readFileTool)

	// write file tool
	writeFileTool := tool.NewWriteFileTool(workspace)
	toolRegistry.Register(writeFileTool)

	// list dir tool
	listDirTool := tool.NewListDirTool(workspace)
	toolRegistry.Register(listDirTool)

	// edit file tool
	editFileTool := tool.NewEditFileTool(workspace)
	toolRegistry.Register(editFileTool)

	// web search tool
	webSearchTool := tool.NewWebSearchTool("", cfg.Tools.WebSearch.MaxResult) // API key should be set via environment variable
	toolRegistry.Register(webSearchTool)

	// web fetch tool
	webFetchTool := tool.NewWebFetchTool(cfg.Tools.WebFetch.MaxChars)
	toolRegistry.Register(webFetchTool)

	chatUseCase, err := chat.NewUseCase(sessionRepo, llm, toolRegistry, cfg.Agents.MaxToolIterations)
	if err != nil {
		return nil, err
	}
	chatUseCase.SetContextBuilder(chat.NewContextBuilder(workspace))

	return &App{
		ChatUseCase: chatUseCase,
		SessionRepo: sessionRepo,
		Config:      cfg,
	}, nil
}
