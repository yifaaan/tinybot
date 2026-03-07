package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFileTool_Spec(t *testing.T) {
	tool := NewReadFileTool(t.TempDir())
	spec := tool.Spec()

	if spec.Name != "read_file" {
		t.Fatalf("Name = %q, want %q", spec.Name, "read_file")
	}
	if spec.Description == "" {
		t.Fatal("Description should not be empty")
	}

	var schema map[string]any
	if err := json.Unmarshal(spec.Parameters, &schema); err != nil {
		t.Fatalf("Parameters is not valid JSON: %v", err)
	}
	if schema["type"] != "object" {
		t.Fatalf("schema type = %v, want %q", schema["type"], "object")
	}
	props, _ := schema["properties"].(map[string]any)
	if props == nil {
		t.Fatal("schema properties is nil")
	}
	if _, ok := props["path"]; !ok {
		t.Fatal("schema missing 'path' property")
	}
}

func TestReadFileTool_Execute(t *testing.T) {
	t.Run("missing path", func(t *testing.T) {
		tool := NewReadFileTool(t.TempDir())
		_, err := tool.Execute(context.Background(), map[string]any{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "path") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("empty string path", func(t *testing.T) {
		tool := NewReadFileTool(t.TempDir())
		_, err := tool.Execute(context.Background(), map[string]any{
			"path": "",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "path") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("whitespace-only path", func(t *testing.T) {
		tool := NewReadFileTool(t.TempDir())
		_, err := tool.Execute(context.Background(), map[string]any{
			"path": "   ",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "path") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("path is non-string type", func(t *testing.T) {
		tool := NewReadFileTool(t.TempDir())
		_, err := tool.Execute(context.Background(), map[string]any{
			"path": 12345,
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "path") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("file not found", func(t *testing.T) {
		tool := NewReadFileTool(t.TempDir())
		_, err := tool.Execute(context.Background(), map[string]any{
			"path": "missing.txt",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "read file tool") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("path is directory", func(t *testing.T) {
		workspace := t.TempDir()
		tool := NewReadFileTool(workspace)
		dirPath := filepath.Join(workspace, "docs")
		if err := os.MkdirAll(dirPath, 0o755); err != nil {
			t.Fatalf("mkdir docs: %v", err)
		}

		_, err := tool.Execute(context.Background(), map[string]any{
			"path": "docs",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "not a file") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("reads relative path from workspace", func(t *testing.T) {
		workspace := t.TempDir()
		tool := NewReadFileTool(workspace)

		filePath := filepath.Join(workspace, "notes.txt")
		if err := os.WriteFile(filePath, []byte("hello from tinybot"), 0o644); err != nil {
			t.Fatalf("write notes.txt: %v", err)
		}

		got, err := tool.Execute(context.Background(), map[string]any{
			"path": "notes.txt",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "hello from tinybot" {
			t.Fatalf("got = %q, want %q", got, "hello from tinybot")
		}
	})

	t.Run("reads nested relative path", func(t *testing.T) {
		workspace := t.TempDir()
		tool := NewReadFileTool(workspace)

		nested := filepath.Join(workspace, "a", "b")
		if err := os.MkdirAll(nested, 0o755); err != nil {
			t.Fatalf("mkdir a/b: %v", err)
		}
		filePath := filepath.Join(nested, "deep.txt")
		if err := os.WriteFile(filePath, []byte("deep content"), 0o644); err != nil {
			t.Fatalf("write deep.txt: %v", err)
		}

		got, err := tool.Execute(context.Background(), map[string]any{
			"path": filepath.Join("a", "b", "deep.txt"),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "deep content" {
			t.Fatalf("got = %q, want %q", got, "deep content")
		}
	})

	t.Run("reads absolute path", func(t *testing.T) {
		workspace := t.TempDir()
		tool := NewReadFileTool(workspace)

		filePath := filepath.Join(workspace, "nested", "todo.md")
		if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			t.Fatalf("mkdir nested: %v", err)
		}
		if err := os.WriteFile(filePath, []byte("todo item"), 0o644); err != nil {
			t.Fatalf("write todo.md: %v", err)
		}

		got, err := tool.Execute(context.Background(), map[string]any{
			"path": filePath,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "todo item" {
			t.Fatalf("got = %q, want %q", got, "todo item")
		}
	})

	t.Run("empty file returns empty string", func(t *testing.T) {
		workspace := t.TempDir()
		tool := NewReadFileTool(workspace)

		filePath := filepath.Join(workspace, "empty.txt")
		if err := os.WriteFile(filePath, []byte{}, 0o644); err != nil {
			t.Fatalf("write empty.txt: %v", err)
		}

		got, err := tool.Execute(context.Background(), map[string]any{
			"path": "empty.txt",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Fatalf("got = %q, want empty string", got)
		}
	})

	t.Run("preserves unicode content", func(t *testing.T) {
		workspace := t.TempDir()
		tool := NewReadFileTool(workspace)

		want := "你好世界\n🐈 nanobot\nこんにちは"
		filePath := filepath.Join(workspace, "unicode.txt")
		if err := os.WriteFile(filePath, []byte(want), 0o644); err != nil {
			t.Fatalf("write unicode.txt: %v", err)
		}

		got, err := tool.Execute(context.Background(), map[string]any{
			"path": "unicode.txt",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != want {
			t.Fatalf("got = %q, want %q", got, want)
		}
	})

	t.Run("preserves multiline and trailing newline", func(t *testing.T) {
		workspace := t.TempDir()
		tool := NewReadFileTool(workspace)

		want := "line1\nline2\nline3\n"
		filePath := filepath.Join(workspace, "multi.txt")
		if err := os.WriteFile(filePath, []byte(want), 0o644); err != nil {
			t.Fatalf("write multi.txt: %v", err)
		}

		got, err := tool.Execute(context.Background(), map[string]any{
			"path": "multi.txt",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != want {
			t.Fatalf("got = %q, want %q", got, want)
		}
	})

	t.Run("nonexistent nested relative path", func(t *testing.T) {
		tool := NewReadFileTool(t.TempDir())
		_, err := tool.Execute(context.Background(), map[string]any{
			"path": filepath.Join("no", "such", "file.txt"),
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "read file tool") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
