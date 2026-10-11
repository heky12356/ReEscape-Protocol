package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

type Registry struct {
	mu        sync.RWMutex
	entries   map[string]ToolEntry
	conflicts []ToolConflict
}

func NewRegistry() *Registry {
	return &Registry{entries: make(map[string]ToolEntry)}
}

func (r *Registry) Register(tool Tool) error {
	return r.RegisterWithMetadata(tool, ToolMetadata{
		Provider: "core",
		Origin:   "go",
		Version:  "builtin",
	})
}

func (r *Registry) RegisterWithMetadata(tool Tool, metadata ToolMetadata) error {
	if tool == nil {
		return fmt.Errorf("tool is nil")
	}
	name := normalizeToolName(tool.Name())
	if name == "" {
		return fmt.Errorf("tool name is empty")
	}
	metadata = normalizeToolMetadata(metadata)

	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, exists := r.entries[name]; exists {
		reason := fmt.Sprintf("tool already registered: %s", name)
		metadata.ConflictReason = reason
		r.conflicts = append(r.conflicts, ToolConflict{
			Name:     name,
			Existing: existing.Metadata,
			Rejected: metadata,
			Reason:   reason,
		})
		return fmt.Errorf("%s", reason)
	}
	r.entries[name] = ToolEntry{Tool: tool, Metadata: metadata}
	return nil
}

func (r *Registry) RegisterProvider(ctx context.Context, provider ToolProvider) error {
	if provider == nil {
		return fmt.Errorf("tool provider is nil")
	}
	providerName := strings.TrimSpace(provider.Name())
	origin := strings.TrimSpace(provider.Origin())
	version := strings.TrimSpace(provider.Version())
	if providerName == "" {
		return fmt.Errorf("tool provider name is empty")
	}
	if origin == "" {
		return fmt.Errorf("tool provider origin is empty: %s", providerName)
	}
	items, err := provider.List(ctx)
	if err != nil {
		return fmt.Errorf("list tools from provider %s: %w", providerName, err)
	}
	var firstErr error
	for _, tool := range items {
		if err := r.RegisterWithMetadata(tool, ToolMetadata{
			Provider: providerName,
			Origin:   origin,
			Version:  version,
		}); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (r *Registry) MustRegister(tool Tool) {
	if err := r.Register(tool); err != nil {
		panic(err)
	}
}

func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.entries[normalizeToolName(name)]
	return entry.Tool, ok
}

func (r *Registry) Metadata(name string) (ToolMetadata, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.entries[normalizeToolName(name)]
	if !ok {
		return ToolMetadata{}, false
	}
	return entry.Metadata, true
}

func (r *Registry) ListEntries() []ToolEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.entries))
	for name := range r.entries {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]ToolEntry, 0, len(names))
	for _, name := range names {
		result = append(result, r.entries[name])
	}
	return result
}

func (r *Registry) Conflicts() []ToolConflict {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]ToolConflict, len(r.conflicts))
	copy(result, r.conflicts)
	return result
}

func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.entries))
	for name := range r.entries {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]Tool, 0, len(names))
	for _, name := range names {
		result = append(result, r.entries[name].Tool)
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

func normalizeToolMetadata(metadata ToolMetadata) ToolMetadata {
	if strings.TrimSpace(metadata.Provider) == "" {
		metadata.Provider = "core"
	}
	if strings.TrimSpace(metadata.Origin) == "" {
		metadata.Origin = "go"
	}
	metadata.Provider = strings.TrimSpace(metadata.Provider)
	metadata.Origin = strings.TrimSpace(metadata.Origin)
	metadata.Version = strings.TrimSpace(metadata.Version)
	metadata.ConflictReason = strings.TrimSpace(metadata.ConflictReason)
	return metadata
}
