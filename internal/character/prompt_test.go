package character

import (
	"strings"
	"testing"
)

func TestBuildPromptUsesProductIdentityWithoutRoleplayWording(t *testing.T) {
	prompt := BuildPrompt(CharacterConfig{
		Name:        "江梦",
		Description: "一个20岁的软件工程专业学生",
		Voice: CharacterVoice{
			Tone:       "熟人面前自然一点",
			Vocabulary: []string{"啊这", "有点"},
			Avoid:      []string{"官方腔"},
		},
	})

	required := []string{
		"【角色身份】",
		"你是江梦。",
		"你在 ReEscape Protocol 中以江梦的身份与用户对话。",
		"【外显说话方式】",
		"【最终角色要求】",
	}
	for _, item := range required {
		if !strings.Contains(prompt, item) {
			t.Fatalf("expected prompt to contain %q, got %q", item, prompt)
		}
	}
	forbidden := []string{"角色扮演", "假扮"}
	for _, item := range forbidden {
		if strings.Contains(prompt, item) {
			t.Fatalf("expected prompt to avoid %q wording, got %q", item, prompt)
		}
	}
}

func TestBuildPromptUsesCharacterCanon(t *testing.T) {
	prompt := BuildPrompt(CharacterConfig{
		Name:        "江梦",
		Description: "学生",
		Background: CharacterBackground{
			Traits: []string{"礼貌、保持距离"},
			Habits: []string{"容易熬夜"},
		},
		Examples: []CharacterExample{{Situation: "熟人", User: "夸我", Reply: "6、乐"}},
	})

	required := []string{
		"【稳定背景】",
		"- 特征：礼貌、保持距离",
		"- 习惯：容易熬夜",
		"【回复样例】",
		"回复：6、乐",
	}
	for _, item := range required {
		if !strings.Contains(prompt, item) {
			t.Fatalf("expected canon detail %q in prompt, got %q", item, prompt)
		}
	}
	forbidden := []string{"旧版角色细节", "personality", "responses", "behavior", "假扮", "角色扮演"}
	for _, item := range forbidden {
		if strings.Contains(prompt, item) {
			t.Fatalf("expected prompt to avoid legacy concept %q, got %q", item, prompt)
		}
	}
}

func TestBuildToneSummaryIsShortClassifierContext(t *testing.T) {
	summary := BuildToneSummary(CharacterConfig{
		Name:        "江梦",
		Description: "学生",
		Voice: CharacterVoice{
			Tone: "轻一点",
		},
	})

	if !strings.Contains(summary, "【角色语气摘要】") {
		t.Fatalf("expected tone summary section, got %q", summary)
	}
	if strings.Contains(summary, "【最终角色要求】") || strings.Contains(summary, "工具调用") {
		t.Fatalf("tone summary should not include full prompt/tool rules, got %q", summary)
	}
}
