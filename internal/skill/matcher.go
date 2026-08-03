package skill

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

func MatchPackages(packages []Package, input MatchInput) []Match {
	limit := input.Limit
	if limit <= 0 {
		limit = 3
	}

	matches := make([]Match, 0, len(packages))
	for _, pkg := range packages {
		if !pkg.Enabled {
			continue
		}
		score, reasons := scorePackage(pkg, input)
		if score <= 0 {
			continue
		}
		matches = append(matches, Match{
			Name:        pkg.Name,
			Description: pkg.Description,
			Score:       score,
			Reason:      strings.Join(reasons, ", "),
			Scope:       pkg.Scope,
		})
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Score == matches[j].Score {
			return matches[i].Name < matches[j].Name
		}
		return matches[i].Score > matches[j].Score
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}
	return matches
}

func scorePackage(pkg Package, input MatchInput) (int, []string) {
	nameTerms := tokenize(pkg.Name)
	descriptionTerms := tokenize(pkg.Description)
	tagTerms := frontmatterStrings(pkg.Frontmatter, "tags")
	triggerTerms := frontmatterStrings(pkg.Frontmatter, "triggers")
	queryTerms := tokenize(strings.Join([]string{
		input.Message,
		input.Trigger,
		input.DialogueState.Emotion,
		input.DialogueState.Intention,
		input.DialogueState.ReplyExpectation,
		input.DialogueState.SupportStrategy,
		input.DialogueState.Topic,
		input.DialogueState.UserNeed,
	}, " "))

	score := 0
	reasons := make([]string, 0, 3)
	if matched := countTermMatches(queryTerms, nameTerms); matched > 0 {
		score += matched * 5
		reasons = append(reasons, "matched skill name")
	}
	if matched := countTermMatches(queryTerms, descriptionTerms); matched > 0 {
		score += matched * 3
		reasons = append(reasons, "matched description keywords")
	}
	if matched := countTermMatches(queryTerms, tagTerms); matched > 0 {
		score += matched * 5
		reasons = append(reasons, "matched tags")
	}
	if matched := countTermMatches(queryTerms, triggerTerms); matched > 0 {
		score += matched * 6
		reasons = append(reasons, "matched triggers")
	}

	stateTerms := tokenize(strings.Join([]string{
		input.DialogueState.Emotion,
		input.DialogueState.Intention,
		input.DialogueState.ReplyExpectation,
		input.DialogueState.SupportStrategy,
		input.DialogueState.Topic,
		input.DialogueState.UserNeed,
	}, " "))
	if matched := countTermMatches(stateTerms, append(append([]string(nil), tagTerms...), triggerTerms...)); matched > 0 {
		score += matched * 4
		reasons = append(reasons, "matched dialogue state")
	}

	if message := strings.TrimSpace(input.Message); message != "" {
		allText := strings.ToLower(strings.Join([]string{pkg.Name, pkg.Description}, " "))
		if strings.Contains(allText, strings.ToLower(message)) {
			score += 8
			reasons = append(reasons, "matched message phrase")
		}
	}
	if len(reasons) == 0 && strings.TrimSpace(input.Trigger) != "" {
		if countTermMatches(tokenize(input.Trigger), triggerTerms) > 0 {
			score += 6
			reasons = append(reasons, "matched trigger")
		}
	}
	return score, uniqueStrings(reasons)
}

func countTermMatches(queryTerms, packageTerms []string) int {
	if len(queryTerms) == 0 || len(packageTerms) == 0 {
		return 0
	}
	matched := 0
	seen := make(map[string]struct{})
	for _, query := range queryTerms {
		for _, term := range packageTerms {
			if query == "" || term == "" {
				continue
			}
			if query == term || strings.Contains(query, term) || strings.Contains(term, query) {
				if _, ok := seen[term]; ok {
					continue
				}
				seen[term] = struct{}{}
				matched++
				break
			}
		}
	}
	return matched
}

func frontmatterStrings(frontmatter map[string]any, key string) []string {
	value, ok := frontmatter[key]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case string:
		return tokenize(typed)
	case []string:
		var result []string
		for _, item := range typed {
			result = append(result, tokenize(item)...)
		}
		return result
	case []any:
		var result []string
		for _, item := range typed {
			if text, ok := item.(string); ok {
				result = append(result, tokenize(text)...)
			}
		}
		return result
	default:
		return nil
	}
}

func tokenize(input string) []string {
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" {
		return nil
	}

	var result []string
	var current []rune
	flush := func() {
		if len(current) == 0 {
			return
		}
		term := string(current)
		result = append(result, term)
		if containsCJK(current) {
			for size := 2; size <= 4 && size <= len(current); size++ {
				for i := 0; i+size <= len(current); i++ {
					result = append(result, string(current[i:i+size]))
				}
			}
		}
		current = nil
	}
	for _, char := range []rune(input) {
		if unicode.IsLetter(char) || unicode.IsNumber(char) {
			current = append(current, char)
			continue
		}
		flush()
	}
	flush()
	return uniqueStrings(result)
}

func containsCJK(value []rune) bool {
	for _, char := range value {
		if unicode.In(char, unicode.Han) {
			return true
		}
	}
	return false
}

func uniqueStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func formatMatchReason(match Match) string {
	if match.Reason == "" {
		return fmt.Sprintf("matched score %d", match.Score)
	}
	return match.Reason
}
