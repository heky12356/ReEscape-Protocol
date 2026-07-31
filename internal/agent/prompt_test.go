package agent

import (
	"strings"
	"testing"
)

func TestBuildSystemPromptSeparatesToolRulesAndFinalOutput(t *testing.T) {
	prompt := BuildSystemPrompt([]string{"get_memory_context", "get_state"})

	required := []string{
		"【Agent Policy】",
		"【Tool Rules】",
		"工具调用保持客观、结构化、可审计。",
		"不要用角色口吻编写工具 reason 或工具参数。",
		"【Scene / Relationship State】",
		"【Final Output Contract】",
		"保持角色身份和说话方式。",
	}
	for _, item := range required {
		if !strings.Contains(prompt, item) {
			t.Fatalf("expected system prompt to contain %q, got %q", item, prompt)
		}
	}
}
