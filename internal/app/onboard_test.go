package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOnBoard_CreatesMinimumWorkspace(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace")

	result, err := OnBoard(context.Background(), workspace)
	if err != nil {
		t.Fatalf("OnBoard() error: %v", err)
	}
	if result == nil {
		t.Fatal("OnBoard() returned nil result")
	}
	if result.Workspace != workspace {
		t.Fatalf("result.Workspace = %q, want %q", result.Workspace, workspace)
	}

	expectedFiles := []string{
		"AGENTS.md",
		"SOUL.md",
		"USER.md",
		"TOOLS.md",
		filepath.Join("memory", "MEMORY.md"),
		"config.json",
	}

	for _, rel := range expectedFiles {
		if !containsString(result.CreatedFiles, rel) {
			t.Fatalf("CreatedFiles missing %q: %#v", rel, result.CreatedFiles)
		}
		if !fileExists(filepath.Join(workspace, rel)) {
			t.Fatalf("expected file to exist: %s", filepath.Join(workspace, rel))
		}
	}

	if !dirExists(filepath.Join(workspace, "skills")) {
		t.Fatalf("expected skills dir to exist: %s", filepath.Join(workspace, "skills"))
	}

	status := CheckStatus(workspace)
	if !status.WorkspaceExists {
		t.Fatal("status.WorkspaceExists = false, want true")
	}
	if !status.ConfigExists {
		t.Fatal("status.ConfigExists = false, want true")
	}
	if !status.MemoryFileExists {
		t.Fatal("status.MemoryFileExists = false, want true")
	}
	if !status.SkillsDirExists {
		t.Fatal("status.SkillsDirExists = false, want true")
	}
	if len(status.MissingFiles) != 0 {
		t.Fatalf("status.MissingFiles = %#v, want empty", status.MissingFiles)
	}
}

func TestOnBoard_DoesNotOverwriteExistingFiles(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace")

	if _, err := OnBoard(context.Background(), workspace); err != nil {
		t.Fatalf("first OnBoard() error: %v", err)
	}

	agentsPath := filepath.Join(workspace, "AGENTS.md")
	customContent := "# custom agents\n"
	if err := os.WriteFile(agentsPath, []byte(customContent), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error: %v", agentsPath, err)
	}

	result, err := OnBoard(context.Background(), workspace)
	if err != nil {
		t.Fatalf("second OnBoard() error: %v", err)
	}
	if len(result.CreatedFiles) != 0 {
		t.Fatalf("len(result.CreatedFiles) = %d, want 0, got %#v", len(result.CreatedFiles), result.CreatedFiles)
	}

	data, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error: %v", agentsPath, err)
	}
	if string(data) != customContent {
		t.Fatalf("AGENTS.md content = %q, want %q", string(data), customContent)
	}
}

func TestCheckStatus_ReportsMissingFiles(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(filepath.Join(workspace, "memory"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "AGENTS.md"), []byte("# agents\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	status := CheckStatus(workspace)
	if !status.WorkspaceExists {
		t.Fatal("status.WorkspaceExists = false, want true")
	}
	if status.ConfigExists {
		t.Fatal("status.ConfigExists = true, want false")
	}
	if status.MemoryFileExists {
		t.Fatal("status.MemoryFileExists = true, want false")
	}
	if status.SkillsDirExists {
		t.Fatal("status.SkillsDirExists = true, want false")
	}

	wantMissing := []string{
		"SOUL.md",
		"USER.md",
		"TOOLS.md",
		"config.json",
		filepath.Join("memory", "MEMORY.md"),
	}

	for _, name := range wantMissing {
		if !containsString(status.MissingFiles, name) {
			t.Fatalf("MissingFiles missing %q: %#v", name, status.MissingFiles)
		}
	}

	if !status.BootstrapFiles["AGENTS.md"] {
		t.Fatal("status.BootstrapFiles[AGENTS.md] = false, want true")
	}
	if status.BootstrapFiles["SOUL.md"] {
		t.Fatal("status.BootstrapFiles[SOUL.md] = true, want false")
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
