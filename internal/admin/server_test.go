package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"project-yume/internal/config"

	"github.com/gin-gonic/gin"
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

func TestAdminAuthMiddleware(t *testing.T) {
	previous := config.GetConfig().AdminAPIKey
	config.GetConfig().AdminAPIKey = "test-secret"
	defer func() { config.GetConfig().AdminAPIKey = previous }()

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(adminAuthMiddleware())
	engine.GET("/admin", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	for _, test := range []struct {
		name   string
		header string
		value  string
		status int
	}{
		{name: "missing", status: http.StatusUnauthorized},
		{name: "wrong", header: "Authorization", value: "Bearer wrong", status: http.StatusForbidden},
		{name: "bearer", header: "Authorization", value: "Bearer test-secret", status: http.StatusNoContent},
		{name: "x-api-key", header: "X-API-Key", value: "test-secret", status: http.StatusNoContent},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/admin", nil)
			if test.header != "" {
				req.Header.Set(test.header, test.value)
			}
			resp := httptest.NewRecorder()
			engine.ServeHTTP(resp, req)
			if resp.Code != test.status {
				t.Fatalf("status = %d, want %d", resp.Code, test.status)
			}
		})
	}
}

func TestCORSMiddlewareUsesConfiguredAllowlist(t *testing.T) {
	previous := config.GetConfig().AdminCORSOrigins
	config.GetConfig().AdminCORSOrigins = []string{"http://allowed.example"}
	defer func() { config.GetConfig().AdminCORSOrigins = previous }()

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(corsMiddleware())
	engine.GET("/resource", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	allowed := httptest.NewRequest(http.MethodGet, "/resource", nil)
	allowed.Header.Set("Origin", "http://allowed.example")
	allowedResp := httptest.NewRecorder()
	engine.ServeHTTP(allowedResp, allowed)
	if got := allowedResp.Header().Get("Access-Control-Allow-Origin"); got != "http://allowed.example" {
		t.Fatalf("allowed origin = %q", got)
	}

	denied := httptest.NewRequest(http.MethodOptions, "/resource", nil)
	denied.Header.Set("Origin", "http://denied.example")
	deniedResp := httptest.NewRecorder()
	engine.ServeHTTP(deniedResp, denied)
	if deniedResp.Code != http.StatusForbidden {
		t.Fatalf("denied preflight status = %d, want %d", deniedResp.Code, http.StatusForbidden)
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
