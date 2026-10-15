package admin

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"project-yume/internal/storage"
)

func TestConversationAPI(t *testing.T) {
	p, engine := setupStateAdmin(t, "sqlite")
	data := []byte(`{"sessions":{"s":{"user_id":42,"conversation":[{"role":"user","content":"hello"},{"role":"assistant","content":"delivered"}],"interrupted_reply":{"status":"cancelled","undelivered_segments":["undelivered"]}}}}`)
	if err := p.Snapshots.Save("memory/conversation_history.json", data); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/admin/sessions/s", "/api/admin/sessions/s/messages"} {
		if r := stateRequest(engine, "GET", path, false); r.Code != 401 {
			t.Fatal(r.Code)
		}
	}
	if r := stateRequest(engine, "GET", "/api/admin/sessions/s", true); r.Code != 200 || strings.Contains(r.Body.String(), "conversation") || strings.Contains(r.Body.String(), "undelivered") {
		t.Fatalf("summary: %d %s", r.Code, r.Body)
	}
	r := stateRequest(engine, "GET", "/api/admin/sessions/s/messages?limit=1", true)
	var page storage.ConversationPage
	if err := json.Unmarshal(r.Body.Bytes(), &page); err != nil || r.Code != 200 || len(page.Messages) != 1 || page.NextCursor == "" {
		t.Fatalf("first: %d %s %v", r.Code, r.Body, err)
	}
	next := "/api/admin/sessions/s/messages?limit=1&cursor=" + url.QueryEscape(page.NextCursor)
	if r := stateRequest(engine, "GET", next, true); r.Code != 200 || !strings.Contains(r.Body.String(), "delivered") || strings.Contains(r.Body.String(), "undelivered") {
		t.Fatalf("next: %d %s", r.Code, r.Body)
	}
	for _, parameters := range []string{"limit=0", "limit=201", "limit=abc", "limit=1&limit=2", "user_id=42", "cursor=bad", "cursor="} {
		if r := stateRequest(engine, "GET", "/api/admin/sessions/s/messages?"+parameters, true); r.Code != 400 {
			t.Fatalf("accepted %s: %d", parameters, r.Code)
		}
	}
	if r := stateRequest(engine, "GET", "/api/admin/sessions/missing/messages", true); r.Code != 404 {
		t.Fatal(r.Code)
	}
	if err := p.Snapshots.Save("memory/conversation_history.json", []byte(`{"sessions":{"s":{"conversation":[]}}}`)); err != nil {
		t.Fatal(err)
	}
	if r := stateRequest(engine, "GET", next, true); r.Code != 409 {
		t.Fatalf("stale: %d %s", r.Code, r.Body)
	}
}

func TestFileBackendConversationUnavailable(t *testing.T) {
	_, engine := setupStateAdmin(t, "file")
	for _, path := range []string{"/api/admin/sessions/s", "/api/admin/sessions/s/messages"} {
		if r := stateRequest(engine, "GET", path, true); r.Code != 501 {
			t.Fatal(r.Code)
		}
	}
}
