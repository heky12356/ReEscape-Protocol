package service

import (
	"regexp"
	"strings"
	"time"

	"project-yume/internal/memory"
)

var (
	likePattern         = regexp.MustCompile(`^我(?:很)?喜欢(.+)$`)
	dislikePattern      = regexp.MustCompile(`^我(?:很)?不喜欢(.+)$`)
	namePattern         = regexp.MustCompile(`^我叫(.+)$`)
	identityPattern     = regexp.MustCompile(`^我是(.+)$`)
	locationPattern     = regexp.MustCompile(`^我住在(.+)$`)
	petPattern          = regexp.MustCompile(`^我(?:养了|有一只|有个)(.+)$`)
	currentPlanPattern  = regexp.MustCompile(`^我(?:最近在|正在)(.+)$`)
	tomorrowPlanPattern = regexp.MustCompile(`^我明天要(.+)$`)
	todayPlanPattern    = regexp.MustCompile(`^我今天要(.+)$`)
)

func UpdateLongTermMemory(sessionID string, userID int64, userMsg, botReply, emotion, intention string) {
	memory.GetManager().RecordInteraction(userID, userMsg, botReply, emotion, intention)

	profilePatch, factCandidates := ExtractStructuredMemory(userMsg)
	memory.GetProfileManager().ApplyPatch(userID, profilePatch)
	memory.GetFactManager().UpsertFacts(userID, sessionID, factCandidates)
}

func ExtractStructuredMemory(message string) (memory.ProfilePatch, []memory.FactMemory) {
	trimmed := normalizeMemoryText(message)
	if trimmed == "" {
		return memory.ProfilePatch{}, nil
	}

	patch := memory.ProfilePatch{}
	facts := make([]memory.FactMemory, 0)

	switch {
	case containsAny(trimmed, "温柔一点", "说话温柔点"):
		patch.PreferredTone = "温柔"
	case containsAny(trimmed, "直接一点", "说话直接点"):
		patch.PreferredTone = "直接"
	case containsAny(trimmed, "简短一点", "说短一点"):
		patch.ReplyStyle = "简短"
	case containsAny(trimmed, "像朋友一点", "像朋友那样"):
		patch.RelationshipStyle = "朋友式"
	}

	if taboo := extractSuffix(trimmed, "别总是"); taboo != "" {
		patch.Taboos = append(patch.Taboos, taboo)
	}
	if taboo := extractSuffix(trimmed, "不要再"); taboo != "" {
		patch.Taboos = append(patch.Taboos, taboo)
	}

	if match := firstMatch(likePattern, trimmed); match != "" {
		object := cleanMemoryObject(match)
		patch.Likes = append(patch.Likes, object)
		facts = append(facts, newFact("likes", object, "用户喜欢"+object, trimmed, nil, "preference", object))
	}

	if match := firstMatch(dislikePattern, trimmed); match != "" {
		object := cleanMemoryObject(match)
		patch.Dislikes = append(patch.Dislikes, object)
		facts = append(facts, newFact("dislikes", object, "用户不喜欢"+object, trimmed, nil, "preference", object))
	}

	if match := firstMatch(namePattern, trimmed); match != "" {
		object := cleanMemoryObject(match)
		facts = append(facts, newFact("name", object, "用户名字是"+object, trimmed, nil, "identity", "name"))
	}

	if match := firstMatch(identityPattern, trimmed); match != "" {
		object := cleanMemoryObject(match)
		facts = append(facts, newFact("identity", object, "用户身份是"+object, trimmed, nil, "identity", object))
	}

	if match := firstMatch(locationPattern, trimmed); match != "" {
		object := cleanMemoryObject(match)
		facts = append(facts, newFact("location", object, "用户住在"+object, trimmed, nil, "location", object))
	}

	if match := firstMatch(petPattern, trimmed); match != "" {
		object := cleanMemoryObject(match)
		facts = append(facts, newFact("pet", object, "用户养了"+object, trimmed, nil, "pet", object))
	}

	if match := firstMatch(currentPlanPattern, trimmed); match != "" {
		object := cleanMemoryObject(match)
		expiresAt := time.Now().Add(7 * 24 * time.Hour)
		facts = append(facts, newFact("current_plan", object, "用户最近在"+object, trimmed, &expiresAt, "recent", "plan"))
	}

	if match := firstMatch(todayPlanPattern, trimmed); match != "" {
		object := cleanMemoryObject(match)
		expiresAt := time.Now().Add(36 * time.Hour)
		facts = append(facts, newFact("today_plan", object, "用户今天要"+object, trimmed, &expiresAt, "today", "plan"))
	}

	if match := firstMatch(tomorrowPlanPattern, trimmed); match != "" {
		object := cleanMemoryObject(match)
		expiresAt := time.Now().Add(48 * time.Hour)
		facts = append(facts, newFact("tomorrow_plan", object, "用户明天要"+object, trimmed, &expiresAt, "tomorrow", "plan"))
	}

	return patch, facts
}

func newFact(predicate, object, summary, source string, expiresAt *time.Time, tags ...string) memory.FactMemory {
	return memory.FactMemory{
		Predicate:     predicate,
		Object:        object,
		Summary:       summary,
		Tags:          tags,
		Confidence:    0.7,
		SourceMessage: source,
		Status:        memory.FactStatusActive,
		ExpiresAt:     expiresAt,
	}
}

func normalizeMemoryText(input string) string {
	return strings.TrimSpace(strings.ReplaceAll(input, "\n", " "))
}

func cleanMemoryObject(input string) string {
	trimmed := strings.TrimSpace(input)
	trimmed = strings.TrimRight(trimmed, "，。！？,.!? ")
	return trimmed
}

func firstMatch(pattern *regexp.Regexp, input string) string {
	matches := pattern.FindStringSubmatch(input)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

func containsAny(input string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(input, needle) {
			return true
		}
	}
	return false
}

func extractSuffix(input, prefix string) string {
	if !strings.HasPrefix(input, prefix) {
		return ""
	}
	return cleanMemoryObject(strings.TrimPrefix(input, prefix))
}
