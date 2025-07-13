package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

var Config struct {
	Hostadd   string
	GroupID   int64
	TargetId  int64
	AiKEY     string
	AiBaseUrl string
	AiPrompt  string
}

func init() {
	err := godotenv.Load(".env")
	// err := godotenv.Load("../../test/.env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	Config.Hostadd = os.Getenv("HOSTADD")
	Config.GroupID, _ = strconv.ParseInt(os.Getenv("GROUPID"), 10, 64)
	Config.TargetId, _ = strconv.ParseInt(os.Getenv("TARGETID"), 10, 64)
	Config.AiKEY = os.Getenv("AI_KEY")
	Config.AiBaseUrl = os.Getenv("AI_BASEURL")
	Config.AiPrompt = os.Getenv("AI_PROMPT")
}
