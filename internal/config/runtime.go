package config

import (
	"fmt"
	"os"
	"strconv"

	"project-yume/internal/character"
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

	if v := os.Getenv("GROUPID"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			config.GroupID = parsed
		}
	}
	if v := os.Getenv("TARGETID"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			config.TargetId = parsed
		}
	}

	if err := loadActiveAIProfileIntoConfig(); err != nil {
		return fmt.Errorf("load ai profile config failed: %w", err)
	}

	config.Character = getStringEnv("CHARACTER", "default")
	config.Token = os.Getenv("Token")

	characterManager, err := character.NewCharacterManager("./config/character", config.Character)
	if err != nil {
		return fmt.Errorf("reload character config failed: %w", err)
	}
	cm = characterManager
	config.AiPrompt = systemBasePrompt + os.Getenv("AI_PROMPT") + cm.GetPrompt()

	return nil
}
