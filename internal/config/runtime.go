package config

import (
	"fmt"
	"os"
	"strconv"

	"project-yume/internal/character"
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
	config.LogLevel = getStringEnv("LOG_LEVEL", config.LogLevel)
	config.LogToFile = getBoolEnv("LOG_TO_FILE", config.LogToFile)
	config.LogFormat = getStringEnv("LOG_FORMAT", config.LogFormat)
	config.LogEnableColor = getBoolEnv("LOG_ENABLE_COLOR", config.LogEnableColor)
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

	config.Character = getStringEnv("CHARACTER", "default")
	config.Token = os.Getenv("Token")

	characterManager, err := character.NewCharacterManager(getCharacterConfigDir(), config.Character)
	if err != nil {
		return fmt.Errorf("reload character config failed: %w", err)
	}
	cm = characterManager
	config.AiPrompt = systemBasePrompt + os.Getenv("AI_PROMPT") + cm.GetPrompt()

	if err := utils.ConfigureDefaultLogger(
		utils.ParseLogLevel(config.LogLevel),
		config.LogToFile,
		config.LogEnableColor,
		config.LogDir,
		config.LogFormat,
	); err != nil {
		return fmt.Errorf("configure logger failed: %w", err)
	}

	return nil
}
