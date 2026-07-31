package affection

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"project-yume/internal/storage"
)

const (
	SnapshotName  = "affection/affection_state.json"
	FlushTaskName = "affection"
)

type Storage struct {
	States map[int64]*State `json:"states"`
}

type Manager struct {
	mu     sync.RWMutex
	states map[int64]*State
	policy Policy
	store  storage.SnapshotStore
	dirty  storage.DirtyMarker
}

var manager = &Manager{
	states: make(map[int64]*State),
	policy: DefaultPolicy(),
}

func GetManager() *Manager {
	return manager
}

func (m *Manager) ConfigurePersistence(store storage.SnapshotStore, dirty storage.DirtyMarker) error {
	m.mu.Lock()
	m.store = store
	m.dirty = dirty
	m.mu.Unlock()

	if store == nil {
		return nil
	}

	data, err := store.Load(SnapshotName)
	if err != nil {
		return fmt.Errorf("load affection state failed: %w", err)
	}
	if len(data) == 0 {
		return nil
	}

	var envelope Storage
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("parse affection state failed: %w", err)
	}

	m.mu.Lock()
	m.states = envelope.States
	m.normalizeLocked()
	m.mu.Unlock()
	return nil
}

func (m *Manager) Flush() error {
	m.mu.RLock()
	store := m.store
	snapshot := m.snapshotLocked()
	m.mu.RUnlock()

	if store == nil {
		return nil
	}

	data, err := json.MarshalIndent(Storage{States: snapshot}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal affection state failed: %w", err)
	}
	if err := store.Save(SnapshotName, data); err != nil {
		return fmt.Errorf("save affection state failed: %w", err)
	}
	return nil
}

func (m *Manager) Get(userID int64) State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state := m.states[userID]
	if state == nil {
		return State{
			UserID:     userID,
			Stage:      "neutral",
			DailyDelta: map[string]int{},
		}
	}
	return cloneState(*state)
}

func (m *Manager) Update(userID int64, req UpdateRequest) (UpdateResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	current := m.ensureLocked(userID)
	result, err := m.policy.Apply(*current, req, time.Now())
	if err != nil {
		return UpdateResult{}, err
	}
	m.states[userID] = &result.State
	m.markDirtyLocked()
	return result, nil
}

func (m *Manager) ensureLocked(userID int64) *State {
	state := m.states[userID]
	if state == nil {
		state = &State{
			UserID:     userID,
			Stage:      "neutral",
			DailyDelta: map[string]int{},
		}
		m.states[userID] = state
	}
	if state.DailyDelta == nil {
		state.DailyDelta = map[string]int{}
	}
	return state
}

func (m *Manager) markDirtyLocked() {
	if m.dirty != nil {
		m.dirty.MarkDirty(FlushTaskName)
	}
}

func (m *Manager) snapshotLocked() map[int64]*State {
	result := make(map[int64]*State, len(m.states))
	for userID, state := range m.states {
		if state == nil {
			continue
		}
		clone := cloneState(*state)
		result[userID] = &clone
	}
	return result
}

func (m *Manager) normalizeLocked() {
	if m.states == nil {
		m.states = make(map[int64]*State)
		return
	}
	for userID, state := range m.states {
		if state == nil {
			delete(m.states, userID)
			continue
		}
		if state.DailyDelta == nil {
			state.DailyDelta = map[string]int{}
		}
		if state.Stage == "" {
			state.Stage = deriveStage(state.Score)
		}
	}
}

func cloneState(state State) State {
	clone := state
	if state.DailyDelta != nil {
		clone.DailyDelta = make(map[string]int, len(state.DailyDelta))
		for k, v := range state.DailyDelta {
			clone.DailyDelta[k] = v
		}
	}
	if state.Changes != nil {
		clone.Changes = append([]ChangeRecord(nil), state.Changes...)
	}
	return clone
}
