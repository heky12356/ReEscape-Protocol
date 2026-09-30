package intent

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	domain "project-yume/internal/domain/intent"
	"project-yume/internal/state"
)

type testTurn struct {
	id, session, message string
	user                 int64
	at                   time.Time
}

func (t testTurn) RequestID() string        { return t.id }
func (t testTurn) SessionID() string        { return t.session }
func (t testTurn) UserID() int64            { return t.user }
func (t testTurn) GroupID() int64           { return 0 }
func (t testTurn) ChatType() int            { return 1 }
func (t testTurn) Message() string          { return t.message }
func (t testTurn) ReferenceTime() time.Time { return t.at }
func (t testTurn) Trigger() string          { return "user" }
func (t testTurn) Actor() string            { return "user" }

func TestUpdateIntentCreatesAndCompletesIntent(t *testing.T) {
	sm := state.GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)
	turn := testTurn{id: "turn-1", session: state.PrivateSessionID(42), user: 42, at: time.Now()}
	tool := NewUpdateIntentTool()
	due := turn.at.Add(time.Hour).Format(time.RFC3339)
	result, err := tool.Execute(context.Background(), turn, json.RawMessage(`{"action":"create","kind":"follow_up","summary":"继续讨论","due_at":"`+due+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	item, ok := result.Data.(domain.Intent)
	if !ok || item.ID == "" {
		t.Fatalf("unexpected intent: %#v", result.Data)
	}
	_, err = tool.Execute(context.Background(), turn, json.RawMessage(`{"action":"complete","intent_id":"`+item.ID+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	updated, ok := sm.GetIntent(item.ID)
	if !ok || updated.Status != domain.StatusCompleted {
		t.Fatalf("unexpected final intent: %+v", updated)
	}
}
