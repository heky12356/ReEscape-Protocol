package service

import (
	"fmt"
	"strings"

	"project-yume/internal/config"
	"project-yume/internal/memory"
	"project-yume/internal/state"
)

type PromptMemory struct {
	ContextLayers    state.ConversationContextLayers
	Profile          memory.UserProfile
	Facts            []memory.FactMemory
	EmotionalPattern string
	RecentEmotions   []string
}

func BuildPromptMemory(userID int64, sessionID, currentMessage string) PromptMemory {
	stateManager := state.GetManager()
	emotionalManager := memory.GetManager()
	cfg := config.GetConfig()
	layers := stateManager.BuildConversationContextLayers(
		sessionID,
		cfg.ContextRecentTurns,
		cfg.ContextSummaryMaxTurns,
		cfg.ContextOpenLoopLimit,
	)

	return PromptMemory{
		ContextLayers:    normalizePromptContextLayers(layers, currentMessage),
		Profile:          memory.GetProfileManager().GetProfile(userID),
		Facts:            memory.GetFactManager().FindRelevantFacts(userID, currentMessage, 4),
		EmotionalPattern: emotionalManager.GetConversationPattern(userID),
		RecentEmotions:   emotionalManager.GetRecentEmotions(userID, 5),
	}
}

func FormatPromptMemory(promptMemory PromptMemory) string {
	parts := make([]string, 0, 4)

	if shortTerm := formatShortTermMemory(promptMemory); shortTerm != "" {
		parts = append(parts, shortTerm)
	}

	if profile := formatProfileMemory(promptMemory.Profile); profile != "" {
		parts = append(parts, profile)
	}

	if facts := formatFactMemory(promptMemory.Facts); facts != "" {
		parts = append(parts, facts)
	}

	if emotional := BuildEmotionalContext(promptMemory.EmotionalPattern, promptMemory.RecentEmotions); emotional != "" {
		parts = append(parts, emotional)
	}

	return strings.Join(parts, "\n\n")
}

func formatShortTermMemory(promptMemory PromptMemory) string {
	lines := make([]string, 0, 3)
	if recent := formatRecentTurns(promptMemory.ContextLayers.RecentTurns); recent != "" {
		lines = append(lines, "最近回合：\n"+recent)
	}
	if summary := strings.TrimSpace(promptMemory.ContextLayers.RollingSummary); summary != "" {
		lines = append(lines, "滚动摘要："+summary)
	}
	if loops := formatOpenLoops(promptMemory.ContextLayers.OpenLoops); loops != "" {
		lines = append(lines, "未闭合事项：\n"+loops)
	}
	if len(lines) == 0 {
		return ""
	}
	return "【短期上下文】\n" + strings.Join(lines, "\n")
}

func formatProfileMemory(profile memory.UserProfile) string {
	lines := make([]string, 0, 6)
	if profile.PreferredTone != "" {
		lines = append(lines, "偏好语气："+profile.PreferredTone)
	}
	if profile.ReplyStyle != "" {
		lines = append(lines, "回复风格："+profile.ReplyStyle)
	}
	if profile.RelationshipStyle != "" {
		lines = append(lines, "互动关系："+profile.RelationshipStyle)
	}
	if len(profile.Likes) > 0 {
		lines = append(lines, "喜欢："+strings.Join(profile.Likes, "、"))
	}
	if len(profile.Dislikes) > 0 {
		lines = append(lines, "不喜欢："+strings.Join(profile.Dislikes, "、"))
	}
	if len(profile.Taboos) > 0 {
		lines = append(lines, "避免："+strings.Join(profile.Taboos, "、"))
	}
	if len(lines) == 0 {
		return ""
	}
	return "【长期偏好】\n" + strings.Join(lines, "\n")
}

func formatFactMemory(facts []memory.FactMemory) string {
	if len(facts) == 0 {
		return ""
	}

	lines := make([]string, 0, len(facts))
	for _, fact := range facts {
		lines = append(lines, fmt.Sprintf("- %s", fact.Summary))
	}

	return "【事实记忆】\n" + strings.Join(lines, "\n")
}

func normalizePromptContextLayers(layers state.ConversationContextLayers, currentMessage string) state.ConversationContextLayers {
	normalizedCurrent := normalizePromptText(currentMessage)
	if normalizedCurrent == "" || len(layers.RecentTurns) == 0 {
		return layers
	}

	last := layers.RecentTurns[len(layers.RecentTurns)-1]
	if last.Assistant == "" && normalizePromptText(last.User) == normalizedCurrent {
		layers.RecentTurns = append([]state.ConversationTurn(nil), layers.RecentTurns[:len(layers.RecentTurns)-1]...)
	}

	return layers
}

func formatRecentTurns(turns []state.ConversationTurn) string {
	if len(turns) == 0 {
		return ""
	}

	lines := make([]string, 0, len(turns)*2)
	for _, turn := range turns {
		if text := strings.TrimSpace(turn.User); text != "" {
			lines = append(lines, "user: "+text)
		}
		if text := strings.TrimSpace(turn.Assistant); text != "" {
			lines = append(lines, "assistant: "+text)
		}
	}

	return strings.Join(lines, "\n")
}

func formatOpenLoops(loops []state.OpenLoop) string {
	if len(loops) == 0 {
		return ""
	}

	lines := make([]string, 0, len(loops))
	for _, loop := range loops {
		description := strings.TrimSpace(loop.Description)
		if description == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("- [%s] %s", mapOpenLoopKind(loop.Kind), description))
	}

	return strings.Join(lines, "\n")
}

func mapOpenLoopKind(kind string) string {
	switch strings.TrimSpace(kind) {
	case "unanswered_question":
		return "待回答问题"
	case "unresolved_problem":
		return "待解决问题"
	case "assistant_follow_up":
		return "待跟进承诺"
	case "pending_user_thread":
		return "待继续话题"
	default:
		return "未闭合事项"
	}
}

func normalizePromptText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.Join(strings.Fields(value), " ")
}
