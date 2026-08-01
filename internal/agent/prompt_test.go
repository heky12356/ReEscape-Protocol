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
		"web_search",
		"web_fetch",
		"外部最新信息",
		"不要凭记忆回答可能已经变化的事实。",
		"【Memory Tool Rules】",
		"当当前问题依赖用户过往偏好、身份、事实、未闭合事项或情绪模式时，先调用 get_memory_context。",
		"当前写记忆工具不可用，不要尝试写入或声称已经记住。",
		"【Interaction Frame】",
		"当前互动发生在聊天软件的对话窗口中",
		"不要主动把自己描述成模型、程序、系统实例、虚拟体、AI 助手或扮演任务。",
		"不主动破坏当前聊天现场感。",
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

func TestBuildSystemPromptPlacesInteractionFrameBeforeSceneState(t *testing.T) {
	sections := BuildPromptSections([]string{"get_state"})
	prompt := sections.String()

	characterIndex := strings.Index(prompt, sections.CharacterIdentity)
	frameIndex := strings.Index(prompt, "【Interaction Frame】")
	sceneIndex := strings.Index(prompt, "【Scene / Relationship State】")

	if characterIndex < 0 || frameIndex < 0 || sceneIndex < 0 {
		t.Fatalf("expected character identity, interaction frame, and scene state in prompt: %q", prompt)
	}
	if !(characterIndex < frameIndex && frameIndex < sceneIndex) {
		t.Fatalf("expected interaction frame between character identity and scene state")
	}
}

func TestBuildSystemPromptIncludesMemoryWriteRulesWhenWriteToolsAreAvailable(t *testing.T) {
	prompt := BuildSystemPrompt([]string{"get_memory_context", "remember_fact", "update_profile"})

	required := []string{
		"【Memory Tool Rules】",
		"记住",
		"我喜欢",
		"我不喜欢",
		"我叫",
		"不要再",
		"update_profile 用于",
		"remember_fact 用于",
		"不要记录一次性情绪、临时吐槽、普通寒暄、含糊猜测",
		"写记忆后不要在最终回复中解释工具调用",
	}
	for _, item := range required {
		if !strings.Contains(prompt, item) {
			t.Fatalf("expected system prompt to contain %q, got %q", item, prompt)
		}
	}
	if strings.Contains(prompt, "当前写记忆工具不可用") {
		t.Fatalf("did not expect unavailable write-tool guidance when write tools are present: %q", prompt)
	}
}

func TestBuildSystemPromptDoesNotSuggestMemoryWritesWithoutWriteTools(t *testing.T) {
	prompt := BuildSystemPrompt([]string{"get_memory_context"})

	forbidden := []string{
		"优先调用 remember_fact 或 update_profile",
		"update_profile 用于",
		"remember_fact 用于",
	}
	for _, item := range forbidden {
		if strings.Contains(prompt, item) {
			t.Fatalf("did not expect system prompt to contain %q, got %q", item, prompt)
		}
	}
	if !strings.Contains(prompt, "当前写记忆工具不可用，不要尝试写入或声称已经记住。") {
		t.Fatalf("expected write-tool unavailable guidance, got %q", prompt)
	}
}

func TestBuildSystemPromptDoesNotContainRuntimeContext(t *testing.T) {
	prompt := BuildSystemPrompt([]string{"get_memory_context", "get_state"})

	forbidden := []string{
		"【Runtime Context】",
		"【Temporal Context】",
		"当前本地时间",
		"当前用户消息发送于",
		"距离上次用户消息",
		"本轮由",
	}
	for _, item := range forbidden {
		if strings.Contains(prompt, item) {
			t.Fatalf("did not expect system prompt to contain runtime context item %q: %q", item, prompt)
		}
	}
}

func TestBuildSystemPromptStableForSameToolSet(t *testing.T) {
	toolNames := []string{"get_memory_context", "get_state", "web_search"}

	first := BuildSystemPrompt(toolNames)
	second := BuildSystemPrompt(toolNames)

	if first != second {
		t.Fatalf("expected system prompt to be stable for same tool set")
	}
}
