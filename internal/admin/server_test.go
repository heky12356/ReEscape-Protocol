package admin

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "  ", want: ""},
		{name: "short", input: "secret", want: "****"},
		{name: "long", input: "tvly_1234567890abcd", want: "tvly***********abcd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := maskSecret(tt.input); got != tt.want {
				t.Fatalf("maskSecret(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestConfigResponseDoesNotExposeWebSearchAPIKey(t *testing.T) {
	const rawKey = "tvly_1234567890abcd"

	data, err := json.Marshal(configResponse{
		WebSearchAPIKeyMasked: maskSecret(rawKey),
		WebSearchAPIKeySet:    true,
		WebSearchProvider:     "tavily",
		WebSearchEndpoint:     "https://api.tavily.com",
	})
	if err != nil {
		t.Fatalf("marshal config response: %v", err)
	}

	body := string(data)
	if strings.Contains(body, rawKey) {
		t.Fatalf("config response contains the raw web search API key: %s", body)
	}
	if !strings.Contains(body, `"webSearchApiKeyMasked":"tvly***********abcd"`) {
		t.Fatalf("config response does not contain the expected masked key: %s", body)
	}
	if strings.Contains(body, `"webSearchApiKey"`) {
		t.Fatalf("config response contains a raw webSearchApiKey field: %s", body)
	}
}
