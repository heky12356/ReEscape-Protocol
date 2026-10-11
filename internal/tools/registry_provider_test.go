package tools

import (
	"context"
	"testing"
)

type registryTestProvider struct {
	name    string
	origin  string
	version string
	items   []Tool
}

func (p registryTestProvider) Name() string                         { return p.name }
func (p registryTestProvider) Origin() string                       { return p.origin }
func (p registryTestProvider) Version() string                      { return p.version }
func (p registryTestProvider) Close() error                         { return nil }
func (p registryTestProvider) List(context.Context) ([]Tool, error) { return p.items, nil }

func TestRegistryDefaultMetadata(t *testing.T) {
	registry := NewRegistry()
	registry.MustRegister(policyTestTool{name: "read_tool", readOnly: true})

	metadata, ok := registry.Metadata("read_tool")
	if !ok || metadata.Provider != "core" || metadata.Origin != "go" || metadata.Version != "builtin" {
		t.Fatalf("unexpected default metadata: %#v, %v", metadata, ok)
	}
}

func TestRegistryProviderMetadataAndConflict(t *testing.T) {
	registry := NewRegistry()
	tool := policyTestTool{name: "shared_tool", readOnly: true}
	if err := registry.RegisterProvider(context.Background(), registryTestProvider{
		name: "skill", origin: "skill", version: "v1", items: []Tool{tool},
	}); err != nil {
		t.Fatalf("register provider: %v", err)
	}
	if err := registry.RegisterProvider(context.Background(), registryTestProvider{
		name: "plugin", origin: "plugin", version: "v2", items: []Tool{tool},
	}); err == nil {
		t.Fatal("expected duplicate provider registration to fail")
	}

	metadata, ok := registry.Metadata("shared_tool")
	if !ok || metadata.Provider != "skill" || metadata.Origin != "skill" || metadata.Version != "v1" {
		t.Fatalf("existing tool was replaced: %#v, %v", metadata, ok)
	}
	conflicts := registry.Conflicts()
	if len(conflicts) != 1 || conflicts[0].Rejected.Provider != "plugin" || conflicts[0].Rejected.ConflictReason == "" {
		t.Fatalf("unexpected conflict record: %#v", conflicts)
	}
}
