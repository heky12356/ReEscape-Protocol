package skill

import (
	"strings"
	"testing"
)

func TestFormatHintsOnlyIncludesMetadata(t *testing.T) {
	content := FormatHints([]Match{{Name: "comfort", Description: "Use for sadness.", Score: 4}})
	if !strings.Contains(content, "【Skill Hints】") || !strings.Contains(content, "- comfort: Use for sadness.") {
		t.Fatalf("unexpected hints: %q", content)
	}
	if strings.Contains(content, "Score") {
		t.Fatalf("did not expect internal score in hints: %q", content)
	}
}
