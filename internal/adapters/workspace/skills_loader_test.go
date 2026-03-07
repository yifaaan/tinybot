package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSkillFile(t *testing.T, root, name, content string) string {
	t.Helper()
	path := filepath.Join(root, name, "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error: %v", path, err)
	}
	return path
}

func findSkill(skills []SkillInfo, name string) (SkillInfo, bool) {
	for _, skill := range skills {
		if skill.Name == name {
			return skill, true
		}
	}
	return SkillInfo{}, false
}

func TestSkillsLoader_ListSkills_Empty(t *testing.T) {
	workspace := t.TempDir()
	builtin := t.TempDir()
	loader := NewSkillsLoader(workspace, builtin)

	skills, err := loader.ListSkills(false)
	if err != nil {
		t.Fatalf("ListSkills() error: %v", err)
	}
	if len(skills) != 0 {
		t.Fatalf("len(skills) = %d, want 0", len(skills))
	}
}

func TestSkillsLoader_ListSkills_FromWorkspace(t *testing.T) {
	workspace := t.TempDir()
	builtin := t.TempDir()
	workspaceSkillPath := writeSkillFile(t, filepath.Join(workspace, "skills"), "foo", "workspace foo")
	builtinBarPath := writeSkillFile(t, builtin, "bar", "builtin bar")
	_ = writeSkillFile(t, builtin, "foo", "builtin foo should be hidden")

	loader := NewSkillsLoader(workspace, builtin)
	skills, err := loader.ListSkills(false)
	if err != nil {
		t.Fatalf("ListSkills() error: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("len(skills) = %d, want 2", len(skills))
	}

	foo, ok := findSkill(skills, "foo")
	if !ok {
		t.Fatal("expected to find workspace skill foo")
	}
	if foo.Source != "workspace" {
		t.Fatalf("foo.Source = %q, want %q", foo.Source, "workspace")
	}
	if foo.Path != workspaceSkillPath {
		t.Fatalf("foo.Path = %q, want %q", foo.Path, workspaceSkillPath)
	}

	bar, ok := findSkill(skills, "bar")
	if !ok {
		t.Fatal("expected to find builtin skill bar")
	}
	if bar.Source != "builtin" {
		t.Fatalf("bar.Source = %q, want %q", bar.Source, "builtin")
	}
	if bar.Path != builtinBarPath {
		t.Fatalf("bar.Path = %q, want %q", bar.Path, builtinBarPath)
	}
}

func TestSkillsLoader_LoadSkill(t *testing.T) {
	workspace := t.TempDir()
	builtin := t.TempDir()
	_ = writeSkillFile(t, filepath.Join(workspace, "skills"), "foo", "workspace version")
	_ = writeSkillFile(t, builtin, "foo", "builtin version")
	_ = writeSkillFile(t, builtin, "bar", "builtin only")

	loader := NewSkillsLoader(workspace, builtin)

	got, err := loader.LoadSkill("foo")
	if err != nil {
		t.Fatalf("LoadSkill(foo) error: %v", err)
	}
	if got != "workspace version" {
		t.Fatalf("LoadSkill(foo) = %q, want %q", got, "workspace version")
	}

	got, err = loader.LoadSkill("bar")
	if err != nil {
		t.Fatalf("LoadSkill(bar) error: %v", err)
	}
	if got != "builtin only" {
		t.Fatalf("LoadSkill(bar) = %q, want %q", got, "builtin only")
	}

	got, err = loader.LoadSkill("missing")
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("LoadSkill(missing) error = %v, want os.ErrNotExist", err)
	}
	if got != "" {
		t.Fatalf("LoadSkill(missing) = %q, want empty string", got)
	}
}

func TestSkillsLoader_LoadSkillsForContext(t *testing.T) {
	workspace := t.TempDir()
	builtin := t.TempDir()
	_ = writeSkillFile(t, filepath.Join(workspace, "skills"), "foo", "workspace foo")
	_ = writeSkillFile(t, builtin, "bar", "builtin bar")

	loader := NewSkillsLoader(workspace, builtin)
	got, err := loader.LoadSkillsForContext([]string{"foo", "missing", "bar"})
	if err != nil {
		t.Fatalf("LoadSkillsForContext() error: %v", err)
	}

	if !strings.Contains(got, "### Skill: foo\n\nworkspace foo") {
		t.Fatalf("context missing foo skill:\n%s", got)
	}
	if !strings.Contains(got, "### Skill: bar\n\nbuiltin bar") {
		t.Fatalf("context missing bar skill:\n%s", got)
	}
	if strings.Contains(got, "missing") {
		t.Fatalf("context should skip missing skills:\n%s", got)
	}
	if !strings.Contains(got, "\n\n---\n\n") {
		t.Fatalf("context missing skill separator:\n%s", got)
	}
}

func TestSkillsLoader_BuildSummary(t *testing.T) {
	workspace := t.TempDir()
	builtin := t.TempDir()
	workspaceSkillPath := writeSkillFile(t, filepath.Join(workspace, "skills"), "foo", "workspace foo")
	loader := NewSkillsLoader(workspace, builtin)

	got, err := loader.BuildSummary()
	if err != nil {
		t.Fatalf("BuildSummary() error: %v", err)
	}
	if !strings.HasPrefix(got, "<skills>\n") {
		t.Fatalf("summary missing opening tag:\n%s", got)
	}
	if !strings.Contains(got, `  <skill available="true">`) {
		t.Fatalf("summary missing skill opening tag:\n%s", got)
	}
	if !strings.Contains(got, "   <name>foo</name>") {
		t.Fatalf("summary missing skill name:\n%s", got)
	}
	if !strings.Contains(got, "   <description></description>") {
		t.Fatalf("summary missing description tag:\n%s", got)
	}
	if !strings.Contains(got, "   <location>"+workspaceSkillPath+"</location>") {
		t.Fatalf("summary missing skill path:\n%s", got)
	}
	if !strings.HasSuffix(got, "</skills>") {
		t.Fatalf("summary missing closing tag:\n%s", got)
	}
}
