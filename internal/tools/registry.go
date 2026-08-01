package tools

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) Register(tool Tool) error {
	if tool == nil {
		return fmt.Errorf("tool is nil")
	}
	name := normalizeToolName(tool.Name())
	if name == "" {
		return fmt.Errorf("tool name is empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool already registered: %s", name)
	}
	r.tools[name] = tool
	return nil
}

func (r *Registry) MustRegister(tool Tool) {
	if err := r.Register(tool); err != nil {
		panic(err)
	}
}

func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[normalizeToolName(name)]
	return tool, ok
}

func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]Tool, 0, len(names))
	for _, name := range names {
		result = append(result, r.tools[name])
	}
	return result
}

func (r *Registry) ListAvailable(policy Policy) []Tool {
	entries := r.List()
	result := make([]Tool, 0, len(entries))
	for _, tool := range entries {
		if !policy.IsAvailable(tool) {
			continue
		}
		result = append(result, tool)
	}
	return result
}

func normalizeToolName(name string) string {
	return strings.TrimSpace(name)
}
