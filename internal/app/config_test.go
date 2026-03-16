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
	if cfg.Log.Level != "info" {
		t.Fatalf("Log.Level = %q, want %q", cfg.Log.Level, "info")
	}
	if cfg.Log.Format != "text" {
		t.Fatalf("Log.Format = %q, want %q", cfg.Log.Format, "text")
	}
	if cfg.Log.Output != "stderr" {
		t.Fatalf("Log.Output = %q, want %q", cfg.Log.Output, "stderr")
	}
}

func TestLoadConfig_OverridesHeartbeatFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	configJSON := `{
  "agents": {
    "workspace": "./workspace",
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

	t.Setenv("TINYBOT_CONFIG", configPath)

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	if !cfg.Heartbeat.Enabled {
		t.Fatalf("Heartbeat.Enabled = false, want true")
	}
	if cfg.Heartbeat.IntervalSeconds != 10 {
		t.Fatalf("Heartbeat.IntervalSeconds = %d, want %d", cfg.Heartbeat.IntervalSeconds, 10)
	}
}

func TestLoadConfig_OverridesLogFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	configJSON := `{
  "log": {
    "level": "debug",
    "format": "json",
    "output": "tinybot.log"
  }
}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0o644); err != nil {
		t.Fatalf("WriteFile(config.json) failed: %v", err)
	}

	t.Setenv("TINYBOT_CONFIG", configPath)

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	if cfg.Log.Level != "debug" {
		t.Fatalf("Log.Level = %q, want %q", cfg.Log.Level, "debug")
	}
	if cfg.Log.Format != "json" {
		t.Fatalf("Log.Format = %q, want %q", cfg.Log.Format, "json")
	}
	if cfg.Log.Output != "tinybot.log" {
		t.Fatalf("Log.Output = %q, want %q", cfg.Log.Output, "tinybot.log")
	}
}

func TestLoadConfig_PartialLogOverrideKeepsDefaultLevelAndFormat(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	configJSON := `{
  "log": {
    "output": "custom.log"
  }
}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0o644); err != nil {
		t.Fatalf("WriteFile(config.json) failed: %v", err)
	}

	t.Setenv("TINYBOT_CONFIG", configPath)

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// JSON 只覆盖 output 时，其它字段应继续沿用 DefaultConfig() 的默认值，
	// 避免用户为了改输出目标就必须把 level/format 全部重写一遍。
	if cfg.Log.Level != "info" {
		t.Fatalf("Log.Level = %q, want %q", cfg.Log.Level, "info")
	}
	if cfg.Log.Format != "text" {
		t.Fatalf("Log.Format = %q, want %q", cfg.Log.Format, "text")
	}
	if cfg.Log.Output != "custom.log" {
		t.Fatalf("Log.Output = %q, want %q", cfg.Log.Output, "custom.log")
	}
}

func TestSaveConfig_PreservesLogConfigRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	t.Setenv("TINYBOT_CONFIG", configPath)

	cfg := DefaultConfig()
	cfg.Log.Level = "debug"
	cfg.Log.Format = "json"
	cfg.Log.Output = "roundtrip.log"

	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig() error: %v", err)
	}

	loaded, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// SaveConfig 写出的 JSON 再被 LoadConfig 读回后，
	// logger 相关字段应保持一致，避免配置写盘过程中丢字段或被默认值覆盖。
	if loaded.Log.Level != "debug" {
		t.Fatalf("Log.Level = %q, want %q", loaded.Log.Level, "debug")
	}
	if loaded.Log.Format != "json" {
		t.Fatalf("Log.Format = %q, want %q", loaded.Log.Format, "json")
	}
	if loaded.Log.Output != "roundtrip.log" {
		t.Fatalf("Log.Output = %q, want %q", loaded.Log.Output, "roundtrip.log")
	}
}
