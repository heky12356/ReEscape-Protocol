package intent

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"project-yume/internal/domain/openloop"
	"project-yume/internal/state"
)

func TestUpdateOpenLoopResolveRequiresResolution(t *testing.T) {
	sm := state.GetManager()
	sm.ClearAllSessions()
	t.Cleanup(sm.ClearAllSessions)
	now := time.Now()
	loop := openloop.NewExplicit(42, state.PrivateSessionID(42), string(openloop.KindFollowUp), "待解决事项", "turn-1", now)
	if _, err := sm.OpenLoopStore().Upsert(loop); err != nil {
		t.Fatal(err)
	}
	turn := testTurn{id: "turn-2", session: loop.SessionID, user: 42, at: now}
	_, err := NewUpdateOpenLoopTool().Execute(context.Background(), turn, json.RawMessage(`{"action":"resolve","open_loop_id":"`+loop.ID+`"}`))
	if err == nil {
		t.Fatal("expected resolution requirement")
	}
}
