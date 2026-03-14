package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	if cfg.Providers.Active != "qwen" {
		t.Fatalf("Providers.Active = %q, want %q", cfg.Providers.Active, "qwen")
	}
	if entry, ok := cfg.Providers.List["qwen"]; !ok {
		t.Fatal("Providers.List missing 'qwen' entry")
	} else if entry.Model != "qwen3-max" {
		t.Fatalf("qwen provider Model = %q, want %q", entry.Model, "qwen3-max")
	}
	if !cfg.Heartbeat.Enabled {
		t.Fatalf("Heartbeat.Enabled = false, want true")
	}
	if cfg.Heartbeat.IntervalSeconds != 60 {
		t.Fatalf("Heartbeat.IntervalSeconds = %d, want %d", cfg.Heartbeat.IntervalSeconds, 60)
	}
}

func TestLoadConfig_OverridesHeartbeatFromFile(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	configPath := filepath.Join(workspace, "config.json")
	configJSON := `{
  "agents": {
    "workspace": ".tinybot/workspace",
    "max_tokens": 8192,
    "temperature": 0.7,
    "max_tool_iterations": 20
  },
  "providers": {
    "active": "qwen",
    "list": {
      "qwen": {
        "kind": "qwen",
        "api_key": "sk-your-openai-key-here",
        "api_base": "https://dashscope.aliyuncs.com/compatible-mode/v1",
        "model": "qwen3-max"
      }
    }
  },
  "heartbeat": {
    "enabled": true,
    "interval_seconds": 10
  }
}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0o644); err != nil {
		t.Fatalf("WriteFile(config.json) failed: %v", err)
	}

	cfg, err := LoadConfig(workspace)
	if err != nil {
		t.Fatalf("LoadConfig(%q) failed: %v", workspace, err)
	}

	if !cfg.Heartbeat.Enabled {
		t.Fatalf("Heartbeat.Enabled = false, want true")
	}
	if cfg.Heartbeat.IntervalSeconds != 10 {
		t.Fatalf("Heartbeat.IntervalSeconds = %d, want %d", cfg.Heartbeat.IntervalSeconds, 10)
	}
}
