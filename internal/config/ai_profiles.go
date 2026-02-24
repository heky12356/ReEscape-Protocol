package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const defaultAIConfigFilePath = "./config/ai_profiles.json"

type AIProfile struct {
	AIBaseURL     string  `json:"aiBaseUrl"`
	AIModel       string  `json:"aiModel"`
	AIKey         string  `json:"aiKey"`
	AITemperature float32 `json:"aiTemperature"`
	AIMaxTokens   int     `json:"aiMaxTokens"`
	AITimeout     int     `json:"aiTimeout"`
	AIRetryCount  int     `json:"aiRetryCount"`
	AIRateLimit   int     `json:"aiRateLimit"`
	AITopP        float32 `json:"aiTopP"`
}

type AIProfileSet struct {
	Active   string               `json:"active"`
	Profiles map[string]AIProfile `json:"profiles"`
}

func GetAIConfigFilePath() string {
	raw := strings.TrimSpace(os.Getenv("AI_CONFIG_FILE"))
	if raw == "" {
		return filepath.Clean(defaultAIConfigFilePath)
	}
	return filepath.Clean(raw)
}

func LoadAIProfileSet(path string) (AIProfileSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return AIProfileSet{}, err
	}

	var set AIProfileSet
	if err := json.Unmarshal(data, &set); err != nil {
		return AIProfileSet{}, fmt.Errorf("parse ai config file failed: %w", err)
	}
	normalizeAIProfileSet(&set)
	return set, nil
}

