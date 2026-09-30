package state

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"project-yume/internal/domain/intent"
	"project-yume/internal/domain/openloop"
)

func (sm *StateManager) ListOpenLoops(userID int64, sessionID string, includeClosed bool) []openloop.OpenLoop {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	result := make([]openloop.OpenLoop, 0)
	for _, item := range sm.openLoops {
		item = openloop.Normalize(item)
		if userID != 0 && item.UserID != userID || sessionID != "" && item.SessionID != sessionID {
			continue
		}
		if !includeClosed && (item.Status == openloop.StatusResolved || item.Status == openloop.StatusCancelled) {
			continue
		}
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].UpdatedAt.Before(result[j].UpdatedAt) })
	return result
}

func (sm *StateManager) GetOpenLoop(id string) (openloop.OpenLoop, bool) {
	sm.mu.RLock()
	item, ok := sm.openLoops[strings.TrimSpace(id)]
	sm.mu.RUnlock()
	if !ok {
		return openloop.OpenLoop{}, false
	}
	return openloop.Normalize(item), true
}

func (sm *StateManager) UpsertOpenLoop(item openloop.OpenLoop) (openloop.OpenLoop, error) {
	item = openloop.Normalize(item)
	if item.ID == "" || item.UserID == 0 || item.Description == "" {
		return openloop.OpenLoop{}, fmt.Errorf("open loop id, user_id and description are required")
	}
	sm.mu.Lock()
	if previous, ok := sm.openLoops[item.ID]; ok && previous.UserID != 0 && previous.UserID != item.UserID {
		sm.mu.Unlock()
		return openloop.OpenLoop{}, fmt.Errorf("open loop belongs to another user")
	}
	sm.openLoops[item.ID] = item
	sm.mu.Unlock()
	sm.markDirty()
	return item, nil
}

func (sm *StateManager) CloseOpenLoop(id, actor string, now time.Time) (openloop.OpenLoop, error) {
	sm.mu.Lock()
	item, ok := sm.openLoops[strings.TrimSpace(id)]
	if !ok {
		sm.mu.Unlock()
		return openloop.OpenLoop{}, fmt.Errorf("open loop not found")
	}
	updated, err := openloop.Transition(item, openloop.StatusResolved, actor, now)
	if err == nil {
		sm.openLoops[updated.ID] = updated
	}
	sm.mu.Unlock()
	if err == nil {
		sm.markDirty()
	}
	return updated, err
}

func (sm *StateManager) DeferOpenLoop(id string, dueAt time.Time, actor string, now time.Time) (openloop.OpenLoop, error) {
	sm.mu.Lock()
	item, ok := sm.openLoops[strings.TrimSpace(id)]
	if !ok {
		sm.mu.Unlock()
		return openloop.OpenLoop{}, fmt.Errorf("open loop not found")
	}
	updated, err := openloop.Transition(item, openloop.StatusDeferred, actor, now)
	if err == nil {
		updated.DueAt = dueAt
		updated.UpdatedAt = now
		sm.openLoops[updated.ID] = updated
	}
	sm.mu.Unlock()
	if err == nil {
		sm.markDirty()
	}
	return updated, err
}

func (sm *StateManager) ListIntents(userID int64, sessionID string, statuses ...intent.Status) []intent.Intent {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	allowed := make(map[intent.Status]struct{}, len(statuses))
	for _, status := range statuses {
		allowed[status] = struct{}{}
	}
	result := make([]intent.Intent, 0)
	for _, item := range sm.intents {
		item = intent.Normalize(item)
		if userID != 0 && item.UserID != userID || sessionID != "" && item.SessionID != sessionID {
			continue
		}
		if len(allowed) > 0 {
			if _, ok := allowed[item.Status]; !ok {
				continue
			}
		}
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].DueAt.Before(result[j].DueAt) })
	return result
}

func (sm *StateManager) GetIntent(id string) (intent.Intent, bool) {
	sm.mu.RLock()
	item, ok := sm.intents[strings.TrimSpace(id)]
	sm.mu.RUnlock()
	if !ok {
		return intent.Intent{}, false
	}
	return intent.Normalize(item), true
}

