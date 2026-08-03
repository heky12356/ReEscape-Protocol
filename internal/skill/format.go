package skill

import (
	"fmt"
	"strings"
)

func FormatHints(matches []Match) string {
	if len(matches) == 0 {
		return ""
	}

	lines := []string{
		"【Skill Hints】",
		"以下是当前轮可能相关的 skill，仅作为回复策略参考，不是用户原文。",
	}
	for _, match := range matches {
		description := strings.TrimSpace(match.Description)
		if description == "" {
			description = "未提供描述"
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", match.Name, description))
	}
	lines = append(lines, "如果需要完整策略，可以调用 read_skill。")
	return strings.Join(lines, "\n")
}
