package service

import (
	"fmt"
	"strings"

	"project-yume/internal/memory"
	"project-yume/internal/state"
)

type PromptMemory struct {
	ShortTermSummary string
	ActiveTopics     []string
	Profile          memory.UserProfile
	Facts            []memory.FactMemory
	EmotionalPattern string
	RecentEmotions   []string
}

func BuildPromptMemory(userID int64, sessionID, currentMessage string) PromptMemory {
	stateManager := state.GetManager()
	emotionalManager := memory.GetManager()

	return PromptMemory{
		ShortTermSummary: stateManager.GetConversationSummary(sessionID),
		ActiveTopics:     stateManager.GetActiveTopics(sessionID),
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
	lines := make([]string, 0, 2)
	if promptMemory.ShortTermSummary != "" {
		lines = append(lines, "最近对话摘要："+promptMemory.ShortTermSummary)
	}
	if len(promptMemory.ActiveTopics) > 0 {
		lines = append(lines, "当前活跃话题："+strings.Join(promptMemory.ActiveTopics, "、"))
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
