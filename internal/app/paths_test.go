package app

import (
	"path/filepath"
	"testing"
)

func TestDefaultWorkspacePath(t *testing.T) {
	got := DefaultWorkspacePath()
	want := filepath.Join(".tinybot", "workspace")

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

func TestDefaultConfig_UsesDefaultWorkspacePath(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	if cfg.Agents.Workspace != DefaultWorkspacePath() {
		t.Fatalf("cfg.Agents.Workspace = %q, want %q", cfg.Agents.Workspace, DefaultWorkspacePath())
	}
}
