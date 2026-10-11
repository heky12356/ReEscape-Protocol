package agent

import (
	"testing"
	"time"
)

func TestEventFromStepTraceCarriesToolMetadata(t *testing.T) {
	turn := NewTurnContext(TurnInput{
		RequestID: "req-1",
		TurnID:    "turn-1",
		SessionID: "session-1",
		UserID:    42,
	})
	step := StepTrace{
		Index:        1,
		ToolName:     "search",
		ToolProvider: "web",
		ToolOrigin:   "external",
		ToolVersion:  "v1",
		ToolCallID:   "call-1",
		FinishedAt:   time.Now(),
	}
	event := eventFromStepTrace(turn, step)
	if event.ToolProvider != "web" || event.ToolOrigin != "external" || event.ToolVersion != "v1" {
		t.Fatalf("event did not carry tool metadata: %#v", event)
	}
	if event.Data["tool_provider"] != "web" || event.Data["tool_origin"] != "external" || event.Data["tool_version"] != "v1" {
		t.Fatalf("event data did not carry tool metadata: %#v", event.Data)
	}
}
