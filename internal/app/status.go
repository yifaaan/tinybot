package app

import (
	"os"
	"path/filepath"
	ws "tinybot/internal/adapters/workspace"
)

type Status struct {
	Workspace           string
	WorkspaceExists     bool
	ConfigExists        bool
	MemoryFileExists    bool
	SkillsDirExists     bool
	HeartbeatFileExists bool
	BootstrapFiles      map[string]bool
	MissingFiles        []string
	Skills              []SkillInfo
}

type SkillInfo struct {
	Name      string
	Available bool
	Missing   []string // if not available, list missing requirements
}

func CheckStatus(workspace string) Status {
	workspace = ResolveWorkspacePath(workspace)

	status := Status{
		Workspace:      workspace,
		BootstrapFiles: make(map[string]bool, len(DefaultBootstrapFiles)),
		MissingFiles:   make([]string, 0, len(DefaultBootstrapFiles)+2),
	}

	status.WorkspaceExists = dirExists(workspace)
	status.ConfigExists = fileExists(getConfigPath())
	status.MemoryFileExists = fileExists(filepath.Join(workspace, "memory", "MEMORY.md"))
	status.SkillsDirExists = dirExists(filepath.Join(workspace, "skills"))
	status.HeartbeatFileExists = fileExists(filepath.Join(workspace, "HEARTBEAT.md"))
	for _, name := range DefaultBootstrapFiles {
		path := filepath.Join(workspace, name)
		ok := fileExists(path)
		status.BootstrapFiles[name] = ok
		if !ok {
			status.MissingFiles = append(status.MissingFiles, name)
		}
	}

	if !status.ConfigExists {
		status.MissingFiles = append(status.MissingFiles, filepath.Base(getConfigPath()))
	}
	if !status.MemoryFileExists {
		status.MissingFiles = append(status.MissingFiles, filepath.Join("memory", "MEMORY.md"))
	}

	loader := ws.NewSkillsLoader(workspace, "")
	skills, err := loader.ListSkills(false)
	if err == nil {
		status.Skills = make([]SkillInfo, len(skills))
		for i, s := range skills {
			status.Skills[i] = SkillInfo{
				Name:      s.Name,
				Available: s.Available,
				Missing:   s.Missing,
			}
		}
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
