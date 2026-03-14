package app

import (
	"fmt"
	"strings"

	"tinybot/internal/adapters/provider"
	"tinybot/internal/adapters/tool"
	workspaceadapter "tinybot/internal/adapters/workspace"
	"tinybot/internal/repository/sessionrepo"
	chatservice "tinybot/internal/service/chat"

	"github.com/joho/godotenv"
)

type App struct {
	ChatService *chatservice.Service
	SessionRepo chatservice.SessionRepository
	Config      *Config
	Tools       *tool.Registry
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

	sessionRepo := sessionrepo.NewFileSessionRepository(workspace)
	llm, err := newLLMClientFromConfig(cfg)
	if err != nil {
		return nil, err
	}

	// 初始化工具注册表，并注册工具
	toolRegistry := buildCoreToolRegistry(workspace, cfg)

	// 初始化会话整合器
	var consolidator *chatservice.Consolidator
	if cfg.Agents.Consolidation.Enabled {
		consolidator = chatservice.NewConsolidator(
			llm,
			cfg.Agents.Consolidation.TokenLimit,
			cfg.Agents.Consolidation.KeepRecent,
		)
	}
	chatService, err := chatservice.NewService(
		sessionRepo,
		llm,
		toolRegistry,
		newPromptBuilder(workspace),
		cfg.Agents.MaxToolIterations,
		cfg.Agents.MaxTokens,
		float32(cfg.Agents.Temperature),
		consolidator,
	)
	if err != nil {
		return nil, err
	}

	return &App{
		ChatService: chatService,
		SessionRepo: sessionRepo,
		Config:      cfg,
		Tools:       toolRegistry,
	}, nil
}

func newPromptBuilder(workspace string) *chatservice.Builder {
	return chatservice.NewPromptBuilder(
		workspace,
		workspaceadapter.NewBootstrapReader(workspace),
		workspaceadapter.NewMemoryStore(workspace),
		workspaceadapter.NewSkillsLoader(workspace, ""),
	)
}

func newLLMClientFromConfig(cfg *Config) (chatservice.CompletionClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("llm config is nil")
	}

	name, entry, err := cfg.ActiveProvider()
	if err != nil {
		return nil, fmt.Errorf("resolve provider: %w", err)

	}
	apiKey := strings.TrimSpace(entry.ApiKey)
	apiBase := strings.TrimSpace(entry.ApiBase)
	model := strings.TrimSpace(entry.Model)

	if apiKey == "" {
		return nil, fmt.Errorf("provider %q: api key is required (set in config or env)", name)
	}
	if apiBase == "" {
		return nil, fmt.Errorf("provider %q: api base is required", name)
	}

	registry := provider.DefaultRegistry()
	return registry.Create(
		entry.Kind,
		apiKey,
		apiBase,
		model,
	)
}

func buildCoreToolRegistry(workspace string, cfg *Config) *tool.Registry {
	reg := tool.NewRegistry()
	reg.Register(tool.NewExecTool(cfg.Tools.Exec.TimeoutSeconds, workspace))
	reg.Register(tool.NewReadFileTool(workspace))
	reg.Register(tool.NewWriteFileTool(workspace))
	reg.Register(tool.NewListDirTool(workspace))
	reg.Register(tool.NewEditFileTool(workspace))
	reg.Register(tool.NewWebSearchTool("", cfg.Tools.WebSearch.MaxResult))
	reg.Register(tool.NewWebFetchTool(cfg.Tools.WebFetch.MaxChars))
	return reg
}
