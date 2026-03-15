package app

import (
	"path/filepath"
	"strings"
)

const (
	// defaultWorkspaceRootDir keeps the MVP workspace inside the current project.
	//
	// For this rewrite phase, a project-local workspace is easier to inspect and reason about
	// than a hidden directory in the user's home folder. The important part is not the exact
	// location, but that every caller uses the same default.
	defaultWorkspaceRootDir = "./.tinybot"
	defaultWorkspaceDirName = "workspace"
)

// DefaultWorkspacePath returns the single source of truth for the app's default workspace.
func DefaultWorkspacePath() string {
	return filepath.Join(defaultWorkspaceRootDir, defaultWorkspaceDirName)
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
