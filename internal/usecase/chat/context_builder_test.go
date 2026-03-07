package chat

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

func TestContextBuilder_BuildSystemPrompt_WithoutWorkspaceFiles(t *testing.T) {
	root := t.TempDir()
	builder := NewContextBuilder(root)

	prompt := builder.BuildSystemPrompt()
	if !strings.Contains(prompt, "You are tinybot, a helpful AI assistant.") {
		t.Fatalf("System prompt missing base instruction: %s", prompt)
	}
}

func TestContextBuilder_BuildSystemPrompt_WithMemory(t *testing.T) {
	root := t.TempDir()
	builder := NewContextBuilder(root)
	writeTestFile(t, filepath.Join(root, "memory", "MEMORY.md"), "user likes Go")
	writeTestFile(t, filepath.Join(root, "memory", utils.TodayDate()+".md"), "today's prompt")

	prompt := builder.BuildSystemPrompt()
	if !strings.Contains(prompt, "## Memory") {
		t.Fatalf("System prompt missing memory section: %s", prompt)
	}
	if !strings.Contains(prompt, "user likes Go") {
		t.Fatalf("System prompt missing long-term memory content: %s", prompt)
	}
	if !strings.Contains(prompt, "today's prompt") {
		t.Fatalf("System prompt missing today's memory content: %s", prompt)
	}
}

func TestContextBuilder_BuildSystemPrompt_WithSkillsSummary(t *testing.T) {
	t.Skip("TODO: implement skills summary prompt test")
}

func TestContextBuilder_BuildMessages_WithSelectedSkills(t *testing.T) {
	t.Skip("TODO: implement selected skills injection test")
}
