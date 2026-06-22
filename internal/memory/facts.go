package memory

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"project-yume/internal/storage"
	"project-yume/internal/utils"
)

const (
	FactStatusActive       = "active"
	FactStatusStale        = "stale"
	FactStatusContradicted = "contradicted"
)

type FactMemory struct {
	ID              string     `json:"id"`
	UserID          int64      `json:"user_id"`
	SessionID       string     `json:"session_id"`
	Predicate       string     `json:"predicate"`
	Object          string     `json:"object"`
	Summary         string     `json:"summary"`
	Tags            []string   `json:"tags"`
	Confidence      float64    `json:"confidence"`
	SourceMessage   string     `json:"source_message"`
	Status          string     `json:"status"`
	CreatedAt       time.Time  `json:"created_at"`
	LastConfirmedAt time.Time  `json:"last_confirmed_at"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
}

type FactManager struct {
	mu    sync.RWMutex
	facts map[int64][]*FactMemory
	store storage.SnapshotStore
	dirty storage.DirtyMarker
}

var factManager *FactManager

const FactSnapshotName = "memory/fact_memories.json"
const FactFlushTaskName = "fact_memories"

var exclusiveFactPredicates = map[string]struct{}{
	"name":         {},
	"identity":     {},
	"location":     {},
	"current_plan": {},
}

func init() {
	factManager = &FactManager{
		facts: make(map[int64][]*FactMemory),
	}
}

func GetFactManager() *FactManager {
	return factManager
}

func (fm *FactManager) UpsertFacts(userID int64, sessionID string, candidates []FactMemory) {
	if userID == 0 || len(candidates) == 0 {
		return
	}

	now := time.Now()

	fm.mu.Lock()
	fm.ensureFactsLocked(userID)

	for _, candidate := range candidates {
		candidate = normalizeFactCandidate(candidate)
		if candidate.Predicate == "" || candidate.Object == "" || candidate.Summary == "" {
			continue
		}

		candidate.UserID = userID
		candidate.SessionID = sessionID
		if candidate.Status == "" {
			candidate.Status = FactStatusActive
		}
		if candidate.Confidence <= 0 {
			candidate.Confidence = 0.6
		}

		if _, ok := exclusiveFactPredicates[candidate.Predicate]; ok {
			for _, existing := range fm.facts[userID] {
				if existing == nil || existing.Status != FactStatusActive {
					continue
				}
				if existing.Predicate == candidate.Predicate && existing.Object != candidate.Object {
					existing.Status = FactStatusContradicted
				}
			}
		}

		if existing := fm.findExactFactLocked(userID, candidate.Predicate, candidate.Object); existing != nil {
			existing.Summary = candidate.Summary
			existing.Tags = mergeUniqueStrings(existing.Tags, candidate.Tags)
			existing.SourceMessage = candidate.SourceMessage
			existing.Status = FactStatusActive
			existing.LastConfirmedAt = now
			if candidate.ExpiresAt != nil {
				existing.ExpiresAt = candidate.ExpiresAt
			}
			if candidate.Confidence > existing.Confidence {
				existing.Confidence = candidate.Confidence
			}
			continue
		}

		record := candidate
		record.ID = utils.NewRequestID("fact")
		record.CreatedAt = now
		record.LastConfirmedAt = now
		record.Tags = mergeUniqueStrings(nil, record.Tags)
		fm.facts[userID] = append(fm.facts[userID], &record)
	}

	fm.mu.Unlock()
	fm.markDirty()
}

func (fm *FactManager) FindRelevantFacts(userID int64, query string, limit int) []FactMemory {
	if limit <= 0 {
		limit = 5
	}

	fm.mu.Lock()
	fm.expireFactsLocked(userID, time.Now())
	fm.mu.Unlock()

	fm.mu.RLock()
	defer fm.mu.RUnlock()

	source := fm.facts[userID]
	if len(source) == 0 {
		return []FactMemory{}
	}

	type scoredFact struct {
		fact  FactMemory
		score int
	}

	keywords := extractMemoryKeywords(query)
	scored := make([]scoredFact, 0, len(source))
	now := time.Now()

	for _, fact := range source {
		if fact == nil || fact.Status != FactStatusActive {
			continue
		}
		if fact.ExpiresAt != nil && fact.ExpiresAt.Before(now) {
			continue
		}

		score := scoreFact(fact, keywords)
		if score == 0 && len(keywords) > 0 {
			continue
		}

		scored = append(scored, scoredFact{
			fact: FactMemory{
				ID:              fact.ID,
				UserID:          fact.UserID,
				SessionID:       fact.SessionID,
				Predicate:       fact.Predicate,
				Object:          fact.Object,
				Summary:         fact.Summary,
				Tags:            append([]string(nil), fact.Tags...),
				Confidence:      fact.Confidence,
				SourceMessage:   fact.SourceMessage,
				Status:          fact.Status,
				CreatedAt:       fact.CreatedAt,
				LastConfirmedAt: fact.LastConfirmedAt,
				ExpiresAt:       cloneTimePointer(fact.ExpiresAt),
			},
			score: score,
		})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].fact.LastConfirmedAt.After(scored[j].fact.LastConfirmedAt)
		}
		return scored[i].score > scored[j].score
	})

	if len(scored) > limit {
		scored = scored[:limit]
	}

	result := make([]FactMemory, 0, len(scored))
	for _, item := range scored {
		result = append(result, item.fact)
	}
	return result
}

func (fm *FactManager) ConfigurePersistence(store storage.SnapshotStore, dirty storage.DirtyMarker) error {
	fm.mu.Lock()
	fm.store = store
	fm.dirty = dirty
	fm.mu.Unlock()

	if store == nil {
		return nil
	}

	data, err := store.Load(FactSnapshotName)
	if err != nil {
		return fmt.Errorf("load facts failed: %w", err)
	}
	if len(data) == 0 {
		return nil
	}

	loaded := make(map[int64][]*FactMemory)
	if err := json.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("unmarshal facts failed: %w", err)
	}

	fm.mu.Lock()
	fm.facts = loaded
	fm.normalizeFactsLocked()
	fm.mu.Unlock()
	return nil
}

func (fm *FactManager) Flush() error {
	fm.mu.RLock()
	store := fm.store
	snapshot := fm.snapshotLocked()
	fm.mu.RUnlock()

	if store == nil {
		return nil
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal facts failed: %w", err)
	}
	if err := store.Save(FactSnapshotName, data); err != nil {
		return fmt.Errorf("save facts failed: %w", err)
	}
	return nil
}

func (fm *FactManager) ensureFactsLocked(userID int64) {
	if fm.facts == nil {
		fm.facts = make(map[int64][]*FactMemory)
	}
	if fm.facts[userID] == nil {
		fm.facts[userID] = []*FactMemory{}
	}
}

func (fm *FactManager) findExactFactLocked(userID int64, predicate, object string) *FactMemory {
	for _, fact := range fm.facts[userID] {
		if fact == nil {
			continue
		}
		if fact.Predicate == predicate && fact.Object == object {
			return fact
		}
	}
	return nil
}

func (fm *FactManager) expireFactsLocked(userID int64, now time.Time) {
	for _, fact := range fm.facts[userID] {
		if fact == nil || fact.Status != FactStatusActive || fact.ExpiresAt == nil {
			continue
		}
		if fact.ExpiresAt.Before(now) {
			fact.Status = FactStatusStale
		}
	}
}

func (fm *FactManager) normalizeFactsLocked() {
	if fm.facts == nil {
		fm.facts = make(map[int64][]*FactMemory)
		return
	}

	for userID, facts := range fm.facts {
		if facts == nil {
			fm.facts[userID] = []*FactMemory{}
			continue
		}

		normalized := make([]*FactMemory, 0, len(facts))
		for _, fact := range facts {
			if fact == nil {
				continue
			}
			if fact.UserID == 0 {
				fact.UserID = userID
			}
			fact.Predicate = strings.TrimSpace(fact.Predicate)
			fact.Object = strings.TrimSpace(fact.Object)
			fact.Summary = strings.TrimSpace(fact.Summary)
			if fact.Tags == nil {
				fact.Tags = []string{}
			}
			if fact.Status == "" {
				fact.Status = FactStatusActive
			}
			if fact.ID == "" {
				fact.ID = utils.NewRequestID("fact")
			}
			if fact.CreatedAt.IsZero() {
				fact.CreatedAt = time.Now()
			}
			if fact.LastConfirmedAt.IsZero() {
				fact.LastConfirmedAt = fact.CreatedAt
			}
			normalized = append(normalized, fact)
		}
		fm.facts[userID] = normalized
	}
}

func (fm *FactManager) snapshotLocked() map[int64][]*FactMemory {
	result := make(map[int64][]*FactMemory, len(fm.facts))
	for userID, facts := range fm.facts {
		copied := make([]*FactMemory, 0, len(facts))
		for _, fact := range facts {
			if fact == nil {
				continue
			}
			copied = append(copied, &FactMemory{
				ID:              fact.ID,
				UserID:          fact.UserID,
				SessionID:       fact.SessionID,
				Predicate:       fact.Predicate,
				Object:          fact.Object,
				Summary:         fact.Summary,
				Tags:            append([]string(nil), fact.Tags...),
				Confidence:      fact.Confidence,
				SourceMessage:   fact.SourceMessage,
				Status:          fact.Status,
				CreatedAt:       fact.CreatedAt,
				LastConfirmedAt: fact.LastConfirmedAt,
				ExpiresAt:       cloneTimePointer(fact.ExpiresAt),
			})
		}
		result[userID] = copied
	}
	return result
}

func (fm *FactManager) markDirty() {
	fm.mu.RLock()
	dirty := fm.dirty
	fm.mu.RUnlock()

	if dirty != nil {
		dirty.MarkDirty(FactFlushTaskName)
	}
}

func normalizeFactCandidate(candidate FactMemory) FactMemory {
	candidate.Predicate = strings.TrimSpace(candidate.Predicate)
	candidate.Object = strings.TrimSpace(candidate.Object)
	candidate.Summary = strings.TrimSpace(candidate.Summary)
	candidate.SourceMessage = strings.TrimSpace(candidate.SourceMessage)
	candidate.Tags = mergeUniqueStrings(nil, candidate.Tags)
	return candidate
}

func scoreFact(fact *FactMemory, keywords []string) int {
	score := 1
	for _, keyword := range keywords {
		if keyword == "" {
			continue
		}
		if strings.Contains(fact.Summary, keyword) {
			score += 5
		}
		if strings.Contains(fact.Object, keyword) {
			score += 4
		}
		if strings.Contains(fact.Predicate, keyword) {
			score += 3
		}
		for _, tag := range fact.Tags {
			if strings.Contains(tag, keyword) || strings.Contains(keyword, tag) {
				score += 2
			}
		}
	}
	return score
}

func extractMemoryKeywords(query string) []string {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return nil
	}

	replacer := strings.NewReplacer(
		"，", " ", "。", " ", "！", " ", "？", " ",
		",", " ", ".", " ", "!", " ", "?", " ",
		"：", " ", "；", " ", "、", " ", "\n", " ",
	)
	normalized := replacer.Replace(trimmed)
	parts := strings.Fields(normalized)

	keywords := make([]string, 0, len(parts)+1)
	seen := make(map[string]struct{})

	addKeyword := func(keyword string) {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			return
		}
		if _, ok := seen[keyword]; ok {
			return
		}
		seen[keyword] = struct{}{}
		keywords = append(keywords, keyword)
	}

	addKeyword(trimmed)
	for _, part := range parts {
		if len([]rune(part)) < 2 {
			continue
		}
		addKeyword(part)
	}

	return keywords
}

func cloneTimePointer(source *time.Time) *time.Time {
	if source == nil {
		return nil
	}
	value := *source
	return &value
}
