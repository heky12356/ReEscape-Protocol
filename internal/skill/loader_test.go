package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDirsScansSkillRootsAndSkipsInvalidAndHidden(t *testing.T) {
	root := t.TempDir()
	writeSkillFile(t, filepath.Join(root, "comfort", "SKILL.md"), "---\nname: comfort\ndescription: emotional support\n---\nbody")
	writeSkillFile(t, filepath.Join(root, "invalid", "SKILL.md"), "---\nname: invalid\n---\nbody")
	writeSkillFile(t, filepath.Join(root, ".system", "SKILL.md"), "---\nname: system\ndescription: system skill\n---\nbody")

	packages, errs := LoadDirs([]string{root}, LoadOptions{Scope: ScopeProject})
	if len(packages) != 1 || packages[0].Name != "comfort" {
		t.Fatalf("unexpected packages: %#v", packages)
	}
	if len(errs) != 1 {
		t.Fatalf("expected one invalid skill error, got %v", errs)
	}
}

func TestLoadDirsAcceptsDirectoryItselfAsSkill(t *testing.T) {
	root := t.TempDir()
	writeSkillFile(t, filepath.Join(root, "SKILL.md"), "---\nname: root\ndescription: root skill\n---\nbody")

	packages, errs := LoadDirs([]string{root}, LoadOptions{Scope: ScopeGlobal})
	if len(errs) != 0 || len(packages) != 1 || packages[0].Scope != ScopeGlobal {
		t.Fatalf("unexpected load result: packages=%#v errs=%v", packages, errs)
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
