package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

var frontMatterRe = regexp.MustCompile(`(?s)^---\r?\n(.*?)\r?\n---`)

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
	Name        string // skill name, e.g. "foo"
	Path        string // absolute path to SKILL.md
	Source      string // workspace or builtin
	Description string
	Available   bool
	Always      bool   // if true, always load full content in context; otherwise use progressive loading
	MetadataRaw string // raw metadata content (e.g. from SKILL.md) for future use
	RequiresEnv []string
	RequiresBin []string
	Missing     []string
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
			skillInfo, err := l.buildSkillInfo(skillDir.Name())
			if err != nil {
				continue
			}
			skillInfo.Source = "workspace"
			skillInfo.Path = skillFile
			skillInfo.Available = true // TODO: determine availability based on metadata or other criteria
			skills = append(skills, skillInfo)

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
			skillInfo, err := l.buildSkillInfo(skillDir.Name())
			if err != nil {
				continue
			}
			skillInfo.Source = "builtin"
			skillInfo.Path = skillFile
			skillInfo.Available = true // TODO: determine availability based on metadata or other criteria
			skills = append(skills, skillInfo)
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

		lines = append(lines, fmt.Sprintf(`  <skill available="%s">`, strconv.FormatBool(skill.Available)))
		lines = append(lines, fmt.Sprintf(`   <name>%s</name>`, name))
		lines = append(lines, fmt.Sprintf(`   <description>%s</description>`, desc))
		lines = append(lines, fmt.Sprintf(`   <location>%s</location>`, path))

		if !skill.Available && len(skill.Missing) > 0 {
			lines = append(lines, fmt.Sprintf(`   <requires>%s</requires>`, xmlEscaper.Replace(strings.Join(skill.Missing, ", "))))
		}
		lines = append(lines, `   </skill>`)
	}
	lines = append(lines, "</skills>")

	return strings.Join(lines, "\n"), nil
}

func (l *SkillsLoader) buildSkillInfo(name string) (SkillInfo, error) {
	skillInfo, err := l.getSkillMetadata(name)
	if err != nil {
		return SkillInfo{}, err
	}
	skillInfo.Available = l.checkRequirements(&skillInfo)
	return skillInfo, nil
}

func (l *SkillsLoader) GetAlwaysSkills() ([]string, error) {
	skill, err := l.ListSkills(true)
	if err != nil {
		return nil, err
	}
	alwaysSkills := make([]string, 0, len(skill))
	for _, skill := range skill {
		if skill.Always {
			alwaysSkills = append(alwaysSkills, skill.Name)
		}
	}
	return alwaysSkills, nil
}

func (l *SkillsLoader) checkRequirements(_ *SkillInfo) bool {
	// TODO: check environment variables and binaries for skill availability
	return true
}

func (l *SkillsLoader) getSkillMetadata(name string) (SkillInfo, error) {
	content, err := l.LoadSkill(name)
	if err != nil {
		return SkillInfo{}, fmt.Errorf("parse kill metadata: %w", err)
	}
	if content == "" || !strings.HasPrefix(content, "---") {
		return SkillInfo{}, errors.New("empty skill content")
	}

	matches := frontMatterRe.FindStringSubmatch(content)
	if len(matches) < 2 {
		return SkillInfo{}, errors.New("invalid skill metadata format")
	}
	raw := matches[1]
	skillInfo := SkillInfo{}
	// YAML parsing
	for _, line := range strings.Split(raw, "\n") {
		if strings.Contains(line, ":") {
			kv := strings.SplitN(line, ":", 1)
			k := kv[0]
			v := kv[1]
			switch k {
			case "description":
				skillInfo.Description = v
			case "name":
				skillInfo.Name = v
			case "metadata":
				skillInfo.MetadataRaw = v

				meta := make(map[string]any)
				_ = json.Unmarshal([]byte(v), &meta)
				skillInfo.Always = false
				if always, ok := meta["always"].(bool); ok && always {
					skillInfo.Always = true
				}

				if openclawMeta, ok := meta["openclaw"].(map[string]any); ok {
					if always, ok := openclawMeta["always"].(bool); ok && always {
						skillInfo.Always = true
					}
				}
			}
		}
	}
	return skillInfo, nil
}

// parseOPenclawMetadata parse openclaw metadata JSON from SKILL.md front matter (if exists) and return as a map for future use
func (l *SkillsLoader) parseOpenclawMetadata(raw string) map[string]any {
	data := make(map[string]any)
	_ = json.Unmarshal([]byte(raw), &data)

	return data["openclaw"].(map[string]any)
}
