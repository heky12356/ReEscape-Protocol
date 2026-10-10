package config

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
)

func cloneConfig(source *Config) *Config {
	if source == nil {
		return &Config{Version: 1, UpdatedAt: time.Now()}
	}
	copy := *source
	copy.ActiveHours = append([]int(nil), source.ActiveHours...)
	copy.SleepHours = append([]int(nil), source.SleepHours...)
	copy.AdminCORSOrigins = append([]string(nil), source.AdminCORSOrigins...)
	copy.SkillDirs = append([]string(nil), source.SkillDirs...)
	copy.LastReloadScopes = append([]ConfigScope(nil), source.LastReloadScopes...)
	return &copy
}

func validateConfig(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if cfg.BaseInterval <= 0 {
		return fmt.Errorf("base interval must be > 0")
	}
	if cfg.RandomFactor < 0 || cfg.RandomFactor > 1 {
		return fmt.Errorf("random factor must be in [0,1]")
	}
	if cfg.ProactiveClaimLeaseMs <= 0 || cfg.ProactiveUserMessageGraceMs < 0 {
		return fmt.Errorf("proactive claim timing is invalid")
	}
	if cfg.MessageAggregateIdleWindowMs <= 0 || cfg.MessageAggregateMaxWindowMs < cfg.MessageAggregateIdleWindowMs || cfg.MessageAggregateMaxMessages <= 0 {
		return fmt.Errorf("message aggregation configuration is invalid")
	}
	if cfg.ContextRecentTurns <= 0 || cfg.ContextSummaryMaxTurns <= 0 || cfg.ContextOpenLoopLimit <= 0 {
		return fmt.Errorf("context limits must be > 0")
	}
	if cfg.ReactMaxSteps <= 0 || cfg.ReactToolTimeoutMs <= 0 || cfg.ReactTotalTimeoutMs <= 0 {
		return fmt.Errorf("react runtime limits must be > 0")
	}
	if cfg.WebSearchMaxResults <= 0 || cfg.WebToolTimeoutMs <= 0 || cfg.WebFetchMaxBytes <= 0 || cfg.WebFetchMaxChars <= 0 {
		return fmt.Errorf("web tool limits must be > 0")
	}
	if cfg.VisionImageDetail != "auto" && cfg.VisionImageDetail != "low" && cfg.VisionImageDetail != "high" {
		return fmt.Errorf("vision image detail must be auto, low or high")
	}
	if _, _, err := ActiveAIProfile(AIProfileSet{Active: cfg.AiProfile, Profiles: map[string]AIProfile{
		cfg.AiProfile: {
			AIBaseURL: cfg.AiBaseUrl, AIModel: cfg.AiModel, AIKey: cfg.AiKEY,
			AITemperature: cfg.AiTemperature, AIMaxTokens: cfg.AiMaxTokens,
			AITimeout: cfg.AiTimeout, AIRetryCount: cfg.AiRetryCount,
			AIRateLimit: cfg.AiRateLimit, AITopP: cfg.AiTopP,
		},
	}}); err != nil {
		return err
	}
	return ValidateAIProfile(AIProfile{
		AIBaseURL: cfg.AiBaseUrl, AIModel: cfg.AiModel, AIKey: cfg.AiKEY,
		AITemperature: cfg.AiTemperature, AIMaxTokens: cfg.AiMaxTokens,
		AITimeout: cfg.AiTimeout, AIRetryCount: cfg.AiRetryCount,
		AIRateLimit: cfg.AiRateLimit, AITopP: cfg.AiTopP,
	})
}

