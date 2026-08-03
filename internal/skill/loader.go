package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type LoadOptions struct {
	Scope         Scope
	IncludeHidden bool
}

func LoadDirs(dirs []string, opts LoadOptions) ([]Package, []error) {
	var packages []Package
	var errs []error

	for _, rawDir := range dirs {
		dir := strings.TrimSpace(rawDir)
		if dir == "" {
			continue
		}
		dir, err := filepath.Abs(filepath.Clean(dir))
		if err != nil {
			errs = append(errs, fmt.Errorf("resolve skill dir %q: %w", rawDir, err))
			continue
		}

		info, err := os.Stat(dir)
		if err != nil {
			errs = append(errs, fmt.Errorf("stat skill dir %q: %w", rawDir, err))
			continue
		}
		if !info.IsDir() {
			errs = append(errs, fmt.Errorf("skill path %q is not a directory", rawDir))
			continue
		}

		candidates, candidateErrs := skillCandidates(dir, opts.IncludeHidden)
		errs = append(errs, candidateErrs...)
		for _, skillFile := range candidates {
			content, readErr := os.ReadFile(skillFile)
			if readErr != nil {
				errs = append(errs, fmt.Errorf("read skill %q: %w", skillFile, readErr))
				continue
			}

			pkg, parseErr := ParseSkillMarkdown(filepath.Dir(skillFile), content)
			if parseErr != nil {
				errs = append(errs, fmt.Errorf("load skill %q: %w", skillFile, parseErr))
				continue
			}
			pkg.Scope = opts.Scope
			pkg.RootDir = filepath.Clean(pkg.RootDir)
			packages = append(packages, pkg)
		}
	}

	sort.SliceStable(packages, func(i, j int) bool {
		if packages[i].Name == packages[j].Name {
			return packages[i].RootDir < packages[j].RootDir
		}
		return packages[i].Name < packages[j].Name
	})
	return packages, errs
}

func skillCandidates(dir string, includeHidden bool) ([]string, []error) {
	direct := filepath.Join(dir, "SKILL.md")
	if info, err := os.Stat(direct); err == nil && !info.IsDir() {
		return []string{direct}, nil
	} else if err != nil && !os.IsNotExist(err) {
		return nil, []error{fmt.Errorf("stat skill file %q: %w", direct, err)}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, []error{fmt.Errorf("read skill directory %q: %w", dir, err)}
	}

	var candidates []string
	var errs []error
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if !includeHidden && strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		skillFile := filepath.Join(dir, entry.Name(), "SKILL.md")
		if info, statErr := os.Stat(skillFile); statErr == nil && !info.IsDir() {
			candidates = append(candidates, skillFile)
		} else if statErr != nil && !os.IsNotExist(statErr) {
			errs = append(errs, fmt.Errorf("stat skill file %q: %w", skillFile, statErr))
		}
	}
	sort.Strings(candidates)
	return candidates, errs
}
