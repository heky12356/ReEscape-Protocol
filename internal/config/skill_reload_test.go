package config

import (
	"os"
	"path/filepath"
	"testing"

	"project-yume/internal/skill"
)

func TestReloadSkillsFromConfigLoadsConfiguredSkillDirs(t *testing.T) {
	cfg := GetConfig()
	previousEnabled := cfg.EnableSkills
	previousDirs := append([]string(nil), cfg.SkillDirs...)
	previousLoadSystem := cfg.SkillLoadSystem
	t.Cleanup(func() {
		cfg.EnableSkills = previousEnabled
		cfg.SkillDirs = previousDirs
		cfg.SkillLoadSystem = previousLoadSystem
		reloadSkillsFromConfig()
	})

	root := t.TempDir()
	skillDir := filepath.Join(root, "startup-load")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte("---\nname: startup-load\ndescription: Use when testing startup skill loading.\ntags: [startup-load-marker]\n---\n# Startup Load\n")
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg.EnableSkills = true
	cfg.SkillDirs = []string{root}
	cfg.SkillLoadSystem = false
	reloadSkillsFromConfig()

	matches := skill.GetManager().Match(skill.MatchInput{
		Message: "startup-load-marker",
		Limit:   3,
	})
	if len(matches) == 0 || matches[0].Name != "startup-load" {
		t.Fatalf("expected configured skill dir to be loaded, got %#v", matches)
	}
}
