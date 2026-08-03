package skill

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	skillpkg "project-yume/internal/skill"
)

func TestSearchSkillsToolReturnsMatches(t *testing.T) {
	root := t.TempDir()
	writeSkillFile(t, filepath.Join(root, "comfort", "SKILL.md"), "---\nname: comfort\ndescription: Use when user is sad.\n---\nbody")
	if errs := skillpkg.GetManager().LoadDirs([]string{root}); len(errs) != 0 {
		t.Fatalf("load skills: %v", errs)
	}
	t.Cleanup(func() { skillpkg.GetManager().LoadDirs(nil) })

	result, err := NewSearchSkillsTool().Execute(context.Background(), nil, json.RawMessage(`{"query":"sad","reason":"test"}`))
	if err != nil {
		t.Fatalf("execute search_skills: %v", err)
	}
	if !strings.Contains(result.Content, "comfort") {
		t.Fatalf("expected comfort match, got %q", result.Content)
	}
}

func TestReadSkillToolReturnsBodyAndResources(t *testing.T) {
	root := t.TempDir()
	writeSkillFile(t, filepath.Join(root, "comfort", "SKILL.md"), "---\nname: comfort\ndescription: Use when user is sad.\n---\n# Comfort")
	writeSkillFile(t, filepath.Join(root, "comfort", "references", "examples.md"), "example")
	if errs := skillpkg.GetManager().LoadDirs([]string{root}); len(errs) != 0 {
		t.Fatalf("load skills: %v", errs)
	}
	t.Cleanup(func() { skillpkg.GetManager().LoadDirs(nil) })

	result, err := NewReadSkillTool().Execute(context.Background(), nil, json.RawMessage(`{"name":"comfort","reason":"test"}`))
	if err != nil {
		t.Fatalf("execute read_skill: %v", err)
	}
	if !strings.Contains(result.Content, "# Comfort") || !strings.Contains(result.Content, "references/examples.md") {
		t.Fatalf("unexpected content: %q", result.Content)
	}
}

func TestReadSkillResourceToolRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	writeSkillFile(t, filepath.Join(root, "comfort", "SKILL.md"), "---\nname: comfort\ndescription: Use when user is sad.\n---\nbody")
	writeSkillFile(t, filepath.Join(root, "outside.txt"), "outside")
	if errs := skillpkg.GetManager().LoadDirs([]string{root}); len(errs) != 0 {
		t.Fatalf("load skills: %v", errs)
	}
	t.Cleanup(func() { skillpkg.GetManager().LoadDirs(nil) })

	_, err := NewReadSkillResourceTool(64).Execute(context.Background(), nil, json.RawMessage(`{"name":"comfort","path":"../outside.txt","reason":"test"}`))
	if err == nil {
		t.Fatal("expected traversal error")
	}
}

func writeSkillFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
