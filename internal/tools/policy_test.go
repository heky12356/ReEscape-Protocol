package tools

import (
	"context"
	"encoding/json"
	"testing"
)

type policyTestTool struct {
	name     string
	readOnly bool
}

func (t policyTestTool) Name() string {
	return t.name
}

func (t policyTestTool) Description() string {
	return t.name
}

func (t policyTestTool) Schema() Schema {
	return EmptyObjectSchema()
}

func (t policyTestTool) ReadOnly() bool {
	return t.readOnly
}

func (t policyTestTool) Execute(ctx context.Context, turn TurnView, input json.RawMessage) (ToolResult, error) {
	return ToolResult{}, nil
}

func TestPolicyIsAvailableFiltersWriteTools(t *testing.T) {
	policy := Policy{AllowWriteTools: false}

	if !policy.IsAvailable(policyTestTool{name: "read_tool", readOnly: true}) {
		t.Fatalf("expected read-only tool to be available")
	}
	if policy.IsAvailable(policyTestTool{name: "write_tool", readOnly: false}) {
		t.Fatalf("expected write tool to be unavailable")
	}
	if policy.IsAvailable(nil) {
		t.Fatalf("expected nil tool to be unavailable")
	}
	if err := policy.Check(policyTestTool{name: "write_tool", readOnly: false}); err == nil {
		t.Fatalf("expected policy check to reject write tool")
	}
}

func TestToOpenAIToolsWithPolicyFiltersWriteTools(t *testing.T) {
	registry := NewRegistry()
	registry.MustRegister(policyTestTool{name: "read_tool", readOnly: true})
	registry.MustRegister(policyTestTool{name: "write_tool", readOnly: false})

	filtered := ToOpenAIToolsWithPolicy(registry, Policy{AllowWriteTools: false})
	if len(filtered) != 1 || filtered[0].Function == nil || filtered[0].Function.Name != "read_tool" {
		t.Fatalf("expected only read_tool, got %#v", filtered)
	}

	allowed := ToOpenAIToolsWithPolicy(registry, Policy{AllowWriteTools: true})
	if len(allowed) != 2 {
		t.Fatalf("expected both tools when writes are allowed, got %#v", allowed)
	}
}
