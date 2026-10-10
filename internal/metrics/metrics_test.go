package metrics

import (
	"strings"
	"testing"
	"time"
)

func TestRegistryRendersCountersAndDurations(t *testing.T) {
	registry := NewRegistry()
	registry.AddCounter("bot_tool_executions_total", "Tool execution outcomes.", 2, map[string]string{
		"tool": "web_search", "result": "success",
	})
	registry.ObserveDuration("bot_tool_duration", "Tool execution duration.", 25*time.Millisecond, map[string]string{
		"tool": "web_search", "source": "web",
	})

	rendered := registry.RenderPrometheus()
	for _, expected := range []string{
		`# TYPE bot_tool_executions_total counter`,
		`bot_tool_executions_total{result="success",tool="web_search"} 2.000000`,
		`bot_tool_duration_count{source="web",tool="web_search"} 1.000000`,
		`bot_tool_duration_ms_total{source="web",tool="web_search"} 25.000000`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("rendered metrics missing %q:\n%s", expected, rendered)
		}
	}
}

func TestRegistryCopiesAndEscapesLabels(t *testing.T) {
	registry := NewRegistry()
	labels := map[string]string{"message": "line\n\"quoted\""}
	registry.AddCounter("bot_test_total", "help", 1, labels)
	labels["message"] = "changed"

	rendered := registry.RenderPrometheus()
	if !strings.Contains(rendered, `message="line\n\"quoted\""`) {
		t.Fatalf("label was not copied or escaped: %s", rendered)
	}
	if strings.Contains(rendered, "changed") {
		t.Fatalf("rendered metric changed after caller mutated labels: %s", rendered)
	}
}
