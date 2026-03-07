package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Default builtin skills directory (relative to this file)
var BuiltinSkillsDir = func() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot get current file")
	}
	return filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(file))))), "skills")
}()

var xmlEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
)

type SkillInfo struct {
	Name        string
	Path        string
	Source      string
	Description string
	Available   bool
}

// SkillsLoader is responsible for discovering and loading skills from the workspace and builtin directories.
type SkillsLoader struct {
	workspace          string
	workspaceSkillsDir string
	builtinDir         string
}

// NewSkillsLoader creates a new SkillsLoader with the given workspace and builtin directories.
func NewSkillsLoader(workspace string, builtinDir string) *SkillsLoader {
	if builtinDir == "" {
		builtinDir = BuiltinSkillsDir
	}
	return &SkillsLoader{
		workspace:          workspace,
		workspaceSkillsDir: filepath.Join(workspace, "skills"),
		builtinDir:         builtinDir,
	}
}

// ListSkills lists all available skills, optionally filtering out unavailable ones.
// TODO: Check skill availability based on metadata or other criteria.
func (l *SkillsLoader) ListSkills(filterUnavailable bool) ([]SkillInfo, error) {
	skills := make([]SkillInfo, 0, 5)
	seen := make(map[string]struct{})

	// Workspace skills (highest priority)
	_, err := os.Stat(l.workspaceSkillsDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		wd, err := os.ReadDir(l.workspaceSkillsDir)
		if err != nil {
			return nil, err
		}

		// Read workspace skills
		for _, skillDir := range wd {
			if !skillDir.IsDir() {
				continue
			}
			skillFile := filepath.Join(l.workspaceSkillsDir, skillDir.Name(), "SKILL.md")
			_, err := os.Stat(skillFile)
			if err != nil {
				continue
			}
			skills = append(skills, SkillInfo{
				Name:      skillDir.Name(),
				Path:      skillFile,
				Source:    "workspace",
				Available: true,
			})
			seen[skillDir.Name()] = struct{}{}
		}
	}

	// Builtin skills (lower priority)
	_, err = os.Stat(l.builtinDir)
	if err == nil {
		bd, err := os.ReadDir(l.builtinDir)
		if err != nil {
			return skills, err
		}
		for _, skillDir := range bd {
			if !skillDir.IsDir() {
				continue
			}
			if _, ok := seen[skillDir.Name()]; ok {
				continue
			}
			skillFile := filepath.Join(l.builtinDir, skillDir.Name(), "SKILL.md")
			_, err := os.Stat(skillFile)
			if err != nil {
				continue
			}
			skills = append(skills, SkillInfo{
				Name:      skillDir.Name(),
				Path:      skillFile,
				Source:    "builtin",
				Available: true,
			})
			seen[skillDir.Name()] = struct{}{}
		}
	}
	return skills, nil
}

// LoadSkill load SKILL.md content by skill name
func (l *SkillsLoader) LoadSkill(name string) (string, error) {
	// Check workspace first
	workspaceSkill := filepath.Join(l.workspaceSkillsDir, name, "SKILL.md")
	_, err := os.Stat(workspaceSkill)
	if err == nil {
		content, err := os.ReadFile(workspaceSkill)
		if err == nil {
			return string(content), nil
		}
	}
	// Check builtin next
	builtinSkill := filepath.Join(l.builtinDir, name, "SKILL.md")
	_, err = os.Stat(builtinSkill)
	if err == nil {
		content, err := os.ReadFile(builtinSkill)
		if err == nil {
			return string(content), nil
		}
	}
	return "", os.ErrNotExist
}

// LoadSkillsForContext load selected skills and format them for context injection
func (l *SkillsLoader) LoadSkillsForContext(names []string) (string, error) {
	parts := make([]string, 0, len(names))
	for _, name := range names {
		content, err := l.LoadSkill(name)
		if err != nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("### Skill: %s\n\n%s", name, content))
	}
	return strings.Join(parts, "\n\n---\n\n"), nil
}

// BuildSummary Build a summary of all skills (XML-formatted, name, description, path, availability).
// This is used for progressive loading - the agent can read the full
// skill content using read_file when needed.
func (l *SkillsLoader) BuildSummary() (string, error) {
	allSkills, err := l.ListSkills(false)
	if err != nil || len(allSkills) == 0 {
		return "", err
	}

	lines := make([]string, 0, len(allSkills)+2)
	lines = append(lines, "<skills>")
	for _, skill := range allSkills {
		name := xmlEscaper.Replace(skill.Name)
		desc := xmlEscaper.Replace(skill.Description)
		path := skill.Path
		// TODO: determine availability based on metadata or other criteria
		available := "true"

		lines = append(lines, fmt.Sprintf(`  <skill available="%s">`, available))
		lines = append(lines, fmt.Sprintf(`   <name>%s</name>`, name))
		lines = append(lines, fmt.Sprintf(`   <description>%s</description>`, desc))
		lines = append(lines, fmt.Sprintf(`   <location>%s</location>`, path))

		// TODO: show missing requirements for unavailable skills
		lines = append(lines, `  </skill>`)
	}
	lines = append(lines, "</skills>")

	return strings.Join(lines, "\n"), nil
}
