package app

import (
	"path/filepath"
	"strings"
)

const (
	// defaultWorkspaceDir is the default workspace directory name.
	// Workspace is located at ./workspace in the project root.
	defaultWorkspaceDir = "workspace"
)

// DefaultWorkspacePath returns the single source of truth for the app's default workspace.
func DefaultWorkspacePath() string {
	return filepath.Join(".", defaultWorkspaceDir)
}

// ResolveWorkspacePath normalizes optional caller input.
//
// Callers can pass an explicit workspace for tests or custom setups. When they pass an empty
// string, we fall back to the shared default path above.
func ResolveWorkspacePath(workspace string) string {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		return DefaultWorkspacePath()
	}
	return workspace
}
