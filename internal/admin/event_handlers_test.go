package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"project-yume/internal/eventlog"
)

func setupReplayAdmin(t *testing.T, backend string) (*server, eventlog.ReplayStore, *httptest.Server) {
	t.Helper()
	p, _ := setupStateAdmin(t, backend)
	var store eventlog.ReplayStore
	if backend == "sqlite" {
		store = eventlog.NewSQLiteEventStore(p.Database())
	} else {
		file, err := eventlog.NewFileStore(nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		store = file
	}
	previous := eventlog.Default()
	eventlog.SetDefault(eventlog.NewHookedStore(store, nil))
	t.Cleanup(func() { eventlog.SetDefault(previous) })
	admin := &server{persistence: p}
	httpServer := httptest.NewServer(admin.routes())
	t.Cleanup(httpServer.Close)
	return admin, store, httpServer
}

func replayHTTP(t *testing.T, server *httptest.Server, query string) (int, eventLogResponse) {
	t.Helper()
	request, _ := http.NewRequest("GET", server.URL+"/api/admin/events"+query, nil)
	request.Header.Set("Authorization", "Bearer query-test-key")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var page eventLogResponse
	if err := json.NewDecoder(response.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, page
}
func replaySocket(t *testing.T, server *httptest.Server, query string, headers http.Header) *websocket.Conn {
	t.Helper()
	if headers == nil {
		headers = http.Header{"Authorization": []string{"Bearer query-test-key"}}
	}
	connection, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/api/admin/events/stream"+query, headers)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { connection.Close() })
	connection.SetReadDeadline(time.Now().Add(5 * time.Second))
	return connection
}

func TestEventHTTPBaselineStreamHandoffAndReconnect(t *testing.T) {
	for _, backend := range []string{"file", "sqlite"} {
		t.Run(backend, func(t *testing.T) {
			_, store, server := setupReplayAdmin(t, backend)
			ctx := context.Background()
			if err := store.Append(ctx, eventlog.Event{Type: "before"}); err != nil {
				t.Fatal(err)
			}
			status, baseline := replayHTTP(t, server, "?limit=1")
			if status != 200 || baseline.Cursor != "1" || baseline.StreamID == "" {
				t.Fatalf("baseline: %d %+v", status, baseline)
			}
			pending := make([]eventlog.Event, 55)
			for i := range pending {
				pending[i] = eventlog.Event{Type: "during_handshake", ID: fmt.Sprintf("replay-%d", i), RequestID: "r", Tool: "search"}
			}
			if err := store.Append(ctx, pending...); err != nil {
				t.Fatal(err)
			}
			connection := replaySocket(t, server, "?limit=7&request_id=r&tool=search&after="+baseline.Cursor+"&stream_id="+url.QueryEscape(baseline.StreamID), nil)
			last := int64(1)
			count, checkpoints := 0, 0
			for {
				var frame struct {
					Type, Cursor string
					Event        eventlog.Event
					HasMore      bool
				}
				if err := connection.ReadJSON(&frame); err != nil {
					t.Fatal(err)
				}
				cursor, err := strconv.ParseInt(frame.Cursor, 10, 64)
				if err != nil {
					t.Fatal(err)
				}
				if frame.Type == "event" {
					if cursor <= last {
						t.Fatal("duplicate")
					}
					last = cursor
					count++
				}
				if frame.Type == "checkpoint" {
					checkpoints++
					if !frame.HasMore {
						break
					}
				}
			}
			if count != 55 || last != 56 || checkpoints < 2 {
				t.Fatalf("truncated replay: %d %d %d", count, last, checkpoints)
			}
			connection.Close()
			for i := 0; i < 2; i++ {
				if err := store.Append(ctx, eventlog.Event{Type: "reconnected", RequestID: "r"}); err != nil {
					t.Fatal(err)
				}
				connection = replaySocket(t, server, fmt.Sprintf("?after=%d&stream_id=%s&request_id=r", last, baseline.StreamID), nil)
				var frame struct {
					Type, Cursor string
					Event        eventlog.Event
				}
				if err := connection.ReadJSON(&frame); err != nil {
					t.Fatal(err)
				}
				if frame.Type != "event" || frame.Event.Type != "reconnected" {
					t.Fatalf("resume: %+v", frame)
				}
				last, _ = strconv.ParseInt(frame.Cursor, 10, 64)
				connection.Close()
			}
		})
	}
}

func TestEventGapResetAndFilters(t *testing.T) {
	_, store, server := setupReplayAdmin(t, "file")
	events := make([]eventlog.Event, 1005)
	for i := range events {
		events[i] = eventlog.Event{Type: "probe", SessionID: "s", RequestID: "r"}
	}
	if err := store.Append(context.Background(), events...); err != nil {
		t.Fatal(err)
	}
	status, page := replayHTTP(t, server, "?after=0")
	if status != 409 || page.Reason != "retention_gap" || page.EarliestCursor != "6" {
		t.Fatalf("gap: %d %+v", status, page)
	}
	connection := replaySocket(t, server, "?after=0", nil)
	var frame struct{ Type, Reason string }
	if err := connection.ReadJSON(&frame); err != nil || frame.Type != "gap" {
		t.Fatalf("gap frame: %+v %v", frame, err)
	}
	connection.Close()
	status, page = replayHTTP(t, server, "?after=5&request_id=missing&limit=2")
	if status != 200 || len(page.Events) != 0 || page.Cursor != "7" || !page.HasMore {
		t.Fatalf("filter progress: %d %+v", status, page)
	}
	for _, query := range []string{"?limit=0", "?limit=1001", "?after=-1", "?after=bad", "?after=1&after=2", "?tool=", "?unknown=x", "?stream_id=old"} {
		request, _ := http.NewRequest("GET", server.URL+"/api/admin/events"+query, nil)
		request.Header.Set("Authorization", "Bearer query-test-key")
		r, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		if r.StatusCode != 400 {
			t.Fatalf("accepted %s: %d", query, r.StatusCode)
		}
	}
	status, page = replayHTTP(t, server, "?after=999999")
	if status != 409 || page.Reason != "cursor_reset" {
		t.Fatalf("reset: %d %+v", status, page)
	}
	status, page = replayHTTP(t, server, "?after=1005&stream_id=previous-process")
	if status != 409 || page.Reason != "stream_changed" {
		t.Fatalf("restart: %d %+v", status, page)
	}
	connection = replaySocket(t, server, "?after=1005&stream_id=previous-process", nil)
	if err := connection.ReadJSON(&frame); err != nil || frame.Type != "reset" || frame.Reason != "stream_changed" {
		t.Fatalf("reset frame: %+v %v", frame, err)
	}
}

func TestEventStreamRevokesExistingCookieConnection(t *testing.T) {
	admin, _, server := setupReplayAdmin(t, "file")
	token, _, err := admin.sessions.create(time.Now(), "query-test-key")
	if err != nil {
		t.Fatal(err)
	}
	connection := replaySocket(t, server, "?after=0", http.Header{"Cookie": []string{adminSessionCookie + "=" + token}, "Origin": []string{server.URL}})
	var frame map[string]any
	if err := connection.ReadJSON(&frame); err != nil {
		t.Fatal(err)
	}
	admin.sessions.revoke(token)
	_, _, err = connection.ReadMessage()
	if !websocket.IsCloseError(err, 4001) {
		t.Fatalf("revoked stream still live: %v", err)
	}
}

func TestEventCursorsKeepInt64Precision(t *testing.T) {
	admin, _, server := setupReplayAdmin(t, "sqlite")
	_, err := admin.persistence.Database().Exec(`INSERT INTO events(cursor,id,type,created_at,payload_json) VALUES(9007199254740993,'large','probe','2026-01-01T00:00:00Z','{"id":"large","type":"probe"}')`)
	if err != nil {
		t.Fatal(err)
	}
	status, page := replayHTTP(t, server, "")
	if status != 200 || page.Cursor != "9007199254740993" || len(page.EventCursors) != 1 || page.EventCursors[0] != page.Cursor {
		t.Fatalf("precision lost: %+v", page)
	}
}

func TestFilteredStreamAdvancesThroughEmptyBatches(t *testing.T) {
	_, store, server := setupReplayAdmin(t, "sqlite")
	events := make([]eventlog.Event, 25)
	for i := range events {
		events[i] = eventlog.Event{Type: "other", RequestID: "not-selected"}
	}
	if err := store.Append(context.Background(), events...); err != nil {
		t.Fatal(err)
	}
	connection := replaySocket(t, server, "?after=0&request_id=selected&limit=7", nil)
	for _, expected := range []string{"7", "14", "21", "25"} {
		var frame struct {
			Type, Cursor string
			HasMore      bool
		}
		if err := connection.ReadJSON(&frame); err != nil {
			t.Fatal(err)
		}
		if frame.Type != "checkpoint" || frame.Cursor != expected || frame.HasMore != (expected != "25") {
			t.Fatalf("filtered checkpoint: %+v", frame)
		}
	}
}

func TestEventStreamRejectsUnauthorizedAndForeignOrigin(t *testing.T) {
	_, _, server := setupReplayAdmin(t, "file")
	endpoint := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/admin/events/stream?after=0"
	for _, test := range []struct {
		headers http.Header
		status  int
	}{
		{http.Header{}, http.StatusUnauthorized},
		{http.Header{"Authorization": []string{"Bearer query-test-key"}, "Origin": []string{"https://foreign.example"}}, http.StatusForbidden},
	} {
		connection, response, err := websocket.DefaultDialer.Dial(endpoint, test.headers)
		if connection != nil {
			connection.Close()
		}
		if err == nil || response == nil || response.StatusCode != test.status {
			t.Fatalf("handshake accepted: %v %+v", err, response)
		}
		response.Body.Close()
	}
}
