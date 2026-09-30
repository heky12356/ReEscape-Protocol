package service

import "strings"

// BuildEmotionalContext formats persistent emotional memory as runtime context.
// It intentionally describes trends, not hidden numeric state.
func BuildEmotionalContext(pattern string, recentEmotions []string) string {
	lines := make([]string, 0, 3)
	switch strings.TrimSpace(pattern) {
	case "需要关怀":
		lines = append(lines, "用户近期更需要关怀和安慰，请保持温暖、体贴的语气。")
	case "积极活跃":
		lines = append(lines, "用户近期情绪积极，可以保持轻松、愉快的对话氛围。")
	case "情绪波动":
		lines = append(lines, "用户近期情绪有波动，请保持平和、耐心，避免激烈表达。")
	case "新用户":
		lines = append(lines, "用户近期是新关系阶段，请保持友善、自然。")
	default:
		if strings.TrimSpace(pattern) != "" {
			lines = append(lines, "用户近期情绪模式："+strings.TrimSpace(pattern)+"。")
		}
	}
	if len(recentEmotions) > 0 {
		lines = append(lines, "最近情绪趋势："+strings.Join(recentEmotions, " → "))
	}
	if len(lines) == 0 {
		return ""
	}
	return "【情绪记忆】\n" + strings.Join(lines, "\n")
}
