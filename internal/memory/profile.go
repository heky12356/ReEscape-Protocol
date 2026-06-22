package memory

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"project-yume/internal/storage"
)

type UserProfile struct {
	UserID            int64     `json:"user_id"`
	PreferredTone     string    `json:"preferred_tone"`
	ReplyStyle        string    `json:"reply_style"`
	RelationshipStyle string    `json:"relationship_style"`
	Likes             []string  `json:"likes"`
	Dislikes          []string  `json:"dislikes"`
	Taboos            []string  `json:"taboos"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type ProfilePatch struct {
	PreferredTone     string
	ReplyStyle        string
	RelationshipStyle string
	Likes             []string
	Dislikes          []string
	Taboos            []string
}

type ProfileManager struct {
	mu       sync.RWMutex
	profiles map[int64]*UserProfile
	store    storage.SnapshotStore
	dirty    storage.DirtyMarker
}

var profileManager *ProfileManager

const ProfileSnapshotName = "memory/user_profiles.json"
const ProfileFlushTaskName = "user_profiles"

func init() {
	profileManager = &ProfileManager{
		profiles: make(map[int64]*UserProfile),
	}
}

func GetProfileManager() *ProfileManager {
	return profileManager
}

func (pm *ProfileManager) ApplyPatch(userID int64, patch ProfilePatch) {
	if userID == 0 || patch.isEmpty() {
		return
	}

	pm.mu.Lock()
	profile := pm.ensureProfileLocked(userID)

	if patch.PreferredTone != "" {
		profile.PreferredTone = patch.PreferredTone
	}
	if patch.ReplyStyle != "" {
		profile.ReplyStyle = patch.ReplyStyle
	}
	if patch.RelationshipStyle != "" {
		profile.RelationshipStyle = patch.RelationshipStyle
	}

	profile.Likes = mergeUniqueStrings(profile.Likes, patch.Likes)
	profile.Dislikes = mergeUniqueStrings(profile.Dislikes, patch.Dislikes)
	profile.Taboos = mergeUniqueStrings(profile.Taboos, patch.Taboos)
	profile.UpdatedAt = time.Now()
	pm.mu.Unlock()

	pm.markDirty()
}

func (pm *ProfileManager) GetProfile(userID int64) UserProfile {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	profile := pm.profiles[userID]
	if profile == nil {
		return UserProfile{UserID: userID}
	}

	return UserProfile{
		UserID:            profile.UserID,
		PreferredTone:     profile.PreferredTone,
		ReplyStyle:        profile.ReplyStyle,
		RelationshipStyle: profile.RelationshipStyle,
		Likes:             append([]string(nil), profile.Likes...),
		Dislikes:          append([]string(nil), profile.Dislikes...),
		Taboos:            append([]string(nil), profile.Taboos...),
		UpdatedAt:         profile.UpdatedAt,
	}
}

func (pm *ProfileManager) ConfigurePersistence(store storage.SnapshotStore, dirty storage.DirtyMarker) error {
	pm.mu.Lock()
	pm.store = store
	pm.dirty = dirty
	pm.mu.Unlock()

	if store == nil {
		return nil
	}

	data, err := store.Load(ProfileSnapshotName)
	if err != nil {
		return fmt.Errorf("load profiles failed: %w", err)
	}
	if len(data) == 0 {
		return nil
	}

	loaded := make(map[int64]*UserProfile)
	if err := json.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("unmarshal profiles failed: %w", err)
	}

	pm.mu.Lock()
	pm.profiles = loaded
	pm.normalizeProfilesLocked()
	pm.mu.Unlock()
	return nil
}

func (pm *ProfileManager) Flush() error {
	pm.mu.RLock()
	store := pm.store
	snapshot := pm.snapshotLocked()
	pm.mu.RUnlock()

	if store == nil {
		return nil
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal profiles failed: %w", err)
	}
	if err := store.Save(ProfileSnapshotName, data); err != nil {
		return fmt.Errorf("save profiles failed: %w", err)
	}
	return nil
}

func (pm *ProfileManager) ensureProfileLocked(userID int64) *UserProfile {
	profile := pm.profiles[userID]
	if profile == nil {
		profile = &UserProfile{
			UserID:    userID,
			Likes:     []string{},
			Dislikes:  []string{},
			Taboos:    []string{},
			UpdatedAt: time.Now(),
		}
		pm.profiles[userID] = profile
		return profile
	}

	if profile.Likes == nil {
		profile.Likes = []string{}
	}
	if profile.Dislikes == nil {
		profile.Dislikes = []string{}
	}
	if profile.Taboos == nil {
		profile.Taboos = []string{}
	}
	if profile.UpdatedAt.IsZero() {
		profile.UpdatedAt = time.Now()
	}
	return profile
}

func (pm *ProfileManager) normalizeProfilesLocked() {
	if pm.profiles == nil {
		pm.profiles = make(map[int64]*UserProfile)
		return
	}

	for userID, profile := range pm.profiles {
		if profile == nil {
			delete(pm.profiles, userID)
			continue
		}
		profile.UserID = userID
		if profile.Likes == nil {
			profile.Likes = []string{}
		}
		if profile.Dislikes == nil {
			profile.Dislikes = []string{}
		}
		if profile.Taboos == nil {
			profile.Taboos = []string{}
		}
		if profile.UpdatedAt.IsZero() {
			profile.UpdatedAt = time.Now()
		}
	}
}

func (pm *ProfileManager) snapshotLocked() map[int64]*UserProfile {
	result := make(map[int64]*UserProfile, len(pm.profiles))
	for userID, profile := range pm.profiles {
		if profile == nil {
			continue
		}
		result[userID] = &UserProfile{
			UserID:            profile.UserID,
			PreferredTone:     profile.PreferredTone,
			ReplyStyle:        profile.ReplyStyle,
			RelationshipStyle: profile.RelationshipStyle,
			Likes:             append([]string(nil), profile.Likes...),
			Dislikes:          append([]string(nil), profile.Dislikes...),
			Taboos:            append([]string(nil), profile.Taboos...),
			UpdatedAt:         profile.UpdatedAt,
		}
	}
	return result
}

func (pm *ProfileManager) markDirty() {
	pm.mu.RLock()
	dirty := pm.dirty
	pm.mu.RUnlock()

	if dirty != nil {
		dirty.MarkDirty(ProfileFlushTaskName)
	}
}

func (patch ProfilePatch) isEmpty() bool {
	return patch.PreferredTone == "" &&
		patch.ReplyStyle == "" &&
		patch.RelationshipStyle == "" &&
		len(patch.Likes) == 0 &&
		len(patch.Dislikes) == 0 &&
		len(patch.Taboos) == 0
}

func mergeUniqueStrings(existing, incoming []string) []string {
	if len(incoming) == 0 {
		return append([]string(nil), existing...)
	}

	result := append([]string(nil), existing...)
	seen := make(map[string]struct{}, len(result))
	for _, item := range result {
		if item == "" {
			continue
		}
		seen[item] = struct{}{}
	}

	for _, item := range incoming {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}

	return result
}
