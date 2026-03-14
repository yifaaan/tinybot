package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"tinybot/internal/utils"
)

type Config struct {
	Agents    AgentsConfig    `json:"agents"`
	Channels  ChannelsConfig  `json:"channels,omitempty"`
	Providers ProvidersConfig `json:"providers"`
	Tools     ToolsConfig     `json:"tools,omitempty"`
	Heartbeat HeartbeatConfig `json:"heartbeat,omitempty"`
}

type AgentsConfig struct {
	Workspace         string  `json:"workspace"`
	Model             string  `json:"model"`
	MaxTokens         int     `json:"max_tokens"`
	Temperature       float64 `json:"temperature"`
	MaxToolIterations int     `json:"max_tool_iterations"`
}

type ChannelsConfig struct {
	// TODO: support multiple channels
	Console  ConsoleChannelConfig  `json:"console,omitempty"`
	Telegram TelegramChannelConfig `json:"telegram,omitempty"`
}

type ConsoleChannelConfig struct {
	Enabled    bool   `json:"enabled"`
	Prompt     string `json:"prompt,omitempty"`
	ChatID     string `json:"chat_id,omitempty"`
	SenderID   string `json:"sender_id,omitempty"`
	ShowPrefix bool   `json:"show_prefix,omitempty"`
}

type TelegramChannelConfig struct {
	Enabled bool   `json:"enabled"`
	Token   string `json:"token"`
}

type ProvidersConfig struct {
	// TODO: support multiple providers
	QWen ProviderConfig `json:"qwen"`
}

type ProviderConfig struct {
	ApiKey  string `json:"api_key"`
	ApiBase string `json:"api_base"`
}

type HeartbeatConfig struct {
	Enabled         bool `json:"enabled"`
	IntervalSeconds int  `json:"interval_seconds"`
}

type ToolsConfig struct {
	// TODO: support multiple tools
	WebSearch WebSearchConfig `json:"web_search,omitempty"`
	WebFetch  WebFetchConfig  `json:"web_fetch,omitempty"`
	Exec      ExecConfig      `json:"exec,omitempty"`
}

type WebSearchConfig struct {
	MaxResult int `json:"max_result"`
}

type WebFetchConfig struct {
	MaxChars int `json:"max_chars"`
}

type ExecConfig struct {
	TimeoutSeconds int `json:"timeout_seconds"`
}

func DefaultConfig() *Config {
	return &Config{
		Agents: AgentsConfig{
			Workspace:         DefaultWorkspacePath(),
			Model:             "qwen3-max",
			MaxTokens:         8192,
			Temperature:       0.7,
			MaxToolIterations: 20,
		},
		Channels: ChannelsConfig{
			Console: ConsoleChannelConfig{
				Enabled:    true,
				Prompt:     "You>",
				ChatID:     "gateway",
				SenderID:   "console-user",
				ShowPrefix: true,
			},
		},
		Providers: ProvidersConfig{
			QWen: ProviderConfig{
				ApiKey:  "",
				ApiBase: "https://dashscope.aliyuncs.com/compatible-mode/v1",
			},
		},
		Tools: ToolsConfig{
			WebSearch: WebSearchConfig{MaxResult: 10},
			WebFetch:  WebFetchConfig{MaxChars: 50000},
			Exec:      ExecConfig{TimeoutSeconds: 10},
		},
		Heartbeat: HeartbeatConfig{
			Enabled:         true,
			IntervalSeconds: 60,
		},
	}
}

// ApplyEnvOverrides overrides config fields with corresponding environment variables if they are set
func (cfg *Config) ApplyEnvOverrides() {
	if cfg == nil {
		return
	}
	if apiKey := os.Getenv("QWEN_API_KEY"); apiKey != "" {
		cfg.Providers.QWen.ApiKey = apiKey
	}
	if apiBase := os.Getenv("QWEN_API_BASE"); apiBase != "" {
		cfg.Providers.QWen.ApiBase = apiBase
	}
	if model := os.Getenv("QWEN_MODEL"); model != "" {
		cfg.Agents.Model = model
	}
	if token := os.Getenv("TELEGRAM_BOT_TOKEN"); token != "" {
		cfg.Channels.Telegram.Token = token
	}
}

func (cfg *Config) WorkspacePath() (string, error) {
	return utils.ExpandUser(cfg.Agents.Workspace)
}

func (cfg *Config) GetAPIKey(providerName string) string {
	switch providerName {
	case "qwen":
		return cfg.Providers.QWen.ApiKey
	// TODO: support more providers
	default:
		return ""
	}
}

func (cfg *Config) GetAPIBase(providerName string) string {
	switch providerName {
	case "qwen":
		return cfg.Providers.QWen.ApiBase
	// TODO: support more providers
	default:
		return ""
	}
}

func LoadConfig(workspace string) (*Config, error) {
	workspace = ResolveWorkspacePath(workspace)
	cfg := DefaultConfig()

	cfgPath := getConfigPath(workspace)
	_, err := os.Stat(cfgPath)
	if err != nil {
		cfg.ApplyEnvOverrides()
		return cfg, nil // return default config if config file does not exist
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	cfg.ApplyEnvOverrides()

	return cfg, nil
}

func SaveConfig(cfg *Config, workspace string) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	workspace = ResolveWorkspacePath(workspace)

	cfgPath := getConfigPath(workspace)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	parent, err := utils.ExpandUser(workspace)
	if err != nil {
		return fmt.Errorf("failed to expand workspace path: %w", err)
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
	}
	return os.WriteFile(cfgPath, data, 0o644)
}

func getConfigPath(workspace string) string {
	workspace = ResolveWorkspacePath(workspace)
	return filepath.Join(workspace, "config.json")
}
