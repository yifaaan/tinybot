package workspace

import (
	"encoding/json"
	"errors"
	"strings"
)

type SkillMetadata struct {
	Name        string
	Description string
	Always      bool // if true, always load full content in context; otherwise use progressive loading
}

func ParseSkillMetadata(content string, fallbackName string) (SkillMetadata, error) {
	meta := SkillMetadata{
		Name: strings.TrimSpace(fallbackName),
	}

	if content == "" {
		return meta, errors.New("empty skill content")
	}

	if !strings.HasPrefix(content, "---") {
		// 没有 front matter，使用默认�?
		return meta, nil
	}

	frontMatter, _, err := splitFrontMatter(content)
	if err != nil {
		return meta, nil
	}

	parsed, err := parseFrontMatter(frontMatter)
	if err != nil {
		return meta, nil
	}
	if parsed.Name != "" {
		meta.Name = parsed.Name
	}
	meta.Description = parsed.Description
	meta.Always = parsed.Always
	return meta, nil
}

// splitFrontMatter splits the content into front matter and body. If no front matter is found, the entire content is returned as body.
func splitFrontMatter(content string) (frontMatter string, body string, err error) {
	matches := reFrontMatter.FindStringSubmatch(content)
	if len(matches) < 3 {
		return "", content, nil
	}
	frontMatter = matches[1]
	body = matches[2]
	return frontMatter, body, nil
}

// parrseFrontMatter is a helper function to parse front matter content and return SkillMetadata. It is used when we only want to parse metadata without loading the full skill content.
func parseFrontMatter(frontMatter string) (SkillMetadata, error) {
	skillMeta := SkillMetadata{}
	// YAML parsing
	for _, line := range strings.Split(frontMatter, "\n") {
		if strings.Contains(line, ":") {
			kv := strings.SplitN(line, ":", 2)
			k := strings.TrimSpace(kv[0])

			v := strings.TrimSpace(kv[1])
			switch k {
			case "description":
				skillMeta.Description = v
			case "name":
				skillMeta.Name = v
			case "metadata":

				meta := make(map[string]any)
				_ = json.Unmarshal([]byte(v), &meta)
				skillMeta.Always = false
				if always, ok := meta["always"].(bool); ok && always {
					skillMeta.Always = true
				}

				if openclawMeta, ok := meta["openclaw"].(map[string]any); ok {
					if always, ok := openclawMeta["always"].(bool); ok && always {
						skillMeta.Always = true
					}
				}
			}
		}
	}
	return skillMeta, nil
}
