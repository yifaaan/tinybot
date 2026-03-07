package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"tinybot/internal/utils"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error: %v", path, err)
	}
}

func todayFilePath(root string) string {
	return filepath.Join(root, "memory", utils.TodayDate()+".md")
}

func TestNewMemoryStore_CreatesMemoryDirAndPaths(t *testing.T) {
	root := t.TempDir()
	store := NewMemoryStore(root)

	wantDir := filepath.Join(root, "memory")
	if store.memoryDir != wantDir {
		t.Fatalf("memoryDir = %q, want %q", store.memoryDir, wantDir)
	}

	info, err := os.Stat(wantDir)
	if err != nil {
		t.Fatalf("memory dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("memory path is not a directory: %q", wantDir)
	}

	wantLongTerm := filepath.Join(wantDir, "MEMORY.md")
	if got := store.LongTermPath(); got != wantLongTerm {
		t.Fatalf("LongTermPath() = %q, want %q", got, wantLongTerm)
	}

	if got := store.TodayPath(); got != todayFilePath(root) {
		t.Fatalf("TodayPath() = %q, want %q", got, todayFilePath(root))
	}
}

func TestReadLongTerm(t *testing.T) {
	root := t.TempDir()
	store := NewMemoryStore(root)

	want := "user likes Go"
	writeTestFile(t, filepath.Join(root, "memory", "MEMORY.md"), want)

	got, err := store.ReadLongTerm()
	if err != nil {
		t.Fatalf("ReadLongTerm() error: %v", err)
	}
	if got != want {
		t.Fatalf("ReadLongTerm() = %q, want %q", got, want)
	}
}

func TestReadLongTerm_NotFound(t *testing.T) {
	root := t.TempDir()
	store := NewMemoryStore(root)

	got, err := store.ReadLongTerm()
	if err == nil {
		t.Fatal("ReadLongTerm() expected error, got nil")
	}
	if got != "" {
		t.Fatalf("ReadLongTerm() = %q, want empty string", got)
	}
}

func TestReadToday_NotFound(t *testing.T) {
	root := t.TempDir()
	store := NewMemoryStore(root)

	got, err := store.ReadToday()
	if err == nil {
		t.Fatal("ReadToday() expected error, got nil")
	}
	if got != "" {
		t.Fatalf("ReadToday() = %q, want empty string", got)
	}
}

func TestReadToday(t *testing.T) {
	root := t.TempDir()
	store := NewMemoryStore(root)

	want := "# notes\nlearn interfaces"
	writeTestFile(t, todayFilePath(root), want)

	got, err := store.ReadToday()
	if err != nil {
		t.Fatalf("ReadToday() error: %v", err)
	}
	if got != want {
		t.Fatalf("ReadToday() = %q, want %q", got, want)
	}
}

func TestMemoryStore_BuildContext_Empty(t *testing.T) {
	root := t.TempDir()
	store := NewMemoryStore(root)

	got := store.BuildContext()
	if got != "" {
		t.Fatalf("BuildContext() = %q, want empty string", got)
	}
}

func TestMemoryStore_BuildContext_WithLongTermMemory(t *testing.T) {
	root := t.TempDir()
	store := NewMemoryStore(root)

	writeTestFile(t, filepath.Join(root, "memory", "MEMORY.md"), "remember user prefers concise replies")

	got := store.BuildContext()
	want := "## Long-term Memory\nremember user prefers concise replies"
	if got != want {
		t.Fatalf("BuildContext() = %q, want %q", got, want)
	}
}

func TestMemoryStore_BuildContext_WithTodayNote(t *testing.T) {
	root := t.TempDir()
	store := NewMemoryStore(root)

	writeTestFile(t, todayFilePath(root), "# today\nfinish tests")

	got := store.BuildContext()
	want := "## Today's Notes\n# today\nfinish tests"
	if got != want {
		t.Fatalf("BuildContext() = %q, want %q", got, want)
	}
}

func TestMemoryStore_BuildContext_WithLongTermAndToday(t *testing.T) {
	root := t.TempDir()
	store := NewMemoryStore(root)

	writeTestFile(t, filepath.Join(root, "memory", "MEMORY.md"), "remember user prefers concise replies")
	writeTestFile(t, todayFilePath(root), "# today\nfinish tests")

	got := store.BuildContext()

	if !strings.Contains(got, "## Long-term Memory\nremember user prefers concise replies") {
		t.Fatalf("BuildContext() missing long-term memory section:\n%s", got)
	}
	if !strings.Contains(got, "## Today's Notes\n# today\nfinish tests") {
		t.Fatalf("BuildContext() missing today's notes section:\n%s", got)
	}

	longTermIndex := strings.Index(got, "## Long-term Memory")
	todayIndex := strings.Index(got, "## Today's Notes")
	if longTermIndex == -1 || todayIndex == -1 {
		t.Fatalf("BuildContext() missing required sections:\n%s", got)
	}
	if longTermIndex > todayIndex {
		t.Fatalf("BuildContext() section order is wrong:\n%s", got)
	}

	want := "## Long-term Memory\nremember user prefers concise replies\n\n## Today's Notes\n# today\nfinish tests"
	if got != want {
		t.Fatalf("BuildContext() = %q, want %q", got, want)
	}
}
