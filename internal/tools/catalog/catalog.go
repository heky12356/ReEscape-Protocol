package catalog

import (
	"project-yume/internal/tools"
	toolaffection "project-yume/internal/tools/affection"
	toolimage "project-yume/internal/tools/image"
	toolmemory "project-yume/internal/tools/memory"
	toolsession "project-yume/internal/tools/session"
	tooltime "project-yume/internal/tools/time"
)

type Definition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ReadOnly    bool   `json:"readOnly"`
}

func NewRegistry() *tools.Registry {
	registry := tools.NewRegistry()
	registry.MustRegister(toolsession.NewGetStateTool())
	registry.MustRegister(toolmemory.NewGetMemoryContextTool())
	registry.MustRegister(toolmemory.NewRememberFactTool())
	registry.MustRegister(toolmemory.NewUpdateProfileTool())
	registry.MustRegister(toolimage.NewListAssetsTool())
	registry.MustRegister(tooltime.NewGetCurrentTimeContextTool())
	registry.MustRegister(toolaffection.NewGetAffectionTool())
	registry.MustRegister(toolaffection.NewUpdateAffectionTool())
	return registry
}

func ListDefinitions() []Definition {
	registry := NewRegistry()
	entries := registry.List()
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
