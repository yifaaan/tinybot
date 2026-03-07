package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"tinybot/internal/utils"
)

// Memory system for the agent.
// Supports daily notes (memory/YYYY-MM-DD.md) and long-term memory (MEMORY.md).
// Compatible with clawbot memory format.
type MemoryStore struct {
	workspace  string
	memoryDir  string
	memoryFile string
}

func NewMemoryStore(workspace string) *MemoryStore {
	memoryDir, err := utils.EnsureDir(filepath.Join(workspace, "memory"))
	if err != nil {
		panic(err)
	}
	return &MemoryStore{
		workspace:  workspace,
		memoryDir:  memoryDir,
		memoryFile: filepath.Join(memoryDir, "MEMORY.md"),
	}
}

// LongTermPath return path to memory/MEMORY.md
func (s *MemoryStore) LongTermPath() string {
	return s.memoryFile
}

// TodayPath return path to memory/YYYY-MM-DD.md for today
func (s *MemoryStore) TodayPath() string {
	filename := utils.TodayDate() + ".md"
	path := filepath.Join(s.memoryDir, filename)
	return path
}

// ReadLongTerm read long-term memory content from MEMORY.md
func (s *MemoryStore) ReadLongTerm() (string, error) {
	content, err := os.ReadFile(s.LongTermPath())
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// ReadToday read today's memory note content
func (s *MemoryStore) ReadToday() (string, error) {

	path := s.TodayPath()
	if path == "" {
		return "", errors.New("read today: today's memory file not found")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// BuildContext combine long-term memory and today's notes into prompt context
func (s *MemoryStore) BuildContext() string {
	parts := make([]string, 0, 2)

	// long-term memory
	longTerm, err := s.ReadLongTerm()
	if err == nil {
		parts = append(parts, fmt.Sprintf("## Long-term Memory\n%s", longTerm))
	}
	// Today's notes
	today, err := s.ReadToday()
	if err == nil {
		parts = append(parts, fmt.Sprintf("## Today's Notes\n%s", today))
	}
	return strings.Join(parts, "\n\n")
}
