package catalog

import (
	"project-yume/internal/tools"
	toolaffection "project-yume/internal/tools/affection"
	toolimage "project-yume/internal/tools/image"
	toolmemory "project-yume/internal/tools/memory"
	toolscheduler "project-yume/internal/tools/scheduler"
	toolsession "project-yume/internal/tools/session"
	toolskill "project-yume/internal/tools/skill"
	tooltime "project-yume/internal/tools/time"
	toolweb "project-yume/internal/tools/web"
	"project-yume/internal/webaccess"
)

type Definition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ReadOnly    bool   `json:"readOnly"`
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
	registry.MustRegister(toolsession.NewGetStateTool())
	registry.MustRegister(toolscheduler.NewGetProactiveScheduleTool())
	registry.MustRegister(toolscheduler.NewUpdateProactiveScheduleTool())
	registry.MustRegister(toolmemory.NewGetMemoryContextTool())
	registry.MustRegister(toolmemory.NewRememberFactTool())
	registry.MustRegister(toolmemory.NewUpdateProfileTool())
	registry.MustRegister(toolimage.NewListAssetsTool())
	registry.MustRegister(tooltime.NewGetCurrentTimeContextTool())
	registry.MustRegister(toolaffection.NewGetAffectionTool())
	registry.MustRegister(toolaffection.NewUpdateAffectionTool())
	registry.MustRegister(toolskill.NewSearchSkillsTool())
	registry.MustRegister(toolskill.NewReadSkillTool())
	registry.MustRegister(toolskill.NewReadSkillResourceTool(optsSkillResourceMaxBytes(opts...)))
	if len(opts) > 0 && opts[0].EnableWebTools {
		if opts[0].SearchClient != nil {
			registry.MustRegister(toolweb.NewSearchTool(opts[0].SearchClient, opts[0].SearchMaxResults))
		}
		if opts[0].FetchClient != nil {
			registry.MustRegister(toolweb.NewFetchTool(opts[0].FetchClient, opts[0].FetchMaxChars))
		}
	}
	return registry
}

func optsSkillResourceMaxBytes(opts ...Options) int {
	if len(opts) == 0 {
		return 0
	}
	return opts[0].SkillResourceMaxBytes
}

func ListDefinitions(opts ...Options) []Definition {
	registry := NewRegistry(opts...)
	return definitionsFromTools(registry.List())
}

func ListDefinitionsWithPolicy(policy tools.Policy, opts ...Options) []Definition {
	registry := NewRegistry(opts...)
	return definitionsFromTools(registry.ListAvailable(policy))
}

func definitionsFromTools(entries []tools.Tool) []Definition {
	result := make([]Definition, 0, len(entries))
	for _, tool := range entries {
		result = append(result, Definition{
			Name:        tool.Name(),
			Description: tool.Description(),
			ReadOnly:    tool.ReadOnly(),
		})
	}
	return result
}