func SaveAIProfileSet(path string, set AIProfileSet) error {
	normalizeAIProfileSet(&set)
	if set.Active == "" {
		return fmt.Errorf("active ai profile is required")
	}
	if _, ok := set.Profiles[set.Active]; !ok {
		return fmt.Errorf("active ai profile not found: %s", set.Active)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(set, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal ai profile set failed: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write ai config file failed: %w", err)
	}
	return nil
}

func EnsureAIProfileSet(path string, fallbackName string, fallbackProfile AIProfile) (AIProfileSet, error) {
	set, err := LoadAIProfileSet(path)
	if err == nil {
		changed := false
		if len(set.Profiles) == 0 {
			name := normalizeAIProfileName(fallbackName)
			if name == "" {
				name = "default"
			}
			if set.Profiles == nil {
				set.Profiles = map[string]AIProfile{}
			}
			set.Profiles[name] = normalizeAIProfile(fallbackProfile)
			set.Active = name
			changed = true
		}
		if set.Active == "" {
			set.Active = firstAIProfileName(set.Profiles)
			changed = true
		}
		if _, ok := set.Profiles[set.Active]; !ok {
			set.Active = firstAIProfileName(set.Profiles)
			changed = true
		}
		if changed {
			if saveErr := SaveAIProfileSet(path, set); saveErr != nil {
				return AIProfileSet{}, saveErr
			}
		}
		return set, nil
	}
	if !os.IsNotExist(err) {
		return AIProfileSet{}, err
	}

	name := normalizeAIProfileName(fallbackName)
	if name == "" {
		name = "default"
	}

	set = AIProfileSet{
		Active: name,
		Profiles: map[string]AIProfile{
			name: normalizeAIProfile(fallbackProfile),
		},
	}
	if saveErr := SaveAIProfileSet(path, set); saveErr != nil {
		return AIProfileSet{}, saveErr
	}
	return set, nil
}

func AIProfileNames(set AIProfileSet) []string {
	names := make([]string, 0, len(set.Profiles))
	for name := range set.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func UpsertAIProfile(set *AIProfileSet, name string, profile AIProfile) (string, error) {
	normalizedName := normalizeAIProfileName(name)
	if normalizedName == "" {
		return "", fmt.Errorf("ai profile name is required")
	}
	if strings.ContainsAny(normalizedName, `/\:*?"<>|`) || strings.Contains(normalizedName, "..") {
		return "", fmt.Errorf("invalid ai profile name")
	}
	if set.Profiles == nil {
		set.Profiles = map[string]AIProfile{}
	}
	set.Profiles[normalizedName] = normalizeAIProfile(profile)
	return normalizedName, nil
}

func SetActiveAIProfileName(set *AIProfileSet, name string) error {
	normalizedName := normalizeAIProfileName(name)
	if normalizedName == "" {
		return fmt.Errorf("active ai profile is required")
	}
	if _, ok := set.Profiles[normalizedName]; !ok {
		return fmt.Errorf("ai profile not found: %s", normalizedName)
	}
	set.Active = normalizedName
	return nil
}

func ActiveAIProfile(set AIProfileSet) (string, AIProfile, error) {
	if set.Active == "" {
		return "", AIProfile{}, fmt.Errorf("active ai profile is empty")
	}
	profile, ok := set.Profiles[set.Active]
	if !ok {
		return "", AIProfile{}, fmt.Errorf("active ai profile not found: %s", set.Active)
	}
	return set.Active, normalizeAIProfile(profile), nil
}

func ValidateAIProfile(profile AIProfile) error {
	profile = normalizeAIProfile(profile)
	if strings.TrimSpace(profile.AIBaseURL) == "" {
		return fmt.Errorf("aiBaseUrl is required")
	}
	if strings.TrimSpace(profile.AIModel) == "" {
		return fmt.Errorf("aiModel is required")
	}
	if profile.AIMaxTokens <= 0 {
		return fmt.Errorf("aiMaxTokens must be > 0")
	}
	if profile.AITimeout <= 0 {
		return fmt.Errorf("aiTimeout must be > 0")
	}
	if profile.AIRetryCount < 0 {
		return fmt.Errorf("aiRetryCount must be >= 0")
	}
	if profile.AIRateLimit <= 0 {
		return fmt.Errorf("aiRateLimit must be > 0")
	}
	if profile.AITemperature < 0 || profile.AITemperature > 2 {
		return fmt.Errorf("aiTemperature must be in [0,2]")
	}
	if profile.AITopP < 0 || profile.AITopP > 1 {
		return fmt.Errorf("aiTopP must be in [0,1]")
	}
	return nil
}

func loadActiveAIProfileIntoConfig() error {
	fallbackName := normalizeAIProfileName(firstNonBlank(os.Getenv("AI_PROFILE"), config.AiProfile, "default"))
	fallbackProfile := normalizeAIProfile(AIProfile{
		AIBaseURL:     firstNonBlank(os.Getenv("AI_BASEURL"), config.AiBaseUrl),
		AIModel:       firstNonBlank(os.Getenv("AI_MODEL"), config.AiModel),
		AIKey:         firstNonBlank(os.Getenv("AI_KEY"), config.AiKEY),
		AITemperature: float32(getFloatEnv("AI_TEMPERATURE", float64(config.AiTemperature))),
		AIMaxTokens:   getIntEnv("AI_MAX_TOKENS", config.AiMaxTokens),
		AITimeout:     getIntEnv("AI_TIMEOUT", config.AiTimeout),
		AIRetryCount:  getIntEnv("AI_RETRY_COUNT", config.AiRetryCount),
		AIRateLimit:   getIntEnv("AI_RATE_LIMIT", config.AiRateLimit),
		AITopP:        float32(getFloatEnv("AI_TOP_P", float64(config.AiTopP))),
	})

	path := GetAIConfigFilePath()
	set, err := EnsureAIProfileSet(path, fallbackName, fallbackProfile)
	if err != nil {
		return err
	}

	desiredActive := normalizeAIProfileName(firstNonBlank(os.Getenv("AI_PROFILE"), set.Active, fallbackName))
	if desiredActive != "" {
		if _, exists := set.Profiles[desiredActive]; !exists {
			set.Profiles[desiredActive] = fallbackProfile
		}
		set.Active = desiredActive
		if err := SaveAIProfileSet(path, set); err != nil {
			return err
		}
	}

	activeName, profile, err := ActiveAIProfile(set)
	if err != nil {
		return err
	}

	config.AiProfile = activeName
	config.AiConfigFile = path
	config.AiBaseUrl = profile.AIBaseURL
	config.AiModel = profile.AIModel
	config.AiKEY = profile.AIKey
	config.AiTemperature = profile.AITemperature
	config.AiMaxTokens = profile.AIMaxTokens
	config.AiTimeout = profile.AITimeout
	config.AiRetryCount = profile.AIRetryCount
	config.AiRateLimit = profile.AIRateLimit
	config.AiTopP = profile.AITopP
	return nil
}

func normalizeAIProfileSet(set *AIProfileSet) {
	if set.Profiles == nil {
		set.Profiles = map[string]AIProfile{}
	}

	normalizedProfiles := make(map[string]AIProfile, len(set.Profiles))
	for rawName, profile := range set.Profiles {
		name := normalizeAIProfileName(rawName)
		if name == "" {
			continue
		}
		normalizedProfiles[name] = normalizeAIProfile(profile)
	}
	set.Profiles = normalizedProfiles
	set.Active = normalizeAIProfileName(set.Active)
}

func normalizeAIProfileName(raw string) string {
	return strings.TrimSpace(raw)
}

func normalizeAIProfile(profile AIProfile) AIProfile {
	profile.AIBaseURL = strings.TrimSpace(profile.AIBaseURL)
	profile.AIModel = strings.TrimSpace(profile.AIModel)
	profile.AIKey = strings.TrimSpace(profile.AIKey)

	if profile.AITemperature < 0 {
		profile.AITemperature = 1
	}
	if profile.AIMaxTokens <= 0 {
		profile.AIMaxTokens = 2000
	}
	if profile.AITimeout <= 0 {
		profile.AITimeout = 30
	}
	if profile.AIRetryCount < 0 {
		profile.AIRetryCount = 0
	}
	if profile.AIRateLimit <= 0 {
		profile.AIRateLimit = 20
	}
	if profile.AITopP < 0 {
		profile.AITopP = 0.9
	}
	return profile
}

func firstAIProfileName(profiles map[string]AIProfile) string {
	names := make([]string, 0, len(profiles))
	for name := range profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
