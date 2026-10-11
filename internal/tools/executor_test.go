package tools

import (
	"context"
	"testing"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

func TestExecutorCarriesToolProviderMetadata(t *testing.T) {
	registry := NewRegistry()
	tool := policyTestTool{name: "external_search", readOnly: true}
	if err := registry.RegisterWithMetadata(tool, ToolMetadata{
		Provider: "web",
		Origin:   "external",
		Version:  "v2",
	}); err != nil {
		t.Fatal(err)
	}

	execution := NewExecutor(registry, Policy{}, time.Second).Execute(context.Background(), nil, openai.ToolCall{
		ID:   "call-1",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "external_search",
			Arguments: `{}`,
		},
	})
	if execution.Error != "" {
		t.Fatalf("unexpected execution error: %s", execution.Error)
	}
	if execution.ToolProvider != "web" || execution.ToolOrigin != "external" || execution.ToolVersion != "v2" {
		t.Fatalf("tool metadata was not carried to execution: %#v", execution)
	}
}
