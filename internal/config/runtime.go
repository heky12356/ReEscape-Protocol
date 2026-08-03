package config

import (
	"fmt"
	"os"
	"strconv"

	"project-yume/internal/character"
	"project-yume/internal/skill"
	"project-yume/internal/utils"
)

func GetEnvFilePath() string {
	envFile := os.Getenv("ENV_FILE")
	if envFile == "" {
		return ".env"
	}
	return envFile
}

func ReloadRuntimeConfig() error {
	config.Hostadd = getStringEnv("HOSTADD", config.Hostadd)
	config.WsPort = getStringEnv("WsPort", config.WsPort)
	config.HttpPort = getStringEnv("HttpPort", "8088")
	config.AiProfile = getStringEnv("AI_PROFILE", config.AiProfile)
	config.AiConfigFile = GetAIConfigFilePath()

	if v := os.Getenv("TARGETID"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			config.TargetId = parsed
		}
	}

	if err := loadActiveAIProfileIntoConfig(); err != nil {
		return fmt.Errorf("load ai profile config failed: %w", err)
	}

	config.EnableNaturalScheduler = getBoolEnv("ENABLE_NATURAL_SCHEDULER", config.EnableNaturalScheduler)
	config.EnableEmotionalMemory = getBoolEnv("ENABLE_EMOTIONAL_MEMORY", config.EnableEmotionalMemory)
	config.ActiveHours = getIntArrayEnv("ACTIVE_HOURS", config.ActiveHours)
	config.SleepHours = getIntArrayEnv("SLEEP_HOURS", config.SleepHours)
	config.BaseInterval = getIntEnv("BASE_INTERVAL", config.BaseInterval)
	config.RandomFactor = getFloatEnv("RANDOM_FACTOR", config.RandomFactor)
	config.ProactiveClaimLeaseMs = getIntEnv("PROACTIVE_CLAIM_LEASE_MS", config.ProactiveClaimLeaseMs)
	config.ProactiveSkipOnPendingUser = getBoolEnv("PROACTIVE_SKIP_ON_PENDING_USER", config.ProactiveSkipOnPendingUser)
	config.ProactiveUserMessageGraceMs = getIntEnv("PROACTIVE_USER_MESSAGE_GRACE_MS", config.ProactiveUserMessageGraceMs)
	config.LogLevel = getStringEnv("LOG_LEVEL", config.LogLevel)
	config.LogToFile = getBoolEnv("LOG_TO_FILE", config.LogToFile)
	config.LogFormat = getStringEnv("LOG_FORMAT", config.LogFormat)
	config.LogEnableColor = getBoolEnv("LOG_ENABLE_COLOR", config.LogEnableColor)
	config.EnableAIRawLog = getBoolEnv("ENABLE_AI_RAW_LOG", config.EnableAIRawLog)
	config.RequestIDHeader = getStringEnv("REQUEST_ID_HEADER", config.RequestIDHeader)
	config.EnableHealthEndpoint = getBoolEnv("ENABLE_HEALTH_ENDPOINT", config.EnableHealthEndpoint)
	config.EnableMetrics = getBoolEnv("ENABLE_METRICS", config.EnableMetrics)
	config.MetricsPath = getStringEnv("METRICS_PATH", config.MetricsPath)
	config.DataDir = getStringEnv("DATA_DIR", config.DataDir)
	config.LogDir = getStringEnv("LOG_DIR", config.LogDir)
	config.EnableOnlyLongChat = getBoolEnv("ENABLE_ONLY_LONG_CHAT", config.EnableOnlyLongChat)
	config.MessageAggregateIdleWindowMs = getIntEnv("MESSAGE_AGGREGATE_IDLE_WINDOW_MS", config.MessageAggregateIdleWindowMs)
	config.MessageAggregateMaxWindowMs = getIntEnv("MESSAGE_AGGREGATE_MAX_WINDOW_MS", config.MessageAggregateMaxWindowMs)
	config.MessageAggregateMaxMessages = getIntEnv("MESSAGE_AGGREGATE_MAX_MESSAGES", config.MessageAggregateMaxMessages)
	config.EnableTimeContext = getBoolEnv("ENABLE_TIME_CONTEXT", config.EnableTimeContext)
	config.TimeContextTimezone = getStringEnv("TIME_CONTEXT_TIMEZONE", config.TimeContextTimezone)
	config.TimeContextFormat = getStringEnv("TIME_CONTEXT_FORMAT", config.TimeContextFormat)
	config.EnableVisionInput = getBoolEnv("ENABLE_VISION_INPUT", config.EnableVisionInput)
	config.VisionImageDetail = getStringEnv("VISION_IMAGE_DETAIL", config.VisionImageDetail)
	config.EnableImageOCRFallback = getBoolEnv("ENABLE_IMAGE_OCR_FALLBACK", config.EnableImageOCRFallback)
	config.EnableImageAssetReply = getBoolEnv("ENABLE_IMAGE_ASSET_REPLY", config.EnableImageAssetReply)
	config.ImageAssetDir = getStringEnv("IMAGE_ASSET_DIR", config.ImageAssetDir)
	config.ImageAssetIndexFile = getStringEnv("IMAGE_ASSET_INDEX_FILE", config.ImageAssetIndexFile)
	config.EnableSpaceSegmentDelimiter = getBoolEnv("ENABLE_SPACE_SEGMENT_DELIMITER", config.EnableSpaceSegmentDelimiter)
	config.LightAckMode = normalizeLightAckMode(getStringEnv("LIGHT_ACK_MODE", config.LightAckMode))
	config.ShortReplyStrictness = normalizeShortReplyStrictness(getStringEnv("SHORT_REPLY_STRICTNESS", config.ShortReplyStrictness))
	config.ContextRecentTurns = getIntEnv("CONTEXT_RECENT_TURNS", config.ContextRecentTurns)
	config.ContextSummaryMaxTurns = getIntEnv("CONTEXT_SUMMARY_MAX_TURNS", config.ContextSummaryMaxTurns)
	config.ContextOpenLoopLimit = getIntEnv("CONTEXT_OPEN_LOOP_LIMIT", config.ContextOpenLoopLimit)
	config.EnableSkills = getBoolEnv("ENABLE_SKILLS", config.EnableSkills)
	config.SkillDirs = getStringArrayEnv("SKILL_DIRS", config.SkillDirs)
	config.SkillAutoHintLimit = getIntEnv("SKILL_AUTO_HINT_LIMIT", config.SkillAutoHintLimit)
	config.SkillResourceMaxBytes = getIntEnv("SKILL_RESOURCE_MAX_BYTES", config.SkillResourceMaxBytes)
	config.SkillAllowScripts = getBoolEnv("SKILL_ALLOW_SCRIPTS", config.SkillAllowScripts)
	config.SkillLoadSystem = getBoolEnv("SKILL_LOAD_SYSTEM", config.SkillLoadSystem)
	if config.EnableSkills {
		for _, err := range skill.GetManager().LoadDirsWithOptions(config.SkillDirs, skill.LoadOptions{
			Scope:         skill.ScopeProject,
			IncludeHidden: config.SkillLoadSystem,
		}) {
			utils.Warn("reload skill failed: %v", err)
		}
	} else {
		skill.GetManager().LoadDirs(nil)
	}
	config.EnableReactAgent = getBoolEnv("ENABLE_REACT_AGENT", config.EnableReactAgent)
	config.ReactMaxSteps = getIntEnv("REACT_MAX_STEPS", config.ReactMaxSteps)
	config.ReactToolTimeoutMs = getIntEnv("REACT_TOOL_TIMEOUT_MS", config.ReactToolTimeoutMs)
	config.ReactAllowWriteTools = getBoolEnv("REACT_ALLOW_WRITE_TOOLS", config.ReactAllowWriteTools)
	config.ReactTraceMode = normalizeReactTraceMode(getStringEnv("REACT_TRACE_MODE", config.ReactTraceMode))
	config.ReactTotalTimeoutMs = getIntEnv("REACT_TOTAL_TIMEOUT_MS", config.ReactTotalTimeoutMs)
	config.EnableWebTools = getBoolEnv("ENABLE_WEB_TOOLS", config.EnableWebTools)
	config.WebSearchProvider = getStringEnv("WEB_SEARCH_PROVIDER", config.WebSearchProvider)
	config.WebSearchEndpoint = getStringEnv("WEB_SEARCH_ENDPOINT", config.WebSearchEndpoint)
	config.WebSearchAPIKey = os.Getenv("WEB_SEARCH_API_KEY")
	config.WebSearchMaxResults = getIntEnv("WEB_SEARCH_MAX_RESULTS", config.WebSearchMaxResults)
	config.WebToolTimeoutMs = getIntEnv("WEB_TOOL_TIMEOUT_MS", config.WebToolTimeoutMs)
	config.WebFetchMaxBytes = getIntEnv("WEB_FETCH_MAX_BYTES", config.WebFetchMaxBytes)
	config.WebFetchMaxChars = getIntEnv("WEB_FETCH_MAX_CHARS", config.WebFetchMaxChars)
	config.WebFetchUserAgent = getStringEnv("WEB_FETCH_USER_AGENT", config.WebFetchUserAgent)
	config.CharacterIdentityMode = normalizeCharacterIdentityMode(getStringEnv("CHARACTER_IDENTITY_MODE", config.CharacterIdentityMode))
	config.AllowCharacterIdentityExplanation = getBoolEnv("ALLOW_CHARACTER_IDENTITY_EXPLANATION", config.AllowCharacterIdentityExplanation)

	config.Character = getStringEnv("CHARACTER", "default")
	config.Token = os.Getenv("Token")

	characterManager, err := character.NewCharacterManager(getCharacterConfigDir(), config.Character)
	if err != nil {
		return fmt.Errorf("reload character config failed: %w", err)
	}
	cm = characterManager
	systemBasePrompt = buildBasePrompt(config.EnableSpaceSegmentDelimiter)
	applyPromptSections(systemBasePrompt, os.Getenv("AI_PROMPT"))

	if err := utils.ConfigureDefaultLogger(
		utils.ParseLogLevel(config.LogLevel),
		config.LogToFile,
		config.LogEnableColor,
		config.LogDir,
		config.LogFormat,
	); err != nil {
		return fmt.Errorf("configure logger failed: %w", err)
	}
	if err := utils.ConfigureAIRawLogger(config.EnableAIRawLog, config.LogDir); err != nil {
		return fmt.Errorf("configure ai raw logger failed: %w", err)
	}

	return nil
}
