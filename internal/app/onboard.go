package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

var DefaultBootstrapFiles = []string{
	"AGENTS.md",
	"SOUL.md",
	"USER.md",
	"TOOLS.md",
}

var DefaultRuntimeFiles = []string{
	"HEARTBEAT.md",
}

type OnBoardResult struct {
	Workspace    string
	CreatedFiles []string
}

// OnBoard initializes the minimum workspace files that ContextBuilder expects.
func OnBoard(ctx context.Context, workspace string) (*OnBoardResult, error) {
	workspace = ResolveWorkspacePath(workspace)
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("onboard: %w", err)
	}

	result := &OnBoardResult{
		Workspace:    workspace,
		CreatedFiles: make([]string, 0, len(DefaultBootstrapFiles)+2),
	}

	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return nil, fmt.Errorf("onboard mkdir workspace: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, "memory"), 0o755); err != nil {
		return nil, fmt.Errorf("onboard mkdir memory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, "skills"), 0o755); err != nil {
		return nil, fmt.Errorf("onboard mkdir skills: %w", err)
	}

	for _, name := range DefaultBootstrapFiles {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("onboard: %w", err)
		}

		created, err := ensureFile(filepath.Join(workspace, name), defaultBootstrapContent(name))
		if err != nil {
			return nil, fmt.Errorf("onboard bootstrap %s: %w", name, err)
		}
		if created {
			result.CreatedFiles = append(result.CreatedFiles, name)
		}
	}

	createdMemory, err := ensureFile(filepath.Join(workspace, "memory", "MEMORY.md"), defaultMemoryMD())
	if err != nil {
		return nil, fmt.Errorf("onboard memory: %w", err)
	}
	if createdMemory {
		result.CreatedFiles = append(result.CreatedFiles, filepath.Join("memory", "MEMORY.md"))
	}

	createHeartbeat, err := ensureFile(filepath.Join(workspace, "HEARTBEAT.md"), defaultHeartbeatMD())
	if err != nil {
		return nil, fmt.Errorf("onboard heartbeat: %w", err)
	}
	if createHeartbeat {
		result.CreatedFiles = append(result.CreatedFiles, "HEARTBEAT.md")
	}

	configPath := getConfigPath(workspace)
	if !fileExists(configPath) {
		cfg := DefaultConfig()
		cfg.Agents.Workspace = workspace

		// TODO: when we support multiple providers, choose defaults here more carefully.
		if err := SaveConfig(cfg, workspace); err != nil {
			return nil, fmt.Errorf("onboard config: %w", err)
		}
		result.CreatedFiles = append(result.CreatedFiles, filepath.Base(configPath))
	}

	return result, nil
}

func ensureFile(path string, content string) (bool, error) {
	if fileExists(path) {
		return false, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("ensure file mkdir: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return false, fmt.Errorf("ensure file write: %w", err)
	}

	return true, nil
}

func defaultBootstrapContent(name string) string {
	switch name {
	case "AGENTS.md":
		return defaultAgentsMD()
	case "SOUL.md":
		return defaultSoulMD()
	case "USER.md":
		return defaultUserMD()
	case "TOOLS.md":
		return defaultToolsMD()
	default:
		return ""
	}
}

func defaultAgentsMD() string {
	return `# Agent Instructions

You are a helpful AI assistant.

## Guidelines

- Explain actions clearly
- Ask for clarification when requests are ambiguous
- Use tools carefully
- Remember important context in memory files
`
}

func defaultSoulMD() string {
	return `# Soul

I am tinybot, a lightweight AI assistant.

## Personality

- Helpful
- Clear
- Practical
`
}

func defaultUserMD() string {
	return `# User

## Preferences

- Language: Chinese
- Technical level: Go beginner
`
}

func defaultToolsMD() string {
	return `# Tools

This workspace can use file, shell, and web tools.

## Reminder

- Prefer reading existing files before editing
- Keep changes small and testable
`
}

func defaultMemoryMD() string {
	return `# Long-term Memory

## User Information

## Preferences

## Important Notes
`
}

func defaultHeartbeatMD() string {
	return `# Heartbeat

  This file is checked by tinybot's background heartbeat service.

  ## How to use
  - Put short actionable tasks here.
  - Empty files, headers, comments, and empty checkboxes are ignored.
  - Keep this file minimal. It should contain recurring self-check instructions.

  <!-- Example:
  Review memory/MEMORY.md and today's note once every morning.
  -->`
}
