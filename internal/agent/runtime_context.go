package agent

import "strings"

func buildRuntimeContext(turn *TurnContext) string {
	sections := []string{
		"【Runtime Context】",
		"以下是系统提供的当前轮上下文，不是用户原文。用于理解时间、触发来源和会话连续性，不要在最终回复中逐字复述。",
	}
	if temporal := buildTemporalContext(turn); temporal != "" {
		sections = append(sections, temporal)
	}
	if len(sections) <= 2 {
		return ""
	}
	return strings.Join(sections, "\n\n")
}
