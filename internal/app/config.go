package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"tinybot/internal/utils"
	"tinybot/internal/utils/logger"
)

type Config struct {
	Agents    AgentsConfig    `json:"agents"`
	Channels  ChannelsConfig  `json:"channels,omitempty"`
	Providers ProvidersConfig `json:"providers"`
	Tools     ToolsConfig     `json:"tools,omitempty"`
	Heartbeat HeartbeatConfig `json:"heartbeat,omitempty"`
	Log       logger.Config   `json:"log,omitempty"`
}

type AgentsConfig struct {
	Workspace         string              `json:"workspace"`
	MaxTokens         int                 `json:"max_tokens"`
	Temperature       float64             `json:"temperature"`
	MaxToolIterations int                 `json:"max_tool_iterations"`
	Consolidation     ConsolidationConfig `json:"consolidation,omitempty"`
}

// ConsolidationConfig 控制会话历史压缩行为
type ConsolidationConfig struct {
	// Enabled 是否启用会话整合，默认 true
	Enabled bool `json:"enabled"`
	// TokenLimit 触发整合的 token 阈值，默认 60000
	TokenLimit int `json:"token_limit"`
	// KeepRecent 保留最近几条消息不压缩，默认 10
	KeepRecent int `json:"keep_recent"`
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

// ProvidersConfig 支持多个命名 provider，通过 Active 指定当前使用哪个。
//
// 配置文件示例：
//
//	{
//	  "providers": {
//	    "active": "qwen",           ← 当前生效的 provider 名称
//	    "list": {
//	      "qwen": {
//	        "kind": "openai",       ← 对应 registry 中注册的类型
//	        "api_key": "sk-xxx",
//	        "api_base": "https://dashscope.aliyuncs.com/compatible-mode/v1",
//	        "model": "qwen3-max"
//	      },
//	      "deepseek": {
//	        "kind": "deepseek",
//	        "api_key": "sk-yyy",
//	        "api_base": "https://api.deepseek.com",
//	        "model": "deepseek-chat"
//	      }
//	    }
//	  }
//	}
//
// 设计决策：
//   - model 放在 provider 级别而不是 agents 级别，
//     因为不同 provider 支持的模型名不同
//   - Active 是一个简单字符串，不是数组 —— 当前不支持"同时用多个 provider"
//   - list 用 map[string]ProviderEntry 而不是 []ProviderEntry，
//     这样可以通过名称直接查找，也方便环境变量覆盖
type ProvidersConfig struct {
	Active string                   `json:"active"`
	List   map[string]ProviderEntry `json:"list"`
}

type ProviderEntry struct {
	Kind    string `json:"kind"`
	Model   string `json:"model"`
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
			MaxTokens:         8192,
			Temperature:       0.7,
			MaxToolIterations: 20,
			Consolidation: ConsolidationConfig{
				Enabled:    true,
				TokenLimit: 60000,
				KeepRecent: 10,
			},
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
			Active: "qwen",
			List: map[string]ProviderEntry{
				"qwen": {
					Kind:    "qwen",
					ApiKey:  "",
					ApiBase: "https://dashscope.aliyuncs.com/compatible-mode/v1",
					Model:   "qwen3-max",
				},
				"openai": {
					Kind:    "openai",
					ApiKey:  "",
					ApiBase: "https://api.openai.com/v1",
					Model:   "gpt-4o-mini",
				},
				"ollama": {
					Kind:    "ollama",
					ApiKey:  "", // Ollama 不需要 API Key
					ApiBase: "http://localhost:11434/v1",
					Model:   "llama3.2",
				},
				"deepseek": {
					Kind:    "deepseek",
					ApiKey:  "",
					ApiBase: "https://api.deepseek.com",
					Model:   "deepseek-chat",
				},
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
		Log: logger.Config{
			Level:  "info",
			Format: "text",
			Output: "stderr",
		},
	}
}

// ApplyEnvOverrides overrides config fields with corresponding environment variables if they are set
func (cfg *Config) ApplyEnvOverrides() {
	if cfg == nil {
		return
	}
	if cfg.Providers.List == nil {
		cfg.Providers.List = make(map[string]ProviderEntry)
	}

	qwen := cfg.Providers.List["qwen"]
	if apiKey := os.Getenv("QWEN_API_KEY"); apiKey != "" {
		qwen.ApiKey = apiKey
	}
	if apiBase := os.Getenv("QWEN_API_BASE"); apiBase != "" {
		qwen.ApiBase = apiBase
	}
	if model := os.Getenv("QWEN_MODEL"); model != "" {
		qwen.Model = model
	}
	qwen.Kind = "qwen"
	cfg.Providers.List["qwen"] = qwen

	if token := os.Getenv("TELEGRAM_BOT_TOKEN"); token != "" {
		cfg.Channels.Telegram.Token = token
	}
}

func (c *Config) ActiveProvider() (string, ProviderEntry, error) {
	name := c.Providers.Active
	if name == "" {
		return "", ProviderEntry{}, fmt.Errorf("no active provider")
	}
	entry, ok := c.Providers.List[name]
	if !ok {
		return "", ProviderEntry{}, fmt.Errorf("active provider %s not found in config", name)
	}
	return name, entry, nil
}

func (cfg *Config) WorkspacePath() (string, error) {
	return utils.ExpandUser(cfg.Agents.Workspace)
}

func (cfg *Config) GetAPIKey(kind string) string {
	switch kind {
	case "qwen":
		return cfg.Providers.List[kind].ApiKey
	// TODO: support more providers
	default:
		return ""
	}
}

func (cfg *Config) GetAPIBase(kind string) string {
	switch kind {
	case "qwen":
		return cfg.Providers.List[kind].ApiBase
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
