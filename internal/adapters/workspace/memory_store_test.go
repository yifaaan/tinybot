package workspace

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMemoryStore_AppendToday(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewMemoryStore(tmpDir)

	err := store.AppendToday("First entry")
	if err != nil {
		t.Fatalf("AppendToday failed: %v", err)
	}

	content, err := os.ReadFile(store.TodayPath())
	if err != nil {
		t.Fatalf("Failed to read today file: %v", err)
	}

	contentStr := string(content)
	if !contains(contentStr, "First entry") {
		t.Errorf("Content doesn't contain 'First entry': %s", contentStr)
	}

	err = store.AppendToday("Second entry")
	if err != nil {
		t.Fatalf("AppendToday (second) failed: %v", err)
	}

	content, err = os.ReadFile(store.TodayPath())
	if err != nil {
		t.Fatalf("Failed to read today file: %v", err)
	}

	contentStr = string(content)
	if !contains(contentStr, "First entry") || !contains(contentStr, "Second entry") {
		t.Errorf("Content doesn't contain both entries: %s", contentStr)
	}
}

func TestMemoryStore_WriteLongTerm(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewMemoryStore(tmpDir)

	content := "This is long-term memory"
	err := store.WriteLongTerm(content)
	if err != nil {
		t.Fatalf("WriteLongTerm failed: %v", err)
	}

	readContent, err := store.ReadLongTerm()
	if err != nil {
		t.Fatalf("ReadLongTerm failed: %v", err)
	}

	if readContent != content {
		t.Errorf("Expected %q, got %q", content, readContent)
	}
}

func TestMemoryStore_GetRecentMemories(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewMemoryStore(tmpDir)
	today := time.Now()
	for i := 0; i < 3; i++ {
		date := today.AddDate(0, 0, -i)
		dateStr := date.Format("2006-01-02")
		filePath := filepath.Join(store.memoryDir, dateStr+".md")
		content := "Memory for " + dateStr
		err := os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	memories, err := store.GetRecentMemories(3)
	if err != nil {
		t.Fatalf("GetRecentMemories failed: %v", err)
	}

	if memories == "" {
		t.Error("Expected non-empty memories")
	}

	for i := 0; i < 3; i++ {
		date := today.AddDate(0, 0, -i)
		dateStr := date.Format("2006-01-02")
		if !contains(memories, dateStr) {
			t.Errorf("Memories don't contain %s", dateStr)
		}
	}
}

func TestMemoryStore_GetRecentMemories_InvalidDays(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewMemoryStore(tmpDir)

	_, err := store.GetRecentMemories(0)
	if err == nil {
		t.Error("Expected error for days=0")
	}

	_, err = store.GetRecentMemories(-1)
	if err == nil {
		t.Error("Expected error for days=-1")
	}
}

func TestMemoryStore_ListMemoryFiles(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewMemoryStore(tmpDir)

	dates := []string{"2026-03-15", "2026-03-16", "2026-03-14"}
	for _, date := range dates {
		filePath := filepath.Join(store.memoryDir, date+".md")
		err := os.WriteFile(filePath, []byte("test"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	files, err := store.ListMemoryFiles()
	if err != nil {
		t.Fatalf("ListMemoryFiles failed: %v", err)
	}

	if len(files) != 3 {
		t.Errorf("Expected 3 files, got %d", len(files))
	}

	if len(files) >= 2 && !contains(files[0], "2026-03-16") {
		t.Errorf("Expected newest file first, got %s", files[0])
	}
}

func TestMemoryStore_ListMemoryFiles_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewMemoryStore(tmpDir)

	files, err := store.ListMemoryFiles()
	if err != nil {
		t.Fatalf("ListMemoryFiles failed: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("Expected 0 files, got %d", len(files))
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
