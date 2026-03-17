package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultWorkspacePath(t *testing.T) {
	originalGetwd := getwd
	originalExecutable := executable
	t.Cleanup(func() {
		getwd = originalGetwd
		executable = originalExecutable
	})

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "config.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("WriteFile(config.json) error: %v", err)
	}
	getwd = func() (string, error) { return filepath.Join(root, "build", "bin"), nil }
	executable = func() (string, error) { return filepath.Join(root, "build", "bin", "tinybot-desktop.exe"), nil }

	got := DefaultWorkspacePath()
	want := filepath.Join(root, "workspace")

	if got != want {
		t.Fatalf("DefaultWorkspacePath() = %q, want %q", got, want)
	}
}

func TestResolveWorkspacePath_UsesDefaultForEmptyInput(t *testing.T) {
	got := ResolveWorkspacePath("")
	want := DefaultWorkspacePath()

	if got != want {
		t.Fatalf("ResolveWorkspacePath(\"\") = %q, want %q", got, want)
	}
}

func TestResolveWorkspacePath_KeepsExplicitValue(t *testing.T) {
	got := ResolveWorkspacePath("custom/workspace")
	want := "custom/workspace"

	if got != want {
		t.Fatalf("ResolveWorkspacePath(explicit) = %q, want %q", got, want)
	}
}

func TestGetConfigPath_FindsProjectRootFromExecutable(t *testing.T) {
	originalGetwd := getwd
	originalExecutable := executable
	t.Cleanup(func() {
		getwd = originalGetwd
		executable = originalExecutable
	})

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "config.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("WriteFile(config.json) error: %v", err)
	}
	getwd = func() (string, error) { return filepath.Join(root, "build", "bin"), nil }
	executable = func() (string, error) { return filepath.Join(root, "build", "bin", "tinybot-desktop.exe"), nil }

	got := getConfigPath()
	want := filepath.Join(root, "config.json")
	if got != want {
		t.Fatalf("getConfigPath() = %q, want %q", got, want)
	}
}

func TestDefaultConfig_UsesDefaultWorkspacePath(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	if cfg.Agents.Workspace != DefaultWorkspacePath() {
		t.Fatalf("cfg.Agents.Workspace = %q, want %q", cfg.Agents.Workspace, DefaultWorkspacePath())
	}
}
