package skill

import (
	"strings"
	"testing"
)

func TestParseSkillMarkdown(t *testing.T) {
	pkg, err := ParseSkillMarkdown("comfort", []byte("---\nname: comfort\ndescription: Use for support.\ntags:\n  - sadness\ncustom: value\n---\n\n# Comfort\n\n先承接情绪。"))
	if err != nil {
		t.Fatalf("parse skill: %v", err)
	}
	if pkg.Name != "comfort" || pkg.Description != "Use for support." {
		t.Fatalf("unexpected metadata: %#v", pkg)
	}
	if pkg.Frontmatter["custom"] != "value" {
		t.Fatalf("expected unknown frontmatter field to be preserved: %#v", pkg.Frontmatter)
	}
	if pkg.Body != "# Comfort\n\n先承接情绪。" {
		t.Fatalf("unexpected body: %q", pkg.Body)
	}
}

func TestParseSkillMarkdownRequiresNameAndDescription(t *testing.T) {
	for _, content := range []string{
		"---\ndescription: test\n---\nbody",
		"---\nname: test\n---\nbody",
	} {
		if _, err := ParseSkillMarkdown("test", []byte(content)); err == nil {
			t.Fatalf("expected required field error for %q", content)
		}
	}
}

func TestParseSkillMarkdownRequiresFrontmatter(t *testing.T) {
	_, err := ParseSkillMarkdown("test", []byte("# no frontmatter"))
	if err == nil || !strings.Contains(err.Error(), "frontmatter") {
		t.Fatalf("expected frontmatter error, got %v", err)
	}
}
