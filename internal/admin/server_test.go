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

func TestAdminSessionLoginAndLogout(t *testing.T) {
	cfg := config.GetConfig()
	previousKey := cfg.AdminAPIKey
	previousSecure := cfg.AdminCookieSecure
	cfg.AdminAPIKey = "test-session-key"
	cfg.AdminCookieSecure = true
	defer func() {
		cfg.AdminAPIKey = previousKey
		cfg.AdminCookieSecure = previousSecure
	}()

	s := &server{sessions: newSessionStore()}
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	auth := engine.Group("/api/auth")
	auth.POST("/login", s.handleAdminLogin)
	auth.GET("/session", s.handleAdminSession)
	auth.POST("/logout", s.handleAdminLogout)
	admin := engine.Group("/api/admin")
	admin.Use(adminAuthMiddleware(s.sessions))
	admin.GET("/probe", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	loginBody, _ := json.Marshal(adminLoginRequest{Key: "test-session-key"})
	login := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(string(loginBody)))
	login.Header.Set("Content-Type", "application/json")
	loginResp := httptest.NewRecorder()
	engine.ServeHTTP(loginResp, login)
	if loginResp.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d: %s", loginResp.Code, http.StatusOK, loginResp.Body.String())
	}
	if !strings.Contains(loginResp.Header().Get("Set-Cookie"), "admin_session=") || !strings.Contains(loginResp.Header().Get("Set-Cookie"), "HttpOnly") || !strings.Contains(loginResp.Header().Get("Set-Cookie"), "Secure") {
		t.Fatalf("login cookie missing security attributes: %q", loginResp.Header().Get("Set-Cookie"))
	}
	cookies := loginResp.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one session cookie, got %d", len(cookies))
	}

	sessionCheck := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	sessionCheck.AddCookie(cookies[0])
	sessionCheckResp := httptest.NewRecorder()
	engine.ServeHTTP(sessionCheckResp, sessionCheck)
	if sessionCheckResp.Code != http.StatusOK {
		t.Fatalf("session check status = %d, want %d", sessionCheckResp.Code, http.StatusOK)
	}

	probe := httptest.NewRequest(http.MethodGet, "/api/admin/probe", nil)
	probe.AddCookie(cookies[0])
	probeResp := httptest.NewRecorder()
	engine.ServeHTTP(probeResp, probe)
	if probeResp.Code != http.StatusNoContent {
		t.Fatalf("session-authenticated probe status = %d, want %d", probeResp.Code, http.StatusNoContent)
	}

	logout := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logout.AddCookie(cookies[0])
	logoutResp := httptest.NewRecorder()
	engine.ServeHTTP(logoutResp, logout)
	if logoutResp.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want %d", logoutResp.Code, http.StatusNoContent)
	}

	afterLogout := httptest.NewRequest(http.MethodGet, "/api/admin/probe", nil)
	afterLogout.AddCookie(cookies[0])
	afterLogoutResp := httptest.NewRecorder()
	engine.ServeHTTP(afterLogoutResp, afterLogout)
	if afterLogoutResp.Code != http.StatusUnauthorized {
		t.Fatalf("post-logout status = %d, want %d", afterLogoutResp.Code, http.StatusUnauthorized)
	}
}

func TestAdminLoginRejectsWrongKey(t *testing.T) {
	cfg := config.GetConfig()
	previous := cfg.AdminAPIKey
	cfg.AdminAPIKey = "correct-key"
	defer func() { cfg.AdminAPIKey = previous }()

	s := &server{sessions: newSessionStore(), loginLimiter: newLoginLimiter()}
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/login", s.handleAdminLogin)
	body, _ := json.Marshal(adminLoginRequest{Key: "wrong-key"})
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("wrong key status = %d, want %d", resp.Code, http.StatusUnauthorized)
	}
	if resp.Header().Get("Set-Cookie") != "" {
		t.Fatalf("wrong key unexpectedly set a cookie: %q", resp.Header().Get("Set-Cookie"))
	}
	if strings.Contains(resp.Body.String(), "correct-key") {
		t.Fatalf("wrong key response exposed configured key: %s", resp.Body.String())
	}
}

func TestAdminLoginUsesSameErrorForEmptyAndWrongKeys(t *testing.T) {
	cfg := config.GetConfig()
	previous := cfg.AdminAPIKey
	cfg.AdminAPIKey = "correct-key"
	defer func() { cfg.AdminAPIKey = previous }()

	s := &server{sessions: newSessionStore(), loginLimiter: newLoginLimiter()}
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/login", s.handleAdminLogin)

	requests := []string{`{"key":"wrong-key"}`, `{"key":""}`, `{`}
	for index, body := range requests {
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		engine.ServeHTTP(resp, req)
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("request %d status = %d, want %d", index, resp.Code, http.StatusUnauthorized)
		}
		if resp.Body.String() != `{"error":"invalid admin key"}` {
			t.Fatalf("request %d body = %q, want generic invalid-key response", index, resp.Body.String())
		}
	}
}

