package skill

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
)

const DefaultCandidateMinScore = 8

var matchStopwords = map[string]struct{}{
	"ai":      {},
	"chat":    {},
	"current": {},
	"message": {},
	"use":     {},
	"user":    {},
	"when":    {},
	"聊天":      {},
	"回复":      {},
	"用户":      {},
}

func MatchPackages(packages []Package, input MatchInput) []Match {
	limit := input.Limit
	if limit <= 0 {
		limit = 3
	}
	minScore := input.MinScore
	if minScore <= 0 {
		minScore = DefaultCandidateMinScore
	}

	matches := make([]Match, 0, len(packages))
	for _, pkg := range packages {
		if !pkg.Enabled {
			continue
		}
		score, confidence, reasons := scorePackage(pkg, input)
		if score < minScore {
			continue
		}
		matches = append(matches, Match{
			Name:        pkg.Name,
			Description: pkg.Description,
			Score:       score,
			Confidence:  confidence,
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
	adjustMatchConfidenceForRanking(matches)
	if len(matches) > limit {
		matches = matches[:limit]
	}
	return matches
}

func scorePackage(pkg Package, input MatchInput) (int, float64, []string) {
	descriptionTerms := tokenize(pkg.Description)
	tagPhrases := frontmatterRawStrings(pkg.Frontmatter, "tags")
	triggerPhrases := frontmatterRawStrings(pkg.Frontmatter, "triggers")
	tagTerms := tokenize(strings.Join(tagPhrases, " "))
	triggerTerms := tokenize(strings.Join(triggerPhrases, " "))
	messageTerms := tokenize(input.Message)

	score := 0
	reasons := make([]string, 0, 5)
	exactTrigger := countPhraseMatches(input.Message, triggerPhrases)
	if exactTrigger == 0 && !isRuntimeTrigger(input.Trigger) {
		exactTrigger = countPhraseMatches(input.Trigger, triggerPhrases)
	}
	if exactTrigger > 0 {
		score += 20
		reasons = append(reasons, "matched exact trigger")
	}

	exactTag := countPhraseMatches(input.Message, tagPhrases)
	if exactTag > 0 {
		score += min(exactTag, 2) * 12
		reasons = append(reasons, "matched exact tag")
	}

	exactName := containsPhrase(input.Message, pkg.Name)
	if exactName {
		score += 12
		reasons = append(reasons, "matched skill name")
	}

	descriptionMatches := countExactTermMatches(messageTerms, descriptionTerms)
	if descriptionMatches > 0 {
		score += min(descriptionMatches, 4) * 2
		reasons = append(reasons, "matched description keywords")
	}

	stateTerms := tokenize(strings.Join([]string{
		input.DialogueState.Emotion,
		input.DialogueState.Intention,
		input.DialogueState.ReplyExpectation,
		input.DialogueState.SupportStrategy,
		input.DialogueState.Topic,
		input.DialogueState.UserNeed,
	}, " "))
	if score > 0 {
		stateMatches := countExactTermMatches(stateTerms, append(append([]string(nil), tagTerms...), triggerTerms...))
		if stateMatches > 0 {
			score += min(stateMatches, 2) * 2
			reasons = append(reasons, "matched dialogue state")
		}
	}

	if exactTrigger == 0 {
		triggerTokenMatches := countExactTermMatches(messageTerms, triggerTerms)
		if triggerTokenMatches >= 2 {
			score += min(triggerTokenMatches, 3) * 3
			reasons = append(reasons, "matched trigger keywords")
		}
	}
	if exactTag == 0 {
		tagTokenMatches := countExactTermMatches(messageTerms, tagTerms)
		if tagTokenMatches >= 2 {
			score += min(tagTokenMatches, 2) * 4
			reasons = append(reasons, "matched tag keywords")
		}
	}

	confidence := matchConfidence(score, exactTrigger > 0, exactTag > 0, exactName, descriptionMatches)
	return score, confidence, uniqueStrings(reasons)
}

func matchConfidence(score int, exactTrigger, exactTag, exactName bool, descriptionMatches int) float64 {
	switch {
	case exactTrigger:
		return 0.95
	case exactTag:
		return 0.84
	case exactName:
		return 0.82
	case descriptionMatches > 0:
		return math.Min(0.65, 0.35+float64(score)*0.02)
	default:
		return 0
	}
}

func adjustMatchConfidenceForRanking(matches []Match) {
	if len(matches) == 0 {
		return
	}
	if len(matches) == 1 {
		matches[0].Confidence = clampConfidence(matches[0].Confidence + 0.03)
		return
	}

	margin := matches[0].Score - matches[1].Score
	switch {
	case margin <= 2:
		matches[0].Confidence = clampConfidence(matches[0].Confidence - 0.12)
	case margin >= 8:
		matches[0].Confidence = clampConfidence(matches[0].Confidence + 0.03)
	}
}

func clampConfidence(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return math.Round(value*100) / 100
}

func countPhraseMatches(text string, phrases []string) int {
	matched := 0
	for _, phrase := range uniqueStrings(phrases) {
		if containsPhrase(text, phrase) {
			matched++
		}
	}
	return matched
}

func containsPhrase(text, phrase string) bool {
	text = normalizePhrase(text)
	phrase = normalizePhrase(phrase)
	if text == "" || phrase == "" {
		return false
	}
	if containsCJK([]rune(phrase)) {
		return strings.Contains(strings.ReplaceAll(text, " ", ""), strings.ReplaceAll(phrase, " ", ""))
	}
	return strings.Contains(" "+text+" ", " "+phrase+" ")
}

func normalizePhrase(input string) string {
	var result []rune
	spacePending := false
	for _, char := range []rune(strings.ToLower(strings.TrimSpace(input))) {
		if unicode.IsLetter(char) || unicode.IsNumber(char) {
			if spacePending && len(result) > 0 {
				result = append(result, ' ')
			}
			result = append(result, char)
			spacePending = false
			continue
		}
		spacePending = true
	}
	return strings.Join(strings.Fields(string(result)), " ")
}

func isRuntimeTrigger(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "message", "proactive":
		return true
	default:
		return false
	}
}

func countExactTermMatches(queryTerms, packageTerms []string) int {
	if len(queryTerms) == 0 || len(packageTerms) == 0 {
		return 0
	}
	querySet := make(map[string]struct{}, len(queryTerms))
	for _, term := range queryTerms {
		querySet[term] = struct{}{}
	}
	matched := 0
	for _, term := range uniqueStrings(packageTerms) {
		if _, ok := querySet[term]; ok {
			matched++
		}
	}
	return matched
}

func frontmatterRawStrings(frontmatter map[string]any, key string) []string {
	value, ok := frontmatter[key]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case string:
		return []string{typed}
	case []string:
		return append([]string(nil), typed...)
	case []any:
		var result []string
		for _, item := range typed {
			if text, ok := item.(string); ok {
				result = append(result, text)
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
	var latin []rune
	var cjk []rune
	flushLatin := func() {
		if len(latin) == 0 {
			return
		}
		result = appendMeaningfulTerm(result, string(latin))
		latin = nil
	}
	flushCJK := func() {
		if len(cjk) == 0 {
			return
		}
		if len(cjk) <= 4 {
			result = appendMeaningfulTerm(result, string(cjk))
		}
		for size := 2; size <= 4 && size <= len(cjk); size++ {
			for i := 0; i+size <= len(cjk); i++ {
				result = appendMeaningfulTerm(result, string(cjk[i:i+size]))
			}
		}
		cjk = nil
	}

	for _, char := range []rune(input) {
		switch {
		case unicode.In(char, unicode.Han):
			flushLatin()
			cjk = append(cjk, char)
		case unicode.IsLetter(char) || unicode.IsNumber(char):
			flushCJK()
			latin = append(latin, char)
		default:
			flushLatin()
			flushCJK()
		}
	}
	flushLatin()
	flushCJK()
	return uniqueStrings(result)
}

func appendMeaningfulTerm(values []string, term string) []string {
	term = strings.TrimSpace(term)
	if term == "" {
		return values
	}
	if _, stopped := matchStopwords[term]; stopped {
		return values
	}
	if !containsCJK([]rune(term)) && len([]rune(term)) < 2 {
		return values
	}
	return append(values, term)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
