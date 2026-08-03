package agent

import (
	"strings"

	"project-yume/internal/config"
	"project-yume/internal/skill"
	"project-yume/internal/state"
)

func buildRuntimeContext(turn *TurnContext) string {
	sections := []string{
		"【Runtime Context】",
		"以下是系统提供的当前轮上下文，不是用户原文。用于理解时间、触发来源和会话连续性，不要在最终回复中逐字复述。",
	}
	if temporal := buildTemporalContext(turn); temporal != "" {
		sections = append(sections, temporal)
	}
	if skillHints := buildSkillHintsContext(turn); skillHints != "" {
		sections = append(sections, skillHints)
	}
	if len(sections) <= 2 {
		return ""
	}
	return strings.Join(sections, "\n\n")
}

func buildSkillHintsContext(turn *TurnContext) string {
	cfg := config.GetConfig()
	if turn == nil || cfg == nil || !cfg.EnableSkills {
		return ""
	}
	limit := cfg.SkillAutoHintLimit
	if limit <= 0 {
		limit = 3
	}
	matches := skill.GetManager().Match(skill.MatchInput{
		Message:       turn.Message(),
		Trigger:       turn.Trigger(),
		DialogueState: state.GetManager().GetDialogueState(turn.SessionID()),
		Limit:         limit,
	})
	return skill.FormatHints(matches)
}
