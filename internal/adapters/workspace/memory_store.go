package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
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

// AppendToday appends content to today's memory notes.
// If the file doesn't exist, creates it with a date header.
func (s *MemoryStore) AppendToday(content string) error {
	todayPath := s.TodayPath()

	var finalContent string

	// Check if file exists
	if _, err := os.Stat(todayPath); os.IsNotExist(err) {
		// File doesn't exist, add header
		header := fmt.Sprintf("# %s\n\n", utils.TodayDate())
		finalContent = header + content
	} else {
		// File exists, read and append
		existing, err := os.ReadFile(todayPath)
	if err != nil {
			return fmt.Errorf("read existing today file: %w", err)
		}
		finalContent = string(existing) + "\n" + content
	}

	// Write the content
	if err := os.WriteFile(todayPath, []byte(finalContent), 0644); err != nil {
		return fmt.Errorf("write today file: %w", err)
	}

	return nil
}

// WriteLongTerm writes content to long-term memory (MEMORY.md).
func (s *MemoryStore) WriteLongTerm(content string) error {
	if err := os.WriteFile(s.memoryFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("write long-term memory: %w", err)
	}
	return nil
}

// GetRecentMemories gets memories from the last N days.
// Returns combined memory content with separator.
func (s *MemoryStore) GetRecentMemories(days int) (string, error) {
	if days <= 0 {
		return "", errors.New("days must be positive")
	}

	memories := make([]string, 0, days)
	today := time.Now()

	for i := 0; i < days; i++ {
		date := today.AddDate(0, 0, -i)
		dateStr := date.Format("2006-01-02")
		filePath := filepath.Join(s.memoryDir, dateStr+".md")

		// Check if file exists
		if _, err := os.Stat(filePath); err == nil {
			content, err := os.ReadFile(filePath)
			if err != nil {
				continue // Skip files that can't be read
			}
			memories = append(memories, string(content))
		}
	}

	if len(memories) == 0 {
		return "", nil
	}

	return strings.Join(memories, "\n---\n\n"), nil
}

// ListMemoryFiles lists all memory files sorted by date (newest first).
// Returns file paths matching YYYY-MM-DD.md pattern.
func (s *MemoryStore) ListMemoryFiles() ([]string, error) {
	// Check if memory directory exists
	if _, err := os.Stat(s.memoryDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	// Find all files matching the date pattern
	pattern := filepath.Join(s.memoryDir, "????-??-??.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob memory files: %w", err)
	}

	// Sort in reverse order (newest first)
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))

	return matches, nil
}
