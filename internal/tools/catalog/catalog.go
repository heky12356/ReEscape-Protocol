package catalog

import (
	"context"
	"fmt"

	"project-yume/internal/tools"
	toolaffection "project-yume/internal/tools/affection"
	toolimage "project-yume/internal/tools/image"
	toolintent "project-yume/internal/tools/intent"
	toolmemory "project-yume/internal/tools/memory"
	toolscheduler "project-yume/internal/tools/scheduler"
	toolsession "project-yume/internal/tools/session"
	toolskill "project-yume/internal/tools/skill"
	tooltime "project-yume/internal/tools/time"
	toolweb "project-yume/internal/tools/web"
	"project-yume/internal/webaccess"
)

type Definition struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	ReadOnly       bool   `json:"readOnly"`
	Provider       string `json:"provider"`
	Origin         string `json:"origin"`
	Version        string `json:"version,omitempty"`
	ConflictReason string `json:"conflictReason,omitempty"`
}

type Options struct {
	EnableWebTools        bool
	SearchClient          webaccess.SearchClient
	FetchClient           webaccess.FetchClient
	SearchMaxResults      int
	FetchMaxChars         int
	SkillResourceMaxBytes int
}

func NewRegistry(opts ...Options) *tools.Registry {
	registry := tools.NewRegistry()
	coreTools := []tools.Tool{
		toolsession.NewGetStateTool(),
		toolscheduler.NewGetProactiveScheduleTool(),
		toolscheduler.NewUpdateProactiveScheduleTool(),
		toolintent.NewGetIntentTool(),
		toolintent.NewUpdateIntentTool(),
		toolintent.NewGetOpenLoopTool(),
		toolintent.NewUpdateOpenLoopTool(),
		toolmemory.NewGetMemoryContextTool(),
		toolmemory.NewRememberFactTool(),
		toolmemory.NewUpdateProfileTool(),
		toolimage.NewListAssetsTool(),
		toolimage.NewSaveAssetTool(),
		tooltime.NewGetCurrentTimeContextTool(),
		toolaffection.NewGetAffectionTool(),
		toolaffection.NewUpdateAffectionTool(),
	}
	mustRegisterProvider(registry, staticProvider{name: "core", origin: "go", version: "builtin", tools: coreTools})
	mustRegisterProvider(registry, staticProvider{
		name: "skill", origin: "skill", version: "builtin",
		tools: []tools.Tool{
			toolskill.NewSearchSkillsTool(),
			toolskill.NewReadSkillTool(),
			toolskill.NewReadSkillResourceTool(optsSkillResourceMaxBytes(opts...)),
		},
	})
	if len(opts) > 0 && opts[0].EnableWebTools {
		webTools := make([]tools.Tool, 0, 2)
		if opts[0].SearchClient != nil {
			webTools = append(webTools, toolweb.NewSearchTool(opts[0].SearchClient, opts[0].SearchMaxResults))
		}
		if opts[0].FetchClient != nil {
			webTools = append(webTools, toolweb.NewFetchTool(opts[0].FetchClient, opts[0].FetchMaxChars))
		}
		mustRegisterProvider(registry, staticProvider{name: "web", origin: "external", version: "configured", tools: webTools})
	}
	return registry
}

type staticProvider struct {
	name    string
	origin  string
	version string
	tools   []tools.Tool
}

func (p staticProvider) Name() string    { return p.name }
func (p staticProvider) Origin() string  { return p.origin }
func (p staticProvider) Version() string { return p.version }
func (p staticProvider) Close() error    { return nil }
func (p staticProvider) List(context.Context) ([]tools.Tool, error) {
	return append([]tools.Tool(nil), p.tools...), nil
}

func mustRegisterProvider(registry *tools.Registry, provider tools.ToolProvider) {
	if err := registry.RegisterProvider(context.Background(), provider); err != nil {
		panic(fmt.Sprintf("register tool provider %s: %v", provider.Name(), err))
	}
}

func optsSkillResourceMaxBytes(opts ...Options) int {
	if len(opts) == 0 {
		return 0
	}
	return opts[0].SkillResourceMaxBytes
}

func ListDefinitions(opts ...Options) []Definition {
	registry := NewRegistry(opts...)
	return definitionsFromRegistry(registry, nil)
}

func ListDefinitionsWithPolicy(policy tools.Policy, opts ...Options) []Definition {
	registry := NewRegistry(opts...)
	return definitionsFromRegistry(registry, &policy)
}

func definitionsFromRegistry(registry *tools.Registry, policy *tools.Policy) []Definition {
	entries := registry.ListEntries()
	result := make([]Definition, 0, len(entries))
	for _, entry := range entries {
		tool := entry.Tool
		if policy != nil && !policy.IsAvailable(tool) {
			continue
		}
		result = append(result, Definition{
			Name:           tool.Name(),
			Description:    tool.Description(),
			ReadOnly:       tool.ReadOnly(),
			Provider:       entry.Metadata.Provider,
			Origin:         entry.Metadata.Origin,
			Version:        entry.Metadata.Version,
			ConflictReason: entry.Metadata.ConflictReason,
		})
	}
	return result
}
