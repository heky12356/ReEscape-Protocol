package state

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sashabaranov/go-openai"
)

const (
	defaultRecentTurns    = 3
	defaultSummaryMaxTurn = 6
	defaultOpenLoopLimit  = 3
)

// ConversationContextLayers 提供面向 prompt assembly 的分层上下文。
type ConversationContextLayers struct {
	RecentTurns    []ConversationTurn `json:"recent_turns"`
	RollingSummary string             `json:"rolling_summary"`
	OpenLoops      []OpenLoop         `json:"open_loops"`
}

// ConversationTurn 表示一个 user turn 及其后续 assistant 回复。
type ConversationTurn struct {
	User        string `json:"user"`
	Assistant   string `json:"assistant,omitempty"`
	UserIndex   int    `json:"user_index"`
	HasReply    bool   `json:"has_reply"`
	IsUnmatched bool   `json:"is_unmatched"`
}

// OpenLoop 表示仍未闭合的对话事项。
type OpenLoop struct {
	Kind        string `json:"kind"`
	Description string `json:"description"`
	TurnIndex   int    `json:"turn_index"`
}

func buildConversationContextLayers(conversation []openai.ChatCompletionMessage, recentTurns int, summaryMaxTurns int, openLoopLimit int) ConversationContextLayers {
	turns := extractConversationTurns(conversation)
	recentTurns = clampPositive(recentTurns, defaultRecentTurns)
	summaryMaxTurns = clampPositive(summaryMaxTurns, defaultSummaryMaxTurn)
	openLoopLimit = clampPositive(openLoopLimit, defaultOpenLoopLimit)

	split := 0
	if len(turns) > recentTurns {
		split = len(turns) - recentTurns
	}

	recent := cloneConversationTurns(turns[split:])
	older := turns[:split]

	return ConversationContextLayers{
		RecentTurns:    recent,
		RollingSummary: buildRollingSummary(older, summaryMaxTurns),
		OpenLoops:      extractOpenLoops(turns, openLoopLimit),
	}
}

func extractConversationTurns(conversation []openai.ChatCompletionMessage) []ConversationTurn {
	turns := make([]ConversationTurn, 0, len(conversation)/2+1)
	currentAssistant := -1

	for idx, msg := range conversation {
		content := normalizeConversationText(chatMessagePlainText(msg))
		if content == "" {
			continue
		}

		switch msg.Role {
		case openai.ChatMessageRoleUser:
			turns = append(turns, ConversationTurn{
				User:      content,
				UserIndex: idx,
			})
			currentAssistant = len(turns) - 1
		case openai.ChatMessageRoleAssistant:
			if currentAssistant >= 0 && !turns[currentAssistant].HasReply {
				turns[currentAssistant].Assistant = content
				turns[currentAssistant].HasReply = true
				turns[currentAssistant].IsUnmatched = false
				continue
			}

			turns = append(turns, ConversationTurn{
				Assistant: content,
				UserIndex: idx,
				HasReply:  true,
			})
			currentAssistant = -1
		}
	}

	for i := range turns {
		if turns[i].User != "" && !turns[i].HasReply {
			turns[i].IsUnmatched = true
		}
	}

	return turns
}

func cloneConversationTurns(turns []ConversationTurn) []ConversationTurn {
	if len(turns) == 0 {
		return nil
	}

	result := make([]ConversationTurn, len(turns))
	copy(result, turns)
	return result
}

func buildRollingSummary(olderTurns []ConversationTurn, maxTurns int) string {
	if len(olderTurns) == 0 {
		return ""
	}

	start := 0
	if len(olderTurns) > maxTurns {
		start = len(olderTurns) - maxTurns
	}

	selected := olderTurns[start:]
	topics := summarizeTopics(selected)
	facts := summarizeStableFacts(selected)
	drift := summarizeEmotionalDrift(selected)
	commitments := summarizeCommitments(selected)

	parts := make([]string, 0, 4)
	if topics != "" {
		parts = append(parts, "Topics: "+topics)
	}
	if facts != "" {
		parts = append(parts, "Facts: "+facts)
	}
	if drift != "" {
		parts = append(parts, "Tone: "+drift)
	}
	if commitments != "" {
		parts = append(parts, "Commitments: "+commitments)
	}

	return strings.Join(parts, " | ")
}

func summarizeTopics(turns []ConversationTurn) string {
	items := make([]string, 0, len(turns))
	for _, turn := range turns {
		if turn.User == "" {
			continue
		}
		items = append(items, truncateRunes(turn.User, 28))
	}
	return joinUnique(items, 3)
}

func summarizeStableFacts(turns []ConversationTurn) string {
	facts := make([]string, 0, len(turns))
	for _, turn := range turns {
		user := turn.User
		if user == "" {
			continue
		}
		if strings.Contains(user, "我是") || strings.Contains(user, "我在") || strings.Contains(user, "我用") || strings.Contains(user, "我现在") || strings.Contains(user, "我已经") {
			facts = append(facts, truncateRunes(user, 32))
		}
	}
	return joinUnique(facts, 2)
}

