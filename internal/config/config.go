package config

import (
	"log"
	"os"
	"strconv"
	"strings"

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
	
	// 新增配置
	EnableNaturalScheduler bool     // 启用自然定时器
	EnableEmotionalMemory  bool     // 启用情感记忆
	ActiveHours           []int    // 活跃时间段
	SleepHours            []int    // 休息时间段
	BaseInterval          int      // 基础发送间隔(分钟)
	RandomFactor          float64  // 随机因子
}

var config = &Config{}

func init() {
	// err := godotenv.Load(".env")
	err := godotenv.Load("debug.env")
	// err := godotenv.Load("../../test/.env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	
	// 原有配置
	config.Hostadd = os.Getenv("HOSTADD")
	config.GroupID, _ = strconv.ParseInt(os.Getenv("GROUPID"), 10, 64)
	config.TargetId, _ = strconv.ParseInt(os.Getenv("TARGETID"), 10, 64)
	config.AiKEY = os.Getenv("AI_KEY")
	config.AiBaseUrl = os.Getenv("AI_BASEURL")
	config.AiPrompt = os.Getenv("AI_PROMPT")
	config.AiModel = os.Getenv("AI_MODEL")
	
	// 新增配置
	config.EnableNaturalScheduler = getBoolEnv("ENABLE_NATURAL_SCHEDULER", true)
	config.EnableEmotionalMemory = getBoolEnv("ENABLE_EMOTIONAL_MEMORY", true)
	config.ActiveHours = getIntArrayEnv("ACTIVE_HOURS", []int{9, 10, 11, 14, 15, 16, 19, 20, 21})
	config.SleepHours = getIntArrayEnv("SLEEP_HOURS", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 22, 23})
	config.BaseInterval = getIntEnv("BASE_INTERVAL", 45)
	config.RandomFactor = getFloatEnv("RANDOM_FACTOR", 0.5)
}

func GetConfig() *Config {
	return config
}

func getBoolEnv(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	result, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return result
}

func getIntEnv(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	result, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return result
}

func getFloatEnv(key string, defaultValue float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	result, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return defaultValue
	}
	return result
}

func getIntArrayEnv(key string, defaultValue []int) []int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	
	parts := strings.Split(value, ",")
	result := make([]int, 0, len(parts))
	
	for _, part := range parts {
		if num, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
			result = append(result, num)
		}
	}
	
	if len(result) == 0 {
		return defaultValue
	}
	
	return result
}
