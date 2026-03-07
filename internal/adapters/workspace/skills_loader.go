package workspace

type SkillInfo struct {
	Name        string
	Path        string
	Source      string
	Description string
	Available   bool
}

type SkillsLoader struct {
	workspace  string
	builtinDir string
}

func NewSkillsLoader(workspace string, builtinDir string) *SkillsLoader {
	// TODO: initialize workspace and builtin skill roots
	return &SkillsLoader{
		workspace:  workspace,
		builtinDir: builtinDir,
	}
}

func (l *SkillsLoader) ListSkills(filterUnavailable bool) ([]SkillInfo, error) {
	// TODO: discover skills under workspace/builtin directories
	return nil, nil
}

func (l *SkillsLoader) LoadSkill(name string) (string, error) {
	// TODO: load SKILL.md content by skill name
	return "", nil
}

func (l *SkillsLoader) LoadSkillsForContext(names []string) (string, error) {
	// TODO: load selected skills and format them for context injection
	return "", nil
}

func (l *SkillsLoader) BuildSummary() (string, error) {
	// TODO: build a summary block for all discoverable skills
	return "", nil
}
