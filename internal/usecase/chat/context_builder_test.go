package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"tinybot/internal/domain/model"
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

	prompt := builder.BuildSystemPrompt(nil)
	if !strings.Contains(prompt, "You are tinybot, a helpful AI assistant.") {
		t.Fatalf("System prompt missing base instruction: %s", prompt)
	}
}

func TestContextBuilder_BuildSystemPrompt_WithMemory(t *testing.T) {
	root := t.TempDir()
	builder := NewContextBuilder(root)
	writeTestFile(t, filepath.Join(root, "memory", "MEMORY.md"), "user likes Go")
	writeTestFile(t, filepath.Join(root, "memory", utils.TodayDate()+".md"), "today's prompt")

	prompt := builder.BuildSystemPrompt(nil)
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
	root := t.TempDir()
	builder := NewContextBuilder(root)
	writeTestFile(t, filepath.Join(root, "skills", "foo", "SKILL.md"), "# Foo Skill\n\nUse foo skill wisely.")

	prompt := builder.BuildSystemPrompt(nil)
	if !strings.Contains(prompt, "# Skills") {
		t.Fatalf("System prompt missing skills section: %s", prompt)
	}
	if !strings.Contains(prompt, "<skills>") {
		t.Fatalf("System prompt missing skills summary root: %s", prompt)
	}
	if !strings.Contains(prompt, "<name>foo</name>") {
		t.Fatalf("System prompt missing skill name in summary: %s", prompt)
	}
	if !strings.Contains(prompt, "<location>") {
		t.Fatalf("System prompt missing skill location in summary: %s", prompt)
	}
}

func TestContextBuilder_BuildMessages_WithSelectedSkills(t *testing.T) {
	root := t.TempDir()
	builder := NewContextBuilder(root)
	writeTestFile(t, filepath.Join(root, "skills", "foo", "SKILL.md"), "# Foo Skill\n\nUse foo skill wisely.")

	history := []*model.Message{{
		Role:    model.RoleAssistant,
		Content: "previous answer",
	}}

	messages := builder.BuildMessages(history, "current question", []string{"foo"})
	if len(messages) != 3 {
		t.Fatalf("len(messages) = %d, want 3", len(messages))
	}

	systemRole, _ := messages[0]["role"].(string)
	if systemRole != "system" {
		t.Fatalf("messages[0].role = %q, want %q", systemRole, "system")
	}
	systemContent, _ := messages[0]["content"].(string)
	if !strings.Contains(systemContent, "# Skills") {
		t.Fatalf("system message missing skills section: %s", systemContent)
	}
	if !strings.Contains(systemContent, "<name>foo</name>") {
		t.Fatalf("system message missing skill summary entry: %s", systemContent)
	}
	if strings.Contains(systemContent, "Use foo skill wisely.") {
		t.Fatalf("system message unexpectedly contains full skill body: %s", systemContent)
	}

	historyRole, _ := messages[1]["role"].(string)
	historyContent, _ := messages[1]["content"].(string)
	if historyRole != model.RoleAssistant || historyContent != "previous answer" {
		t.Fatalf("unexpected history message: %#v", messages[1])
	}

	userRole, _ := messages[2]["role"].(string)
	userContent, _ := messages[2]["content"].(string)
	if userRole != model.RoleUser || userContent != "current question" {
		t.Fatalf("unexpected current user message: %#v", messages[2])
	}
}
