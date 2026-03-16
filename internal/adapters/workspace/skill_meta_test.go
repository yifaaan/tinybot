package workspace

import "testing"

func TestParseSkillMetadata_PlainMarkdown(t *testing.T) {
	content := "# Foo Skill\n\nUse foo skill wisely."

	meta, err := ParseSkillMetadata(content, "foo")
	if err != nil {
		t.Fatalf("ParseSkillMetadata() error: %v", err)
	}

	if meta.Name != "foo" {
		t.Fatalf("meta.Name = %q, want %q", meta.Name, "foo")
	}
}

func TestParseSkillMetadata_WithFrontMatter(t *testing.T) {
	content := `---
name: foo
description: test skill
---

# Foo
`

	meta, err := ParseSkillMetadata(content, "fallback")
	if err != nil {
		t.Fatalf("ParseSkillMetadata() error: %v", err)
	}

	if meta.Name != "foo" {
		t.Fatalf("meta.Name = %q, want %q", meta.Name, "foo")
	}
	if meta.Description != "test skill" {
		t.Fatalf("meta.Description = %q, want %q", meta.Description, "test skill")
	}
}

func TestParseSkillMetadata_FallbackName(t *testing.T) {
	content := `---
description: test skill
---

# Foo
`

	meta, err := ParseSkillMetadata(content, "fallback-name")
	if err != nil {
		t.Fatalf("ParseSkillMetadata() error: %v", err)
	}

	if meta.Name != "fallback-name" {
		t.Fatalf("meta.Name = %q, want %q", meta.Name, "fallback-name")
	}
}

func TestParseSkillMetadata_WithRequires(t *testing.T) {
	content := `---
name: github
description: GitHub operations
metadata: {"openclaw":{"requires":{"bins":["gh","git"],"env":["GITHUB_TOKEN"]}}}
---

# GitHub Skill
`

	meta, err := ParseSkillMetadata(content, "fallback")
	if err != nil {
		t.Fatalf("ParseSkillMetadata() error: %v", err)
	}

	if meta.Name != "github" {
		t.Fatalf("meta.Name = %q, want %q", meta.Name, "github")
	}
	if len(meta.RequiresBin) != 2 || meta.RequiresBin[0] != "gh" || meta.RequiresBin[1] != "git" {
		t.Fatalf("meta.RequiresBin = %v, want [gh git]", meta.RequiresBin)
	}
	if len(meta.RequiresEnv) != 1 || meta.RequiresEnv[0] != "GITHUB_TOKEN" {
		t.Fatalf("meta.RequiresEnv = %v, want [GITHUB_TOKEN]", meta.RequiresEnv)
	}
}

func TestParseSkillMetadata_RequiresOnlyBins(t *testing.T) {
	content := `---
name: weather
metadata: {"openclaw":{"requires":{"bins":["curl"]}}}
---
`

	meta, err := ParseSkillMetadata(content, "fallback")
	if err != nil {
		t.Fatalf("ParseSkillMetadata() error: %v", err)
	}

	if len(meta.RequiresBin) != 1 || meta.RequiresBin[0] != "curl" {
		t.Fatalf("meta.RequiresBin = %v, want [curl]", meta.RequiresBin)
	}
	if len(meta.RequiresEnv) != 0 {
		t.Fatalf("meta.RequiresEnv = %v, want []", meta.RequiresEnv)
	}
}

func TestParseSkillMetadata_RequiresOnlyEnv(t *testing.T) {
	content := `---
name: api
metadata: {"openclaw":{"requires":{"env":["API_KEY","API_SECRET"]}}}
---
`

	meta, err := ParseSkillMetadata(content, "fallback")
	if err != nil {
		t.Fatalf("ParseSkillMetadata() error: %v", err)
	}

	if len(meta.RequiresEnv) != 2 || meta.RequiresEnv[0] != "API_KEY" || meta.RequiresEnv[1] != "API_SECRET" {
		t.Fatalf("meta.RequiresEnv = %v, want [API_KEY API_SECRET]", meta.RequiresEnv)
	}
	if len(meta.RequiresBin) != 0 {
		t.Fatalf("meta.RequiresBin = %v, want []", meta.RequiresBin)
	}
}
