package app

import (
	"os"
	"path/filepath"
)

type Status struct {
	Workspace        string
	WorkspaceExists  bool
	ConfigExists     bool
	MemoryFileExists bool
	SkillsDirExists  bool
	BootstrapFiles   map[string]bool
	MissingFiles     []string
}

func CheckStatus(workspace string) Status {
	workspace = ResolveWorkspacePath(workspace)

	status := Status{
		Workspace:      workspace,
		BootstrapFiles: make(map[string]bool, len(DefaultBootstrapFiles)),
		MissingFiles:   make([]string, 0, len(DefaultBootstrapFiles)+2),
	}

	status.WorkspaceExists = dirExists(workspace)
	status.ConfigExists = fileExists(getConfigPath(workspace))
	status.MemoryFileExists = fileExists(filepath.Join(workspace, "memory", "MEMORY.md"))
	status.SkillsDirExists = dirExists(filepath.Join(workspace, "skills"))

	for _, name := range DefaultBootstrapFiles {
		path := filepath.Join(workspace, name)
		ok := fileExists(path)
		status.BootstrapFiles[name] = ok
		if !ok {
			status.MissingFiles = append(status.MissingFiles, name)
		}
	}

	if !status.ConfigExists {
		status.MissingFiles = append(status.MissingFiles, filepath.Base(getConfigPath(workspace)))
	}
	if !status.MemoryFileExists {
		status.MissingFiles = append(status.MissingFiles, filepath.Join("memory", "MEMORY.md"))
	}

	return status
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
