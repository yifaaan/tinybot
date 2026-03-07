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

type WriteFileTool struct {
	workspace string
}

func NewWriteFileTool(workspace string) *WriteFileTool {
	return &WriteFileTool{workspace: workspace}
}

func (t *WriteFileTool) Spec() ports.ToolSpec {
	return ports.ToolSpec{
		Name:        "write_file",
		Description: "Write content to a file at the given path. Creates parent directories if needed.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path":{
					"type": "string",
					"description": "The path to the file to write"
				},
				"content":{
					"type": "string",
					"description": "The content to write to the file"
				}
			},
			"required": ["path", "content"]
		}`),
	}
}

func (t *WriteFileTool) Execute(ctx context.Context, params map[string]any) (string, error) {
	path, _ := params["path"].(string)
	content, _ := params["content"].(string)
	if strings.TrimSpace(path) == "" {
		return "", errors.New("write file tool: `path` is required")
	}
	if strings.TrimSpace(content) == "" {
		return "", errors.New("write file tool: `content` is required")
	}
	path, err := ExpandUser(path)
	if err != nil {
		return "", fmt.Errorf("write file tool: %w", err)
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(t.workspace, path)
	}
	path = filepath.Clean(path)
	err = os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return "", fmt.Errorf("write file tool: %w", err)
	}
	err = os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return "", fmt.Errorf("write file tool: %w", err)
	}
	return fmt.Sprintf("Successfully write %d bytes to %s", len(content), path), nil
}
