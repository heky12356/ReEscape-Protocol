package eventlog

import (
	"encoding/json"
	"testing"
)

func TestEventToolMetadataRoundTrip(t *testing.T) {
	event := Event{
		Type:         "tool_called",
		ToolName:     "search",
		ToolProvider: "web",
		ToolOrigin:   "external",
		ToolVersion:  "v1",
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Event
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ToolProvider != "web" || decoded.ToolOrigin != "external" || decoded.ToolVersion != "v1" {
		t.Fatalf("tool metadata did not round trip: %#v", decoded)
	}
}

func TestEventCorrelationAddsToolMetadata(t *testing.T) {
	event := (Event{Type: "tool_called"}).WithCorrelation(Correlation{
		ToolProvider: "skill",
		ToolOrigin:   "skill",
		ToolVersion:  "2026.10",
	})
	if event.ToolProvider != "skill" || event.ToolOrigin != "skill" || event.ToolVersion != "2026.10" {
		t.Fatalf("tool metadata was not propagated: %#v", event)
	}
}
