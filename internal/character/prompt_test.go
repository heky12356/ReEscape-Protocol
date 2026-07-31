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

func TestBuildPromptKeepsLegacyFields(t *testing.T) {
	prompt := BuildPrompt(CharacterConfig{
		Name:        "江梦",
		Description: "学生",
		Personality: map[string]string{
			"outer": "礼貌、保持距离",
		},
		Responses: map[string]interface{}{
			"familiar": []interface{}{"6", "乐"},
		},
		Behavior: map[string]interface{}{
			"sleep": "容易熬夜",
		},
		Quotes: []string{"不行，我需要点熟人buff再说话"},
	})

	required := []string{
		"【旧版角色细节】",
		"- outer：礼貌、保持距离",
		"- familiar：6、乐",
		"- sleep：容易熬夜",
		"不行，我需要点熟人buff再说话",
	}
	for _, item := range required {
		if !strings.Contains(prompt, item) {
			t.Fatalf("expected legacy detail %q in prompt, got %q", item, prompt)
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
