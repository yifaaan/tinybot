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

// ListDirTool is tool to list the contents of a directory.
type ListDirTool struct {
	workspace string
}

func NewListDirTool(workspace string) *ListDirTool {
	return &ListDirTool{workspace: workspace}
}

func (t *ListDirTool) Spec() ports.ToolSpec {
	return ports.ToolSpec{
		Name:        "list_dir",
		Description: "List the contents of a directory.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "The directory path to list"
				}
			},
			"required": ["path"]
		}`),
	}
}

func (t *ListDirTool) Execute(ctx context.Context, params map[string]any) (string, error) {

	path, _ := params["path"].(string)
	if path == "" {
		return "", errors.New("list dir tool: `path` is required")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(t.workspace, path)
	}
	path = filepath.Clean(path)
	stat, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("list dir tool: %w", err)
	}
	if !stat.IsDir() {
		return "", errors.New("list dir tool: not a directory")
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("list dir tool: %w", err)
	}

	content := make([]string, 0, len(entries))
	for _, entry := range entries {
		prefix := "📄 "
		if entry.IsDir() {
			prefix = "📁 "
		}
		content = append(content, prefix+entry.Name())
	}
	return strings.Join(content, "\n"), nil
}
