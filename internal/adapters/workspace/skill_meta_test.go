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
