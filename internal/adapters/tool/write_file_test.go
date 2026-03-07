package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileTool_Spec(t *testing.T) {
	tool := NewWriteFileTool(t.TempDir())
	spec := tool.Spec()

	if spec.Name != "write_file" {
		t.Fatalf("Name = %q, want %q", spec.Name, "write_file")
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
	if _, ok := props["content"]; !ok {
		t.Fatal("schema missing 'content' property")
	}
}

func TestWriteFileTool_Execute(t *testing.T) {
	t.Run("missing path", func(t *testing.T) {
		tool := NewWriteFileTool(t.TempDir())
		_, err := tool.Execute(context.Background(), map[string]any{
			"content": "hello",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "path") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("missing content", func(t *testing.T) {
		tool := NewWriteFileTool(t.TempDir())
		_, err := tool.Execute(context.Background(), map[string]any{
			"path": "test.txt",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "content") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("empty string path", func(t *testing.T) {
		tool := NewWriteFileTool(t.TempDir())
		_, err := tool.Execute(context.Background(), map[string]any{
			"path":    "",
			"content": "hello",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "path") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("whitespace-only path", func(t *testing.T) {
		tool := NewWriteFileTool(t.TempDir())
		_, err := tool.Execute(context.Background(), map[string]any{
			"path":    "   ",
			"content": "hello",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "path") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("empty string content", func(t *testing.T) {
		tool := NewWriteFileTool(t.TempDir())
		_, err := tool.Execute(context.Background(), map[string]any{
			"path":    "test.txt",
			"content": "",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "content") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("whitespace-only content", func(t *testing.T) {
		tool := NewWriteFileTool(t.TempDir())
		_, err := tool.Execute(context.Background(), map[string]any{
			"path":    "test.txt",
			"content": "   ",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "content") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("writes relative path in workspace", func(t *testing.T) {
		workspace := t.TempDir()
		tool := NewWriteFileTool(workspace)

		result, err := tool.Execute(context.Background(), map[string]any{
			"path":    "notes.txt",
			"content": "hello from write_file",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(result, "Successfully write") {
			t.Fatalf("unexpected result message: %q", result)
		}

		filePath := filepath.Join(workspace, "notes.txt")
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("failed to read written file: %v", err)
		}
		if string(content) != "hello from write_file" {
			t.Fatalf("got = %q, want %q", string(content), "hello from write_file")
		}
	})

	t.Run("creates parent directories", func(t *testing.T) {
		workspace := t.TempDir()
		tool := NewWriteFileTool(workspace)

		// The nested directory doesn't exist yet
		relPath := filepath.Join("a", "b", "c", "deep.txt")
		_, err := tool.Execute(context.Background(), map[string]any{
			"path":    relPath,
			"content": "deep content",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		filePath := filepath.Join(workspace, relPath)
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("failed to read written file: %v", err)
		}
		if string(content) != "deep content" {
			t.Fatalf("got = %q, want %q", string(content), "deep content")
		}
	})

	t.Run("writes absolute path", func(t *testing.T) {
		workspace := t.TempDir()
		tool := NewWriteFileTool(workspace)

		// Create a separate temp dir for absolute path testing
		outDir := t.TempDir()
		absPath := filepath.Join(outDir, "abs.txt")

		_, err := tool.Execute(context.Background(), map[string]any{
			"path":    absPath,
			"content": "absolute content",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		content, err := os.ReadFile(absPath)
		if err != nil {
			t.Fatalf("failed to read written file: %v", err)
		}
		if string(content) != "absolute content" {
			t.Fatalf("got = %q, want %q", string(content), "absolute content")
		}
	})

	t.Run("overwrites existing file", func(t *testing.T) {
		workspace := t.TempDir()
		tool := NewWriteFileTool(workspace)

		filePath := filepath.Join(workspace, "overwrite.txt")
		if err := os.WriteFile(filePath, []byte("old content"), 0644); err != nil {
			t.Fatalf("failed to write original file: %v", err)
		}

		_, err := tool.Execute(context.Background(), map[string]any{
			"path":    "overwrite.txt",
			"content": "new content",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("failed to read written file: %v", err)
		}
		if string(content) != "new content" {
			t.Fatalf("got = %q, want %q", string(content), "new content")
		}
	})
}
