package catalog

import (
	"context"
	"testing"

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
