package connect

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"project-yume/internal/model"
)

func TestCallAPIReceivesMatchingResponse(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		var request model.Message
		if err := conn.ReadJSON(&request); err != nil {
			t.Error(err)
			return
		}
		response, _ := json.Marshal(model.APIResponse{Status: "ok", Echo: request.Echo})
		if err := conn.WriteMessage(websocket.TextMessage, response); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()

	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial("ws"+server.URL[len("http"):], nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer Close(conn)
	go func() {
		for {
			_, payload, err := conn.ReadMessage()
			if err != nil {
				return
			}
			_ = DispatchAPIResponse(payload)
		}
	}()

	response, err := CallAPI(conn, "get_status", map[string]any{"detail": true})
	if err != nil {
		t.Fatalf("CallAPI: %v", err)
	}
	if response.Status != "ok" {
		t.Fatalf("status = %q, want ok", response.Status)
	}
}

func TestCallAPIFailsWhenConnectionCloses(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err == nil {
			defer conn.Close()
			_, _, _ = conn.ReadMessage()
		}
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial("ws"+server.URL[len("http"):], nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	result := make(chan error, 1)
	go func() {
		_, err := CallAPI(conn, "get_status", nil)
		result <- err
	}()
	time.Sleep(20 * time.Millisecond)
	if err := Close(conn); err != nil {
		t.Fatalf("close: %v", err)
	}
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("CallAPI returned nil after connection close")
		}
	case <-time.After(time.Second):
		t.Fatal("CallAPI did not fail after connection close")
	}
}
