package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	workspaceadapter "tinybot/internal/adapters/workspace"
	"tinybot/internal/domain/model"
	"tinybot/internal/utils"
)

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error: %v", path, err)
	}
}

type stubBootstrapReader struct {
	docs map[string]string
	err  error
}

func (s stubBootstrapReader) Load(names []string) (map[string]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make(map[string]string, len(names))
	for _, name := range names {
		if content, ok := s.docs[name]; ok {
			out[name] = content
		}
	}
	return out, nil
}

type stubMemoryReader struct {
	content string
}

func (s stubMemoryReader) BuildContext() string {
	return s.content
}

type stubSkillCatalog struct {
	always  []string
	body    string
	summary string
	loaded  [][]string
}

func (s *stubSkillCatalog) GetAlwaysSkills() ([]string, error) {
	return append([]string(nil), s.always...), nil
}

func (s *stubSkillCatalog) LoadSkillsForContext(names []string) (string, error) {
	s.loaded = append(s.loaded, append([]string(nil), names...))
	return s.body, nil
}

func (s *stubSkillCatalog) BuildSummary() (string, error) {
	return s.summary, nil
}

func newWorkspacePromptBuilder(workspace string) *Builder {
	return NewPromptBuilder(
		workspace,
		workspaceadapter.NewBootstrapReader(workspace),
		workspaceadapter.NewMemoryStore(workspace),
		workspaceadapter.NewSkillsLoader(workspace, ""),
	)
}

func TestPromptBuilder_BuildSystemPrompt_WithoutWorkspaceFiles(t *testing.T) {
	builder := newWorkspacePromptBuilder(t.TempDir())

	prompt := builder.BuildSystemPrompt(nil)
	if !strings.Contains(prompt, "You are tinybot, a helpful AI assistant.") {
		t.Fatalf("system prompt missing base instruction: %s", prompt)
	}
}

func TestPromptBuilder_BuildSystemPrompt_UsesInjectedDependencies(t *testing.T) {
	skills := &stubSkillCatalog{
		always:  []string{"foo"},
		body:    "### Skill: foo\n\nUse foo carefully.",
		summary: "<skills>\n  <skill available=\"true\">\n   <name>foo</name>\n   <description>test</description>\n   <location>/skills/foo/SKILL.md</location>\n   </skill>\n</skills>",
	}
	builder := NewPromptBuilder(
		t.TempDir(),
		stubBootstrapReader{docs: map[string]string{
			"AGENTS.md": "agents",
			"SOUL.md":   "soul",
		}},
		stubMemoryReader{content: "## Long-term Memory\nprefers Go"},
		skills,
	)

	prompt := builder.BuildSystemPrompt(nil)
	if !strings.Contains(prompt, "## AGENTS.md") || !strings.Contains(prompt, "agents") {
		t.Fatalf("system prompt missing injected bootstrap docs: %s", prompt)
	}
	if !strings.Contains(prompt, "## Memory") || !strings.Contains(prompt, "prefers Go") {
		t.Fatalf("system prompt missing injected memory content: %s", prompt)
	}
	if !strings.Contains(prompt, "# Active Skills") || !strings.Contains(prompt, "Use foo carefully.") {
		t.Fatalf("system prompt missing injected active skill content: %s", prompt)
	}
	if !strings.Contains(prompt, "# Skills") || !strings.Contains(prompt, "<name>foo</name>") {
		t.Fatalf("system prompt missing injected skill summary: %s", prompt)
	}
}

func TestPromptBuilder_BuildSystemPrompt_BootstrapOrderAndOptionalIdentity(t *testing.T) {
	root := t.TempDir()
	builder := newWorkspacePromptBuilder(root)
	writeTestFile(t, filepath.Join(root, "AGENTS.md"), "agents")
	writeTestFile(t, filepath.Join(root, "SOUL.md"), "soul")
	writeTestFile(t, filepath.Join(root, "USER.md"), "user")
	writeTestFile(t, filepath.Join(root, "TOOLS.md"), "tools")

	prompt := builder.BuildSystemPrompt(nil)
	if strings.Contains(prompt, "IDENTITY.md") {
		t.Fatalf("optional IDENTITY.md should not appear when absent: %s", prompt)
	}

	agentsIdx := strings.Index(prompt, "## AGENTS.md")
	soulIdx := strings.Index(prompt, "## SOUL.md")
	userIdx := strings.Index(prompt, "## USER.md")
	toolsIdx := strings.Index(prompt, "## TOOLS.md")
	if !(agentsIdx < soulIdx && soulIdx < userIdx && userIdx < toolsIdx) {
		t.Fatalf("bootstrap docs are not in stable order: %s", prompt)
	}
}

func TestPromptBuilder_BuildSystemPrompt_WithOptionalIdentity(t *testing.T) {
	root := t.TempDir()
	builder := newWorkspacePromptBuilder(root)
	writeTestFile(t, filepath.Join(root, "IDENTITY.md"), "custom identity")

	prompt := builder.BuildSystemPrompt(nil)
	if !strings.Contains(prompt, "## IDENTITY.md") {
		t.Fatalf("system prompt missing optional identity section: %s", prompt)
	}
	if !strings.Contains(prompt, "custom identity") {
		t.Fatalf("system prompt missing optional identity content: %s", prompt)
	}
}