func (sm *StateManager) UpsertIntent(item intent.Intent) (intent.Intent, error) {
	item = intent.Normalize(item)
	if item.ID == "" || item.UserID == 0 || item.Summary == "" {
		return intent.Intent{}, fmt.Errorf("intent id, user_id and summary are required")
	}
	sm.mu.Lock()
	if previous, ok := sm.intents[item.ID]; ok && previous.UserID != 0 && previous.UserID != item.UserID {
		sm.mu.Unlock()
		return intent.Intent{}, fmt.Errorf("intent belongs to another user")
	}
	sm.intents[item.ID] = item
	sm.mu.Unlock()
	sm.markDirty()
	return item, nil
}

func (sm *StateManager) ClaimDueIntent(userID int64, sessionID string, now time.Time) (intent.Intent, bool) {
	if now.IsZero() {
		now = time.Now()
	}
	sm.mu.Lock()
	var selected intent.Intent
	found := false
	for id, item := range sm.intents {
		item = intent.Normalize(item)
		if userID != 0 && item.UserID != userID || sessionID != "" && item.SessionID != sessionID || !intent.IsDue(item, now) {
			continue
		}
		if !found || item.DueAt.Before(selected.DueAt) || (item.DueAt.Equal(selected.DueAt) && item.Priority > selected.Priority) {
			selected, found = item, true
		}
		_ = id
	}
	if !found {
		sm.mu.Unlock()
		return intent.Intent{}, false
	}
	selected.Status = intent.StatusClaimed
	selected.UpdatedAt = now
	sm.intents[selected.ID] = selected
	sm.mu.Unlock()
	sm.markDirty()
	return selected, true
}

func (sm *StateManager) TransitionIntent(id string, to intent.Status, now time.Time) (intent.Intent, error) {
	sm.mu.Lock()
	item, ok := sm.intents[strings.TrimSpace(id)]
	if !ok {
		sm.mu.Unlock()
		return intent.Intent{}, fmt.Errorf("intent not found")
	}
	updated, err := intent.Transition(item, to, now)
	if err == nil {
		sm.intents[updated.ID] = updated
	}
	sm.mu.Unlock()
	if err == nil {
		sm.markDirty()
	}
	return updated, err
}

func (sm *StateManager) OpenLoopStore() *StateOpenLoopStore { return &StateOpenLoopStore{manager: sm} }
func (sm *StateManager) IntentStore() *StateIntentStore     { return &StateIntentStore{manager: sm} }

type StateOpenLoopStore struct{ manager *StateManager }

func (s *StateOpenLoopStore) List(userID int64, sessionID string, includeClosed bool) []openloop.OpenLoop {
	return s.manager.ListOpenLoops(userID, sessionID, includeClosed)
}
func (s *StateOpenLoopStore) Get(id string) (openloop.OpenLoop, bool) {
	return s.manager.GetOpenLoop(id)
}
func (s *StateOpenLoopStore) Upsert(item openloop.OpenLoop) (openloop.OpenLoop, error) {
	return s.manager.UpsertOpenLoop(item)
}
func (s *StateOpenLoopStore) Close(id, actor string, now time.Time) (openloop.OpenLoop, error) {
	return s.manager.CloseOpenLoop(id, actor, now)
}
func (s *StateOpenLoopStore) Defer(id string, dueAt time.Time, actor string, now time.Time) (openloop.OpenLoop, error) {
	return s.manager.DeferOpenLoop(id, dueAt, actor, now)
}

type StateIntentStore struct{ manager *StateManager }

func (s *StateIntentStore) List(userID int64, sessionID string, statuses ...intent.Status) []intent.Intent {
	return s.manager.ListIntents(userID, sessionID, statuses...)
}
func (s *StateIntentStore) Get(id string) (intent.Intent, bool) { return s.manager.GetIntent(id) }
func (s *StateIntentStore) Upsert(item intent.Intent) (intent.Intent, error) {
	return s.manager.UpsertIntent(item)
}
func (s *StateIntentStore) ClaimDue(userID int64, sessionID string, now time.Time) (intent.Intent, bool) {
	return s.manager.ClaimDueIntent(userID, sessionID, now)
}
func (s *StateIntentStore) Transition(id string, to intent.Status, now time.Time) (intent.Intent, error) {
	return s.manager.TransitionIntent(id, to, now)
}
