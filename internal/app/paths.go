package app

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	// defaultWorkspaceDir is the default workspace directory name.
	// Workspace is located at ./workspace in the project root.
	defaultWorkspaceDir = "workspace"
)

var (
	getwd      = os.Getwd
	executable = os.Executable
)

// DefaultWorkspacePath returns the single source of truth for the app's default workspace.
func DefaultWorkspacePath() string {
	return filepath.Join(defaultBaseDir(), defaultWorkspaceDir)
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

func defaultBaseDir() string {
	for _, base := range candidateBaseDirs() {
		if fileExists(filepath.Join(base, "config.json")) {
			return base
		}
		if fileExists(filepath.Join(base, "go.mod")) && fileExists(filepath.Join(base, "wails.json")) {
			return base
		}
	}

	candidates := candidateBaseDirs()
	if len(candidates) > 0 {
		return candidates[0]
	}
	return "."
}

func candidateBaseDirs() []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 8)

	addChain := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return
		}
		for _, dir := range ascendDirs(abs) {
			if _, ok := seen[dir]; ok {
				continue
			}
			seen[dir] = struct{}{}
			out = append(out, dir)
		}
	}

	if cwd, err := getwd(); err == nil {
		addChain(cwd)
	}
	if exe, err := executable(); err == nil {
		addChain(filepath.Dir(exe))
	}

	return out
}

func ascendDirs(path string) []string {
	out := make([]string, 0, 8)
	for {
		out = append(out, path)
		parent := filepath.Dir(path)
		if parent == path {
			break
		}
		path = parent
	}
	return out
}
