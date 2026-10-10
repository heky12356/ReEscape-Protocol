package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type Manager struct {
	mu       sync.RWMutex
	packages map[string]Package
}

var manager = &Manager{packages: make(map[string]Package)}

func GetManager() *Manager {
	return manager
}

func (m *Manager) LoadDirs(dirs []string) []error {
	return m.LoadDirsWithOptions(dirs, LoadOptions{Scope: ScopeProject})
}

func (m *Manager) LoadDirsWithOptions(dirs []string, opts LoadOptions) []error {
	packages, errs := LoadDirs(dirs, opts)
	next := make(map[string]Package, len(packages))
	for _, pkg := range packages {
		next[pkg.Name] = clonePackage(pkg)
	}

	m.mu.Lock()
	m.packages = next
	m.mu.Unlock()
	return errs
}

// ReplacePackages swaps the active package index after a caller has finished
// validating a candidate configuration. Packages are cloned so later caller
// mutations cannot alter the manager's runtime view.
func (m *Manager) ReplacePackages(packages []Package) {
	next := make(map[string]Package, len(packages))
	for _, pkg := range packages {
		next[pkg.Name] = clonePackage(pkg)
	}
	m.mu.Lock()
	m.packages = next
	m.mu.Unlock()
}

func (m *Manager) List() []Package {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Package, 0, len(m.packages))
	for _, pkg := range m.packages {
		result = append(result, clonePackage(pkg))
	}
	sortPackages(result)
	return result
}

func (m *Manager) Get(name string) (Package, bool) {
	name = strings.TrimSpace(name)
	m.mu.RLock()
	defer m.mu.RUnlock()
	pkg, ok := m.packages[name]
	if !ok {
		return Package{}, false
	}
	return clonePackage(pkg), true
}

func (m *Manager) Search(query string, limit int) []Match {
	return m.Match(MatchInput{Message: query, Limit: limit, MinScore: 2})
}

func (m *Manager) Match(input MatchInput) []Match {
	m.mu.RLock()
	packages := make([]Package, 0, len(m.packages))
	for _, pkg := range m.packages {
		packages = append(packages, clonePackage(pkg))
	}
	m.mu.RUnlock()
	return MatchPackages(packages, input)
}

func (m *Manager) ReadSkill(name string) (string, error) {
	pkg, ok := m.Get(name)
	if !ok {
		return "", fmt.Errorf("skill not found: %s", strings.TrimSpace(name))
	}
	return pkg.Body, nil
}

func (m *Manager) ReadResource(name string, relativePath string, maxBytes int) ([]byte, error) {
	pkg, ok := m.Get(name)
	if !ok {
		return nil, fmt.Errorf("skill not found: %s", strings.TrimSpace(name))
	}
	return ReadResource(pkg, relativePath, maxBytes)
}

func (m *Manager) Resources(name string) ([]string, error) {
	pkg, ok := m.Get(name)
	if !ok {
		return nil, fmt.Errorf("skill not found: %s", strings.TrimSpace(name))
	}
	return ListResources(pkg)
}

func clonePackage(pkg Package) Package {
	frontmatter := make(map[string]any, len(pkg.Frontmatter))
	for key, value := range pkg.Frontmatter {
		frontmatter[key] = value
	}
	pkg.Frontmatter = frontmatter
	return pkg
}

func sortPackages(packages []Package) {
	sort.Slice(packages, func(i, j int) bool {
		if packages[i].Name == packages[j].Name {
			return packages[i].RootDir < packages[j].RootDir
		}
		return packages[i].Name < packages[j].Name
	})
}

func IsSkillRoot(path string) bool {
	info, err := os.Stat(filepath.Join(path, "SKILL.md"))
	return err == nil && !info.IsDir()
}
