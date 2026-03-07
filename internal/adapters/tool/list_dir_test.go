package tool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListDirTool_Execute(t *testing.T) {
	t.Run("missing path", func(t *testing.T) {
		tool := NewListDirTool(t.TempDir())
		_, err := tool.Execute(context.Background(), map[string]any{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "path") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("directory not found", func(t *testing.T) {
		tool := NewListDirTool(t.TempDir())
		_, err := tool.Execute(context.Background(), map[string]any{
			"path": "missing-dir",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "list dir tool") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("path is file", func(t *testing.T) {
		workspace := t.TempDir()
		tool := NewListDirTool(workspace)

		filePath := filepath.Join(workspace, "notes.txt")
		if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
			t.Fatalf("write notes.txt: %v", err)
		}

		_, err := tool.Execute(context.Background(), map[string]any{
			"path": "notes.txt",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "not a directory") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("lists relative directory contents in sorted order", func(t *testing.T) {
		workspace := t.TempDir()
		tool := NewListDirTool(workspace)

		if err := os.MkdirAll(filepath.Join(workspace, "workspace"), 0o755); err != nil {
			t.Fatalf("mkdir workspace: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(workspace, "workspace", "beta"), 0o755); err != nil {
			t.Fatalf("mkdir beta: %v", err)
		}
		if err := os.WriteFile(filepath.Join(workspace, "workspace", "alpha.txt"), []byte("a"), 0o644); err != nil {
			t.Fatalf("write alpha.txt: %v", err)
		}
		if err := os.WriteFile(filepath.Join(workspace, "workspace", "zeta.md"), []byte("z"), 0o644); err != nil {
			t.Fatalf("write zeta.md: %v", err)
		}

		got, err := tool.Execute(context.Background(), map[string]any{
			"path": "workspace",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		lines := strings.Split(strings.TrimSpace(got), "\n")
		want := []string{
			"FILE alpha.txt",
			"DIR  beta",
			"FILE zeta.md",
		}
		if len(lines) != len(want) {
			t.Fatalf("lines len = %d, want %d; output = %q", len(lines), len(want), got)
		}
		for i := range want {
			if lines[i] != want[i] {
				t.Fatalf("line %d = %q, want %q; output = %q", i, lines[i], want[i], got)
			}
		}
	})

	t.Run("lists absolute directory path", func(t *testing.T) {
		workspace := t.TempDir()
		tool := NewListDirTool(workspace)

		dirPath := filepath.Join(workspace, "docs")
		if err := os.MkdirAll(dirPath, 0o755); err != nil {
			t.Fatalf("mkdir docs: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dirPath, "guide.md"), []byte("guide"), 0o644); err != nil {
			t.Fatalf("write guide.md: %v", err)
		}

		got, err := tool.Execute(context.Background(), map[string]any{
			"path": dirPath,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(got, "FILE guide.md") {
			t.Fatalf("unexpected output: %q", got)
		}
	})
}