func summarizeEmotionalDrift(turns []ConversationTurn) string {
	negative := 0
	positive := 0

	for _, turn := range turns {
		text := turn.User
		switch {
		case containsAny(text, "难受", "焦虑", "担心", "害怕", "烦", "崩溃", "累", "痛苦", "不想", "麻烦"):
			negative++
		case containsAny(text, "开心", "高兴", "放心", "期待", "喜欢", "顺利", "好起来", "轻松"):
			positive++
		}
	}

	switch {
	case negative > positive && negative > 0:
		return "user expressed stress or frustration"
	case positive > negative && positive > 0:
		return "user expressed positive momentum"
	default:
		return ""
	}
}

func summarizeCommitments(turns []ConversationTurn) string {
	items := make([]string, 0, len(turns))
	for _, turn := range turns {
		if turn.Assistant == "" {
			continue
		}
		if containsAny(turn.Assistant, "我会", "稍后", "接下来", "之后", "帮你", "继续", "待会") {
			items = append(items, truncateRunes(turn.Assistant, 32))
		}
	}
	return joinUnique(items, 2)
}

func extractOpenLoops(turns []ConversationTurn, limit int) []OpenLoop {
	if len(turns) == 0 || limit <= 0 {
		return nil
	}

	loops := make([]OpenLoop, 0, limit)
	seen := make(map[string]struct{})

	for idx := len(turns) - 1; idx >= 0 && len(loops) < limit; idx-- {
		turn := turns[idx]
		if turn.User != "" && !turn.HasReply {
			kind := "pending_user_thread"
			description := describeTurnLoop(turn)
			switch {
			case containsQuestion(turn.User):
				kind = "unanswered_question"
			case isProblemStatement(turn.User):
				kind = "unresolved_problem"
				description = truncateRunes(turn.User, 48)
			}
			addOpenLoop(&loops, seen, OpenLoop{
				Kind:        kind,
				Description: description,
				TurnIndex:   idx,
			}, limit)
		}

		if turn.User != "" && isProblemStatement(turn.User) && !assistantLikelyResolved(turn.Assistant) {
			addOpenLoop(&loops, seen, OpenLoop{
				Kind:        "unresolved_problem",
				Description: truncateRunes(turn.User, 48),
				TurnIndex:   idx,
			}, limit)
		}

		if turn.Assistant != "" && assistantPromisesFollowUp(turn.Assistant) {
			addOpenLoop(&loops, seen, OpenLoop{
				Kind:        "assistant_follow_up",
				Description: truncateRunes(turn.Assistant, 48),
				TurnIndex:   idx,
			}, limit)
		}
	}

	sort.Slice(loops, func(i, j int) bool {
		return loops[i].TurnIndex < loops[j].TurnIndex
	})

	return loops
}

func addOpenLoop(loops *[]OpenLoop, seen map[string]struct{}, loop OpenLoop, limit int) {
	if len(*loops) >= limit {
		return
	}
	key := loop.Kind + "::" + normalizeDedupKey(loop.Description)
	if _, ok := seen[key]; ok {
		return
	}
	seen[key] = struct{}{}
	*loops = append(*loops, loop)
}

func describeTurnLoop(turn ConversationTurn) string {
	text := truncateRunes(turn.User, 48)
	if containsQuestion(turn.User) {
		return text
	}
	return fmt.Sprintf("Pending user thread: %s", text)
}

func containsQuestion(text string) bool {
	return strings.Contains(text, "?") || strings.Contains(text, "？") || strings.Contains(text, "怎么") || strings.Contains(text, "为什么") || strings.Contains(text, "如何")
}

func isProblemStatement(text string) bool {
	return containsAny(text, "问题", "报错", "失败", "不行", "不会", "卡住", "麻烦", "怎么", "无法", "需要帮")
}

func assistantLikelyResolved(text string) bool {
	if assistantPromisesFollowUp(text) {
		return false
	}
	return containsAny(text, "可以这样", "解决", "已经", "步骤", "先", "然后", "建议", "做法")
}

func assistantPromisesFollowUp(text string) bool {
	return containsAny(text, "我会", "稍后", "之后", "待会", "下次", "回头", "继续帮你")
}

func normalizeConversationText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	return strings.Join(strings.Fields(text), " ")
}

func normalizeDedupKey(text string) string {
	text = strings.ToLower(text)
	text = strings.ReplaceAll(text, "？", "?")
	text = strings.ReplaceAll(text, "，", ",")
	return strings.Join(strings.Fields(text), " ")
}

func joinUnique(items []string, limit int) string {
	if limit <= 0 {
		return ""
	}

	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, limit)
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
		if len(result) == limit {
			break
		}
	}

	return strings.Join(result, "; ")
}

func containsAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func clampPositive(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
