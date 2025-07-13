package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Hostadd   string
	GroupID   int64
	TargetId  int64
	AiKEY     string
	AiBaseUrl string
	AiPrompt  string
	AiModel   string
}

var config *Config

func init() {
	err := godotenv.Load(".env")
	// err := godotenv.Load("../../test/.env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	config.Hostadd = os.Getenv("HOSTADD")
	config.GroupID, _ = strconv.ParseInt(os.Getenv("GROUPID"), 10, 64)
	config.TargetId, _ = strconv.ParseInt(os.Getenv("TARGETID"), 10, 64)
	config.AiKEY = os.Getenv("AI_KEY")
	config.AiBaseUrl = os.Getenv("AI_BASEURL")
	config.AiPrompt = os.Getenv("AI_PROMPT")
	config.AiModel = os.Getenv("AI_MODEL")
}

func GetConfig() *Config {
	return config
}
