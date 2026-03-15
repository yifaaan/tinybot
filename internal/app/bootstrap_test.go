package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tinybot/internal/utils/logger"
)

func stubBootstrapDeps(t *testing.T, loggerInit func(cfg logger.Config) error) {
	t.Helper()

	oldLoadDotEnv := loadDotEnv
	oldInitLogger := initLogger

	loadDotEnv = func(filenames ...string) error {
		return nil
	}
	initLogger = loggerInit

	t.Cleanup(func() {
		loadDotEnv = oldLoadDotEnv
		initLogger = oldInitLogger
	})
}

func TestBuildCoreToolRegistry_DoesNotRegisterMessageTool(t *testing.T) {
	cfg := DefaultConfig()
	reg := buildCoreToolRegistry(t.TempDir(), cfg)

	if reg.Has("message") {
		t.Fatal("buildCoreToolRegistry() should not register message tool")
	}
}

func TestBuildCoreToolRegistry_DefinitionsDoNotExposeMessageTool(t *testing.T) {
	cfg := DefaultConfig()
	reg := buildCoreToolRegistry(t.TempDir(), cfg)

	definitions := reg.GetDefinitions()
	for _, definition := range definitions {
		function, ok := definition["function"].(map[string]any)
		if !ok {
			continue
		}
		name, _ := function["name"].(string)
		if name == "message" {
			t.Fatal("buildCoreToolRegistry() should not expose message tool definitions")
		}
	}
}

func TestNewApp_InvalidLogOutputReturnsInitializeLoggerError(t *testing.T) {
	stubBootstrapDeps(t, logger.Init)

	workspace := t.TempDir()
	configPath := filepath.Join(workspace, "config.json")
	invalidLogOutput := filepath.Join(workspace, "missing", "logs", "tinybot.log")
	configJSON := fmt.Sprintf(`{
  "log": {
    "output": %q
  }
}`, invalidLogOutput)

	if err := os.WriteFile(configPath, []byte(configJSON), 0o644); err != nil {
		t.Fatalf("WriteFile(config.json) failed: %v", err)
	}

	_, err := NewApp(workspace)
	if err == nil {
		t.Fatal("expected NewApp() error, got nil")
	}
	if !strings.Contains(err.Error(), "initialize logger") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewApp_StdoutLogOutputInitializesSuccessfully(t *testing.T) {
	var gotLoggerCfg logger.Config
	stubBootstrapDeps(t, func(cfg logger.Config) error {
		gotLoggerCfg = cfg
		return nil
	})

	workspace := t.TempDir()
	configPath := filepath.Join(workspace, "config.json")
	configJSON := fmt.Sprintf(`{
  "providers": {
    "active": "qwen",
    "list": {
      "qwen": {
        "kind": "qwen",
        "api_key": "test-key",
        "api_base": "https://dashscope.aliyuncs.com/compatible-mode/v1",
        "model": "qwen3-max"
      }
    }
  },
  "log": {
    "level": "info",
    "format": "text",
    "output": "stdout"
  }
}`)

	if err := os.WriteFile(configPath, []byte(configJSON), 0o644); err != nil {
		t.Fatalf("WriteFile(config.json) failed: %v", err)
	}

	app, err := NewApp(workspace)
	if err != nil {
		t.Fatalf("NewApp() error: %v", err)
	}
	if app == nil {
		t.Fatal("NewApp() returned nil app")
	}
	if app.Config == nil {
		t.Fatal("app.Config is nil")
	}
	if app.Config.Log.Output != "stdout" {
		t.Fatalf("Log.Output = %q, want %q", app.Config.Log.Output, "stdout")
	}
	if gotLoggerCfg.Output != "stdout" {
		t.Fatalf("logger init output = %q, want %q", gotLoggerCfg.Output, "stdout")
	}
}
