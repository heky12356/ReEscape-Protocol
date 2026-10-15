package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"project-yume/internal/config"
	"project-yume/internal/storage"

	"github.com/gin-gonic/gin"
)

func setupStateAdmin(t *testing.T, backend string) (*storage.Persistence, *gin.Engine) {
	t.Helper()
	cfg := config.GetConfig()
	previous := cfg.AdminAPIKey
	cfg.AdminAPIKey = "query-test-key"
	t.Cleanup(func() { cfg.AdminAPIKey = previous })
	p, err := storage.OpenPersistence(storage.PersistenceOptions{Backend: backend, DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	return p, (&server{persistence: p}).routes()
}

func stateRequest(engine http.Handler, method, path string, authenticated bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if authenticated {
		req.Header.Set("Authorization", "Bearer query-test-key")
	}
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, req)
	return response
}

func TestStateAPIAuthenticationFiltersAndPagination(t *testing.T) {
	p, engine := setupStateAdmin(t, "sqlite")
	sessions := map[string]any{}
	for i := 0; i < 202; i++ {
		sessions[fmt.Sprintf("s-%03d", i)] = map[string]any{"id": fmt.Sprintf("s-%03d", i), "user_id": 42, "last_updated": "2026-01-01T00:00:00Z", "conversation": []any{map[string]any{"content": "private transcript"}}}
	}
	data, err := json.Marshal(map[string]any{"sessions": sessions})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Snapshots.Save("memory/conversation_history.json", data); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/admin/state/sessions", "/api/admin/state/sessions/s-001", "/api/admin/persistence"} {
		if response := stateRequest(engine, "GET", path, false); response.Code != 401 {
			t.Fatalf("unauthenticated %s: %d", path, response.Code)
		}
	}
	response := stateRequest(engine, "GET", "/api/admin/state/sessions?user_id=42&limit=200", true)
	if response.Code != 200 {
		t.Fatalf("query: %d %s", response.Code, response.Body)
	}
	var page storage.ReadModelPage
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Records) != 200 || page.Limit != 200 || page.NextCursor == "" || strings.Contains(response.Body.String(), "private transcript") {
		t.Fatalf("list contract: %d", len(page.Records))
	}
	nextURL := "/api/admin/state/sessions?user_id=42&limit=200&cursor=" + url.QueryEscape(page.NextCursor)
	next := stateRequest(engine, "GET", nextURL, true)
	var remaining storage.ReadModelPage
	if err := json.Unmarshal(next.Body.Bytes(), &remaining); err != nil {
		t.Fatal(err)
	}
	if next.Code != 200 || len(remaining.Records) != 2 || remaining.NextCursor != "" {
		t.Fatalf("next page: %d %s", next.Code, next.Body)
	}
	detail := stateRequest(engine, "GET", "/api/admin/state/sessions/s-001", true)
	if detail.Code != 200 || !strings.Contains(detail.Body.String(), "private transcript") {
		t.Fatalf("detail: %d %s", detail.Code, detail.Body)
	}
	if response := stateRequest(engine, "GET", "/api/admin/state/sessions/missing", true); response.Code != 404 {
		t.Fatal(response.Code)
	}
	for _, parameters := range []string{"status=pending", "limit=201", "limit=0", "limit=-1", "limit=abc", "user_id=-1", "user_id=abc", "limit=1&limit=2", "unknown=yes", "cursor=bad", "session_id="} {
		response := stateRequest(engine, "GET", "/api/admin/state/sessions?"+parameters, true)
		if response.Code != 400 {
			t.Fatalf("accepted %s: %d %s", parameters, response.Code, response.Body)
		}
	}
	if response := stateRequest(engine, "GET", "/api/admin/state/unknown", true); response.Code != 400 {
		t.Fatal(response.Code)
	}
	if err := p.Snapshots.Save("memory/conversation_history.json", data); err != nil {
		t.Fatal(err)
	}
	if response := stateRequest(engine, "GET", nextURL, true); response.Code != 409 || !strings.Contains(response.Body.String(), "read_model_changed") {
		t.Fatalf("stale cursor: %d %s", response.Code, response.Body)
	}
}

func TestStateAPIRunningBackendAndRebuildBoundaries(t *testing.T) {
	p, engine := setupStateAdmin(t, "file")
	cfg := config.GetConfig()
	previous := cfg.PersistenceBackend
	cfg.PersistenceBackend = "sqlite"
	t.Cleanup(func() { cfg.PersistenceBackend = previous })
	response := stateRequest(engine, "GET", "/api/admin/persistence", true)
	var info storage.PersistenceInfo
	if err := json.Unmarshal(response.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || info.Backend != storage.BackendFile || len(info.ReadModels) != 0 {
		t.Fatalf("reported env rather than actual backend: %s", response.Body)
	}
	if response := stateRequest(engine, "GET", "/api/admin/state/sessions", true); response.Code != 501 {
		t.Fatal(response.Code)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if response := stateRequest(engine, "GET", "/api/admin/state/sessions", true); response.Code != 503 {
		t.Fatal(response.Code)
	}
}

func TestStateRebuildRequiresAuthenticationAndCookieOrigin(t *testing.T) {
	_, engine := setupStateAdmin(t, "sqlite")
	if response := stateRequest(engine, "POST", "/api/admin/state/deliveries/rebuild", false); response.Code != 401 {
		t.Fatal(response.Code)
	}
	req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"key":"query-test-key"}`))
	req.Header.Set("Content-Type", "application/json")
	login := httptest.NewRecorder()
	engine.ServeHTTP(login, req)
	if login.Code != 200 {
		t.Fatalf("login: %d %s", login.Code, login.Body)
	}
	rebuild := httptest.NewRequest("POST", "/api/admin/state/deliveries/rebuild", nil)
	for _, cookie := range login.Result().Cookies() {
		rebuild.AddCookie(cookie)
	}
	denied := httptest.NewRecorder()
	engine.ServeHTTP(denied, rebuild)
	if denied.Code != 403 {
		t.Fatalf("missing cookie Origin accepted: %d", denied.Code)
	}
	accepted := stateRequest(engine, "POST", "/api/admin/state/deliveries/rebuild", true)
	if accepted.Code != 200 {
		t.Fatalf("bearer rebuild: %d %s", accepted.Code, accepted.Body)
	}
	response := stateRequest(engine, "GET", "/api/admin/persistence", true)
	var info storage.PersistenceInfo
	if err := json.Unmarshal(response.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || info.SchemaVersion != 3 || len(info.ReadModels) != 4 {
		t.Fatalf("schema info: %s", response.Body)
	}
}