func TestPromptBuilder_BuildSystemPrompt_WithMemory(t *testing.T) {
	root := t.TempDir()
	builder := newWorkspacePromptBuilder(root)
	writeTestFile(t, filepath.Join(root, "memory", "MEMORY.md"), "user likes Go")
	writeTestFile(t, filepath.Join(root, "memory", utils.TodayDate()+".md"), "today's prompt")

	prompt := builder.BuildSystemPrompt(nil)
	if !strings.Contains(prompt, "## Memory") {
		t.Fatalf("system prompt missing memory section: %s", prompt)
	}
	if !strings.Contains(prompt, "user likes Go") {
		t.Fatalf("system prompt missing long-term memory content: %s", prompt)
	}
	if !strings.Contains(prompt, "today's prompt") {
		t.Fatalf("system prompt missing today's memory content: %s", prompt)
	}
}

func TestPromptBuilder_BuildSystemPrompt_MissingMemoryDoesNotFail(t *testing.T) {
	builder := newWorkspacePromptBuilder(t.TempDir())

	prompt := builder.BuildSystemPrompt(nil)
	if strings.Contains(prompt, "panic") {
		t.Fatalf("system prompt should degrade gracefully: %s", prompt)
	}
}

func TestPromptBuilder_BuildSystemPrompt_WithAlwaysSkill(t *testing.T) {
	root := t.TempDir()
	builder := newWorkspacePromptBuilder(root)
	writeTestFile(t, filepath.Join(root, "skills", "foo", "SKILL.md"), `---
name: foo
description: test skill
metadata: {"always": true}
---

# Foo Skill

Use foo skill wisely.`)

	prompt := builder.BuildSystemPrompt(nil)
	if !strings.Contains(prompt, "# Active Skills") {
		t.Fatalf("system prompt missing active skills section: %s", prompt)
	}
	if !strings.Contains(prompt, "Use foo skill wisely.") {
		t.Fatalf("system prompt missing full active skill body: %s", prompt)
	}
}

func TestPromptBuilder_BuildSystemPrompt_LoadsSelectedSkills(t *testing.T) {
	root := t.TempDir()
	builder := newWorkspacePromptBuilder(root)
	writeTestFile(t, filepath.Join(root, "skills", "foo", "SKILL.md"), "# Foo Skill\n\nUse foo skill wisely.")

	prompt := builder.BuildSystemPrompt([]string{"foo"})
	if !strings.Contains(prompt, "# Active Skills") {
		t.Fatalf("system prompt missing active skills section: %s", prompt)
	}
	if !strings.Contains(prompt, "Use foo skill wisely.") {
		t.Fatalf("system prompt missing selected skill body: %s", prompt)
	}
}

func TestPromptBuilder_BuildSystemPrompt_MissingSelectedSkillDoesNotFail(t *testing.T) {
	builder := newWorkspacePromptBuilder(t.TempDir())

	prompt := builder.BuildSystemPrompt([]string{"missing"})
	if strings.Contains(prompt, "panic") {
		t.Fatalf("system prompt should degrade gracefully for missing selected skill: %s", prompt)
	}
	if strings.Contains(prompt, "### Skill: missing") {
		t.Fatalf("system prompt should skip missing selected skill: %s", prompt)
	}
}

func TestPromptBuilder_BuildSystemPrompt_DeduplicatesAlwaysAndSelectedSkills(t *testing.T) {
	root := t.TempDir()
	skills := &stubSkillCatalog{
		always:  []string{"foo"},
		body:    "### Skill: foo\n\nUse foo carefully.",
		summary: "<skills></skills>",
	}
	builder := NewPromptBuilder(root, stubBootstrapReader{}, stubMemoryReader{}, skills)

	prompt := builder.BuildSystemPrompt([]string{"foo", "foo"})
	if count := strings.Count(prompt, "### Skill: foo"); count != 1 {
		t.Fatalf("selected and always skills should be loaded once, count=%d prompt=%s", count, prompt)
	}
	if len(skills.loaded) != 1 {
		t.Fatalf("LoadSkillsForContext call count = %d, want 1", len(skills.loaded))
	}
	if got := strings.Join(skills.loaded[0], ","); got != "foo" {
		t.Fatalf("loaded skills = %q, want %q", got, "foo")
	}
}

func TestPromptBuilder_BuildSystemPrompt_WithSkillsSummary(t *testing.T) {
	root := t.TempDir()
	builder := newWorkspacePromptBuilder(root)
	writeTestFile(t, filepath.Join(root, "skills", "foo", "SKILL.md"), "# Foo Skill\n\nUse foo skill wisely.")

	prompt := builder.BuildSystemPrompt(nil)
	if !strings.Contains(prompt, "# Skills") {
		t.Fatalf("system prompt missing skills section: %s", prompt)
	}
	if !strings.Contains(prompt, "<skills>") {
		t.Fatalf("system prompt missing skills summary root: %s", prompt)
	}
	if !strings.Contains(prompt, "<name>foo</name>") {
		t.Fatalf("system prompt missing skill name in summary: %s", prompt)
	}
	if !strings.Contains(prompt, "<location>") {
		t.Fatalf("system prompt missing skill location in summary: %s", prompt)
	}
}

func TestPromptBuilder_BuildMessages_WithHistoryAndSelectedSkill(t *testing.T) {
	root := t.TempDir()
	builder := newWorkspacePromptBuilder(root)
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
	if systemRole != model.RoleSystem {
		t.Fatalf("messages[0].role = %q, want %q", systemRole, model.RoleSystem)
	}

	systemContent, _ := messages[0]["content"].(string)
	if !strings.Contains(systemContent, "# Skills") {
		t.Fatalf("system message missing skills section: %s", systemContent)
	}
	if !strings.Contains(systemContent, "<name>foo</name>") {
		t.Fatalf("system message missing skill summary entry: %s", systemContent)
	}
	if !strings.Contains(systemContent, "Use foo skill wisely.") {
		t.Fatalf("system message missing selected skill body: %s", systemContent)
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
