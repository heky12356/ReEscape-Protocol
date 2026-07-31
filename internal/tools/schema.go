package tools

import openai "github.com/sashabaranov/go-openai"

type Schema struct {
	Type                 string              `json:"type"`
	Properties           map[string]Property `json:"properties,omitempty"`
	Required             []string            `json:"required,omitempty"`
	AdditionalProperties bool                `json:"additionalProperties"`
}

type Property struct {
	Type        string    `json:"type,omitempty"`
	Description string    `json:"description,omitempty"`
	Enum        []string  `json:"enum,omitempty"`
	Items       *Property `json:"items,omitempty"`
}

func EmptyObjectSchema() Schema {
	return Schema{
		Type:                 "object",
		Properties:           map[string]Property{},
		AdditionalProperties: false,
	}
}

func ToOpenAITools(registry *Registry) []openai.Tool {
	if registry == nil {
		return nil
	}

	entries := registry.List()
	result := make([]openai.Tool, 0, len(entries))
	for _, tool := range entries {
		schema := tool.Schema()
		if schema.Type == "" {
			schema = EmptyObjectSchema()
		}
		result = append(result, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        tool.Name(),
				Description: tool.Description(),
				Parameters:  schema,
			},
		})
	}
	return result
}
