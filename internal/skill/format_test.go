package skill

import (
	"strings"
	"testing"
)

func TestFormatHintsOnlyIncludesMetadata(t *testing.T) {
	content := FormatHints([]Match{{Name: "comfort", Description: "Use for sadness.", Score: 4}})
	if !strings.Contains(content, "【Skill Candidates】") || !strings.Contains(content, "- comfort: Use for sadness.") {
		t.Fatalf("unexpected hints: %q", content)
	}
	if strings.Contains(content, "Score") {
		t.Fatalf("did not expect internal score in hints: %q", content)
	}
	if !strings.Contains(content, "先调用 read_skill") {
		t.Fatalf("expected candidates to require read_skill, got %q", content)
	}
}

func TestFormatActivatedSkillsIncludesBody(t *testing.T) {
	content := FormatActivatedSkills([]ActivatedSkill{{
		Match: Match{Name: "comfort"},
		Body:  "# Comfort\nFollow the workflow.",
	}})
	if !strings.Contains(content, "【Activated Skills】") ||
		!strings.Contains(content, "【Skill: comfort】") ||
		!strings.Contains(content, "Follow the workflow.") {
		t.Fatalf("unexpected activated skills: %q", content)
	}
}
