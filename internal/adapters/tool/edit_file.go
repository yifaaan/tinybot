package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"tinybot/internal/ports"
)

type EditFileTool struct {
	workspace string
}

func NewEditFileTool(workspace string) *EditFileTool {
	return &EditFileTool{workspace: workspace}
}

func (t *EditFileTool) Spec() ports.ToolSpec {
	return ports.ToolSpec{
		Name:        "edit_file",
		Description: "Edit a file and return its content.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "The path to the file to edit"
				},
				"old_text": {
					"type": "string",
					"description": "The old text to replace"
				},
				"new_text": {
					"type": "string",
					"description": "The new text to replace with"
				}
			},
			"required": ["path", "old_text", "new_text"]
		}`),
	}
}

func (t *EditFileTool) Execute(ctx context.Context, params map[string]any) (string, error) {
	path, _ := params["path"].(string)
	oldText, _ := params["old_text"].(string)
	newText, _ := params["new_text"].(string)
	if strings.TrimSpace(path) == "" {
		return "", errors.New("edit file tool: `path` is required")
	}
	if strings.TrimSpace(oldText) == "" {
		return "", errors.New("edit file tool: `old_text` is required")
	}
	if strings.TrimSpace(newText) == "" {
		return "", errors.New("edit file tool: `new_text` is required")
	}
	path, err := ExpandUser(path)
	if err != nil {
		return "", fmt.Errorf("edit file tool: %w", err)
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(t.workspace, path)
	}
	path = filepath.Clean(path)
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("edit file tool: %w", err)
	}
	if !strings.Contains(string(content), oldText) {
		return "", errors.New("edit file tool: old text not found")
	}
	count := strings.Count(string(content), oldText)
	if count > 1 {
		return "", fmt.Errorf("edit file tool: old text appears %d times. Please provide more context to make it unique.", count)
	}
	content = []byte(strings.Replace(string(content), oldText, newText, 1))
	err = os.WriteFile(path, content, 0644)
	if err != nil {
		return "", fmt.Errorf("edit file tool: %w", err)
	}
	return fmt.Sprintf("Successfully edited %s", path), nil
}
