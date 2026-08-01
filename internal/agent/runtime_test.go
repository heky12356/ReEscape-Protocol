package agent

import (
	"strings"
	"testing"
	"time"

	"project-yume/internal/state"
	"project-yume/internal/tools"
	"project-yume/internal/tools/catalog"
)

func TestRuntimeBuildMessagesFiltersWriteToolsFromPromptWhenPolicyDisallowsWrites(t *testing.T) {
	restoreTemporalConfig(t, false, "Asia/Shanghai", "2006-01-02 15:04")
	state.GetManager().ClearAllSessions()
	t.Cleanup(state.GetManager().ClearAllSessions)

	registry := catalog.NewRegistry()
	executor := tools.NewExecutor(registry, tools.Policy{AllowWriteTools: false}, time.Second)
	runtime := NewRuntime(registry, executor, Budget{})

	messages := runtime.buildMessages(NewTurnContext(TurnInput{
		SessionID: "private:99",
		UserID:    99,
		ChatType:  1,
	}))
	if len(messages) == 0 {
		t.Fatalf("expected at least one system message")
	}

	prompt := messages[0].Content
	forbidden := []string{"remember_fact", "update_profile", "update_affection"}
	for _, item := range forbidden {
		if strings.Contains(prompt, item) {
			t.Fatalf("did not expect filtered write tool %q in prompt: %q", item, prompt)
		}
	}
	if !strings.Contains(prompt, "当前写记忆工具不可用，不要尝试写入或声称已经记住。") {
		t.Fatalf("expected memory write unavailable guidance, got %q", prompt)
	}
}
