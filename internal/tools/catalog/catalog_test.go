package catalog

import (
	"context"
	"testing"

	"project-yume/internal/tools"
	"project-yume/internal/webaccess"
)

type testSearchClient struct{}

func (testSearchClient) Search(ctx context.Context, req webaccess.SearchRequest) (webaccess.SearchResponse, error) {
	return webaccess.SearchResponse{}, nil
}

type testFetchClient struct{}

func (testFetchClient) Fetch(ctx context.Context, req webaccess.FetchRequest) (webaccess.FetchResponse, error) {
	return webaccess.FetchResponse{}, nil
}

func TestNewRegistryDoesNotRegisterWebToolsWhenDisabled(t *testing.T) {
	registry := NewRegistry(Options{
		EnableWebTools: true,
	})
	if _, ok := registry.Get("web_search"); ok {
		t.Fatalf("web_search should not be registered without a search client")
	}
	if _, ok := registry.Get("web_fetch"); ok {
		t.Fatalf("web_fetch should not be registered without a fetch client")
	}

	registry = NewRegistry()
	if _, ok := registry.Get("web_search"); ok {
		t.Fatalf("web_search should not be registered by default")
	}
	if _, ok := registry.Get("web_fetch"); ok {
		t.Fatalf("web_fetch should not be registered by default")
	}
}

func TestNewRegistryRegistersWebToolsWhenEnabled(t *testing.T) {
	registry := NewRegistry(Options{
		EnableWebTools:   true,
		SearchClient:     testSearchClient{},
		FetchClient:      testFetchClient{},
		SearchMaxResults: 5,
		FetchMaxChars:    6000,
	})
	if tool, ok := registry.Get("web_search"); !ok || !tool.ReadOnly() {
		t.Fatalf("expected read-only web_search to be registered")
	}
	if tool, ok := registry.Get("web_fetch"); !ok || !tool.ReadOnly() {
		t.Fatalf("expected read-only web_fetch to be registered")
	}
}

func TestNewRegistryRegistersProactiveScheduleTools(t *testing.T) {
	registry := NewRegistry()

	if tool, ok := registry.Get("get_proactive_schedule"); !ok || !tool.ReadOnly() {
		t.Fatalf("expected read-only get_proactive_schedule to be registered")
	}
	if tool, ok := registry.Get("update_proactive_schedule"); !ok || tool.ReadOnly() {
		t.Fatalf("expected write update_proactive_schedule to be registered")
	}
}

func TestNewRegistryRegistersReadOnlySkillTools(t *testing.T) {
	registry := NewRegistry()
	for _, name := range []string{"search_skills", "read_skill", "read_skill_resource"} {
		tool, ok := registry.Get(name)
		if !ok {
			t.Fatalf("expected %s to be registered", name)
		}
		if !tool.ReadOnly() {
			t.Fatalf("expected %s to be read-only", name)
		}
	}
}

func TestSkillToolsRemainAvailableWhenWriteToolsAreDisabled(t *testing.T) {
	definitions := ListDefinitionsWithPolicy(tools.Policy{AllowWriteTools: false})
	for _, name := range []string{"search_skills", "read_skill", "read_skill_resource"} {
		if !containsDefinition(definitions, name) {
			t.Fatalf("expected read-only %s to remain available: %#v", name, definitions)
		}
	}
}

func TestListDefinitionsWithPolicyFiltersProactiveScheduleWriteTool(t *testing.T) {
	definitions := ListDefinitionsWithPolicy(tools.Policy{AllowWriteTools: false})

	if !containsDefinition(definitions, "get_proactive_schedule") {
		t.Fatalf("expected read-only schedule tool to remain available: %#v", definitions)
	}
	if containsDefinition(definitions, "update_proactive_schedule") {
		t.Fatalf("did not expect write schedule tool when writes are disabled: %#v", definitions)
	}
}

func containsDefinition(definitions []Definition, name string) bool {
	for _, definition := range definitions {
		if definition.Name == name {
			return true
		}
	}
	return false
}
