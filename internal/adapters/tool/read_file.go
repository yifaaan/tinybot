package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"tinybot/internal/utils"
)

// ReadFileTool is tool to read a file and return its content.
type ReadFileTool struct {
	workspace string
}

func NewReadFileTool(workspace string) *ReadFileTool {
	return &ReadFileTool{workspace: workspace}
}

func (t *ReadFileTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        "read_file",
		Description: "Read a file and return its content.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "The path to the file to read"
				}
			},
			"required": ["path"]
		}`),
	}
}

func (t *ReadFileTool) Execute(ctx context.Context, params map[string]any) (string, error) {

	path, _ := params["path"].(string)
	if strings.TrimSpace(path) == "" {
		return "", errors.New("read file tool: `path` is required")
	}
	path, err := utils.ExpandUser(path)
	if err != nil {
		return "", fmt.Errorf("read file tool: %w", err)
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(t.workspace, path)
	}
	path = filepath.Clean(path)
	stat, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("read file tool: %w", err)
	}
	if stat.IsDir() {
		return "", errors.New("read file tool: not a file")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file tool: %w", err)
	}
	return string(content), nil
}