func TestAdminLoginRateLimitsFailuresByClientIP(t *testing.T) {
	cfg := config.GetConfig()
	previous := cfg.AdminAPIKey
	cfg.AdminAPIKey = "correct-key"
	defer func() { cfg.AdminAPIKey = previous }()

	s := &server{sessions: newSessionStore(), loginLimiter: newLoginLimiter()}
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/login", s.handleAdminLogin)
	for attempt := 0; attempt < loginFailureLimit; attempt++ {
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"key":"wrong-key"}`))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		engine.ServeHTTP(resp, req)
		if resp.Code != http.StatusUnauthorized && resp.Code != http.StatusTooManyRequests {
			t.Fatalf("attempt %d status = %d, want unauthorized or rate limited", attempt+1, resp.Code)
		}
	}

	blocked := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"key":"correct-key"}`))
	blocked.Header.Set("Content-Type", "application/json")
	blockedResp := httptest.NewRecorder()
	engine.ServeHTTP(blockedResp, blocked)
	if blockedResp.Code != http.StatusTooManyRequests {
		t.Fatalf("blocked login status = %d, want %d", blockedResp.Code, http.StatusTooManyRequests)
	}
	if blockedResp.Header().Get("Retry-After") == "" {
		t.Fatal("blocked login did not include Retry-After")
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

func TestCORSMiddlewareRejectsWildcardWithCredentials(t *testing.T) {
	previous := config.GetConfig().AdminCORSOrigins
	config.GetConfig().AdminCORSOrigins = []string{"*"}
	defer func() { config.GetConfig().AdminCORSOrigins = previous }()

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(corsMiddleware())
	engine.GET("/resource", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	request := httptest.NewRequest(http.MethodGet, "/resource", nil)
	request.Header.Set("Origin", "https://attacker.example")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("wildcard CORS unexpectedly allowed origin %q", got)
	}

	preflight := httptest.NewRequest(http.MethodOptions, "/resource", nil)
	preflight.Header.Set("Origin", "https://attacker.example")
	preflightResponse := httptest.NewRecorder()
	engine.ServeHTTP(preflightResponse, preflight)
	if preflightResponse.Code != http.StatusForbidden {
		t.Fatalf("wildcard preflight status = %d, want %d", preflightResponse.Code, http.StatusForbidden)
	}
}

func TestSessionWritesRequireAllowedOriginButBearerWritesRemainCompatible(t *testing.T) {
	cfg := config.GetConfig()
	previousKey := cfg.AdminAPIKey
	previousOrigins := cfg.AdminCORSOrigins
	previousSecure := cfg.AdminCookieSecure
	cfg.AdminAPIKey = "test-origin-key"
	cfg.AdminCORSOrigins = []string{"https://console.example"}
	cfg.AdminCookieSecure = false
	t.Cleanup(func() {
		cfg.AdminAPIKey = previousKey
		cfg.AdminCORSOrigins = previousOrigins
		cfg.AdminCookieSecure = previousSecure
	})

	s := &server{sessions: newSessionStore(), loginLimiter: newLoginLimiter()}
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	auth := engine.Group("/api/auth")
	auth.POST("/login", s.handleAdminLogin)
	admin := engine.Group("/api/admin")
	admin.Use(adminAuthMiddleware(s.sessions), adminWriteOriginMiddleware())
	admin.PUT("/write", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	loginBody, _ := json.Marshal(adminLoginRequest{Key: "test-origin-key"})
	login := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(string(loginBody)))
	login.Header.Set("Content-Type", "application/json")
	loginResp := httptest.NewRecorder()
	engine.ServeHTTP(loginResp, login)
	if loginResp.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", loginResp.Code, http.StatusOK)
	}
	cookies := loginResp.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one session cookie, got %d", len(cookies))
	}

	for _, test := range []struct {
		name   string
		origin string
		status int
	}{
		{name: "allowed session origin", origin: "https://console.example", status: http.StatusNoContent},
		{name: "denied session origin", origin: "https://attacker.example", status: http.StatusForbidden},
		{name: "missing session origin", status: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/api/admin/write", nil)
			req.AddCookie(cookies[0])
			if test.origin != "" {
				req.Header.Set("Origin", test.origin)
			}
			resp := httptest.NewRecorder()
			engine.ServeHTTP(resp, req)
			if resp.Code != test.status {
				t.Fatalf("status = %d, want %d", resp.Code, test.status)
			}
		})
	}

	bearer := httptest.NewRequest(http.MethodPut, "/api/admin/write", nil)
	bearer.Header.Set("Authorization", "Bearer test-origin-key")
	bearerResp := httptest.NewRecorder()
	engine.ServeHTTP(bearerResp, bearer)
	if bearerResp.Code != http.StatusNoContent {
		t.Fatalf("Bearer write status = %d, want %d", bearerResp.Code, http.StatusNoContent)
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