func changedScopes(before, after *Config) []ConfigScope {
	if before == nil || after == nil {
		return []ConfigScope{ScopeBot, ScopeModel, ScopeSkill, ScopeLogging, ScopeRuntime}
	}
	result := make([]ConfigScope, 0, 5)
	if !reflect.DeepEqual([]any{before.Hostadd, before.WsPort, before.HttpPort, before.TargetId, before.Token, before.AdminListenHost, before.AdminAPIKey, before.AdminCORSOrigins, before.OneBotDialTimeoutMs, before.OneBotReconnectInitialMs, before.OneBotReconnectMaxMs, before.OneBotHeartbeatIntervalMs, before.OneBotReadTimeoutMs, before.ShutdownTimeoutMs}, []any{after.Hostadd, after.WsPort, after.HttpPort, after.TargetId, after.Token, after.AdminListenHost, after.AdminAPIKey, after.AdminCORSOrigins, after.OneBotDialTimeoutMs, after.OneBotReconnectInitialMs, after.OneBotReconnectMaxMs, after.OneBotHeartbeatIntervalMs, after.OneBotReadTimeoutMs, after.ShutdownTimeoutMs}) {
		result = append(result, ScopeBot)
	}
	if !reflect.DeepEqual([]any{before.AiKEY, before.AiBaseUrl, before.AiModel, before.AiProfile, before.AiConfigFile, before.AiTemperature, before.AiMaxTokens, before.AiTimeout, before.AiRetryCount, before.AiRateLimit, before.AiTopP, before.EnableWebTools, before.WebSearchProvider, before.WebSearchEndpoint, before.WebSearchAPIKey}, []any{after.AiKEY, after.AiBaseUrl, after.AiModel, after.AiProfile, after.AiConfigFile, after.AiTemperature, after.AiMaxTokens, after.AiTimeout, after.AiRetryCount, after.AiRateLimit, after.AiTopP, after.EnableWebTools, after.WebSearchProvider, after.WebSearchEndpoint, after.WebSearchAPIKey}) {
		result = append(result, ScopeModel)
	}
	if !reflect.DeepEqual([]any{before.EnableSkills, before.SkillDirs, before.SkillAutoHintLimit, before.SkillCandidateMinScore, before.SkillAutoLoadMinScore, before.SkillAutoLoadMinConfidence, before.SkillMaxAutoLoaded, before.SkillForceReadOnCandidate, before.SkillResourceMaxBytes, before.SkillAllowScripts, before.SkillLoadSystem}, []any{after.EnableSkills, after.SkillDirs, after.SkillAutoHintLimit, after.SkillCandidateMinScore, after.SkillAutoLoadMinScore, after.SkillAutoLoadMinConfidence, after.SkillMaxAutoLoaded, after.SkillForceReadOnCandidate, after.SkillResourceMaxBytes, after.SkillAllowScripts, after.SkillLoadSystem}) {
		result = append(result, ScopeSkill)
	}
	if !reflect.DeepEqual([]any{before.LogLevel, before.LogToFile, before.LogFormat, before.LogEnableColor, before.EnableAIRawLog, before.LogDir}, []any{after.LogLevel, after.LogToFile, after.LogFormat, after.LogEnableColor, after.EnableAIRawLog, after.LogDir}) {
		result = append(result, ScopeLogging)
	}
	if !reflect.DeepEqual([]any{before.EnableNaturalScheduler, before.EnableEmotionalMemory, before.ActiveHours, before.SleepHours, before.BaseInterval, before.RandomFactor, before.ProactiveClaimLeaseMs, before.ProactiveSkipOnPendingUser, before.ProactiveUserMessageGraceMs, before.RequestIDHeader, before.EnableHealthEndpoint, before.EnableMetrics, before.MetricsPath, before.DataDir, before.MessageAggregateIdleWindowMs, before.MessageAggregateMaxWindowMs, before.MessageAggregateMaxMessages, before.EnableTimeContext, before.TimeContextTimezone, before.TimeContextFormat, before.EnableVisionInput, before.VisionImageDetail, before.EnableImageOCRFallback, before.EnableImageAssetReply, before.ImageAssetDir, before.ImageAssetIndexFile, before.EnableSpaceSegmentDelimiter, before.ContextRecentTurns, before.ContextSummaryMaxTurns, before.ContextOpenLoopLimit, before.EnableReactAgent, before.ReactMaxSteps, before.ReactToolTimeoutMs, before.ReactAllowWriteTools, before.ReactTraceMode, before.ReactTotalTimeoutMs, before.WebSearchMaxResults, before.WebToolTimeoutMs, before.WebFetchMaxBytes, before.WebFetchMaxChars, before.WebFetchUserAgent, before.TavilySearchDepth, before.TavilyTopic, before.TavilyIncludeAnswer, before.TavilyIncludeRawContent, before.TavilySafeSearch, before.Character, before.AllowCharacterIdentityExplanation, before.AiPrompt, before.BasePrompt, before.UserPrompt, before.CharacterPrompt}, []any{after.EnableNaturalScheduler, after.EnableEmotionalMemory, after.ActiveHours, after.SleepHours, after.BaseInterval, after.RandomFactor, after.ProactiveClaimLeaseMs, after.ProactiveSkipOnPendingUser, after.ProactiveUserMessageGraceMs, after.RequestIDHeader, after.EnableHealthEndpoint, after.EnableMetrics, after.MetricsPath, after.DataDir, after.MessageAggregateIdleWindowMs, after.MessageAggregateMaxWindowMs, after.MessageAggregateMaxMessages, after.EnableTimeContext, after.TimeContextTimezone, after.TimeContextFormat, after.EnableVisionInput, after.VisionImageDetail, after.EnableImageOCRFallback, after.EnableImageAssetReply, after.ImageAssetDir, after.ImageAssetIndexFile, after.EnableSpaceSegmentDelimiter, after.ContextRecentTurns, after.ContextSummaryMaxTurns, after.ContextOpenLoopLimit, after.EnableReactAgent, after.ReactMaxSteps, after.ReactToolTimeoutMs, after.ReactAllowWriteTools, after.ReactTraceMode, after.ReactTotalTimeoutMs, after.WebSearchMaxResults, after.WebToolTimeoutMs, after.WebFetchMaxBytes, after.WebFetchMaxChars, after.WebFetchUserAgent, after.TavilySearchDepth, after.TavilyTopic, after.TavilyIncludeAnswer, after.TavilyIncludeRawContent, after.TavilySafeSearch, after.Character, after.AllowCharacterIdentityExplanation, after.AiPrompt, after.BasePrompt, after.UserPrompt, after.CharacterPrompt}) {
		result = append(result, ScopeRuntime)
	}
	sort.Slice(result, func(i, j int) bool { return strings.Compare(string(result[i]), string(result[j])) < 0 })
	return result
}
