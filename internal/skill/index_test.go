package skill

import (
	"path/filepath"
	"strings"
	"testing"

	"project-yume/internal/state"
)

func TestManagerListSearchAndRead(t *testing.T) {
	root := t.TempDir()
	writeSkillFile(t, filepath.Join(root, "comfort", "SKILL.md"), "---\nname: comfort\ndescription: Use when the user is sad or overwhelmed.\ntags: [sad, support]\n---\n# Comfort")
	writeSkillFile(t, filepath.Join(root, "schedule", "SKILL.md"), "---\nname: schedule\ndescription: Use for future reminders.\ntriggers: [proactive, reminder]\n---\n# Schedule")

	manager := &Manager{}
	if errs := manager.LoadDirs([]string{root}); len(errs) != 0 {
		t.Fatalf("load skills: %v", errs)
	}
	if len(manager.List()) != 2 {
		t.Fatalf("expected two skills, got %#v", manager.List())
	}
	matches := manager.Match(MatchInput{
		Message:       "I feel sad",
		DialogueState: state.DialogueState{Emotion: "sad"},
		Limit:         1,
	})
	if len(matches) != 1 || matches[0].Name != "comfort" {
		t.Fatalf("unexpected matches: %#v", matches)
	}
	body, err := manager.ReadSkill("comfort")
	if err != nil || !strings.Contains(body, "# Comfort") {
		t.Fatalf("unexpected body: %q, err=%v", body, err)
	}
}

func TestManagerReadResourceRejectsTraversalAndLargeFiles(t *testing.T) {
	root := t.TempDir()
	writeSkillFile(t, filepath.Join(root, "comfort", "SKILL.md"), "---\nname: comfort\ndescription: support\n---\nbody")
	writeSkillFile(t, filepath.Join(root, "comfort", "references", "example.md"), "reference")
	writeSkillFile(t, filepath.Join(root, "outside.txt"), "outside")

	manager := &Manager{}
	if errs := manager.LoadDirs([]string{root}); len(errs) != 0 {
		t.Fatalf("load skills: %v", errs)
	}
	content, err := manager.ReadResource("comfort", "references/example.md", 100)
	if err != nil || string(content) != "reference" {
		t.Fatalf("unexpected resource: %q, err=%v", content, err)
	}
	if _, err := manager.ReadResource("comfort", "../outside.txt", 100); err == nil {
		t.Fatal("expected path traversal to be rejected")
	}
	if _, err := manager.ReadResource("comfort", "references/example.md", 2); err == nil {
		t.Fatal("expected large resource to be rejected")
	}
}
