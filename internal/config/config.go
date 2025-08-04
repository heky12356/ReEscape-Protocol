package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"project-yume/internal/character"

	"github.com/joho/godotenv"
)

type Config struct {
	// 基础配置
	Hostadd   string
	GroupID   int64
	TargetId  int64
	AiKEY     string
	AiBaseUrl string
	AiPrompt  string
	AiModel   string
	Character string

	// 调度器配置
	EnableNaturalScheduler bool    // 启用自然定时器
	EnableEmotionalMemory  bool    // 启用情感记忆
	ActiveHours            []int   // 活跃时间段
	SleepHours             []int   // 休息时间段
	BaseInterval           int     // 基础发送间隔(分钟)
	RandomFactor           float64 // 随机因子

	// AI配置增强
	AiTemperature float32 // AI温度参数
	AiMaxTokens   int     // 最大token数
	AiTimeout     int     // AI请求超时(秒)
	AiRetryCount  int     // AI请求重试次数
	AiRateLimit   int     // AI请求速率限制(每分钟)
	AiTopP        float32 // AI top_p参数

	// 状态管理配置
	StateIdleTimeout        int // 空闲超时(分钟)
	StateComfortThreshold   int // 安慰阈值
	StateEncourageThreshold int // 鼓励阈值
	StateLongChatMinLength  int // 长对话最小长度
	StateMaxConversationLen int // 最大对话长度

	// 日志配置
	LogLevel    string // 日志级别
	LogToFile   bool   // 是否记录到文件
	LogMaxFiles int    // 最大日志文件数
	LogMaxSize  int    // 最大日志文件大小(MB)

	// 存储配置
	DataDir   string // 数据目录
	LogDir    string // 日志目录
	BackupDir string // 备份目录

	// 功能开关
	EnableDebugMode    bool // 调试模式
	EnableHealthCheck  bool // 健康检查
	EnableMetrics      bool // 指标收集
	EnableAutoBackup   bool // 自动备份
	EnableOnlyLongChat bool // 仅长对话模式
}

var config = &Config{}

var cm *character.CharacterManager

func init() {
	err := godotenv.Load(".env")
	// err := godotenv.Load("debug.env")
	// err := godotenv.Load("../../test/.env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// basePrompt
	var basePrompt string = `
	【技术指令】
	0. 注意将无意义的乱码去掉
	1. 回复时使用 $ 作为分段标记，每段内容应该简短自然
	2. 每段长度控制在10-20字以内
	3. 分段应该符合语义完整性
	4. 避免在一个完整的句子中间分段

	【回复格式示例】
	第一段内容$第二段内容$第三段内容

	【角色】
	`

	// 基础配置
	config.Hostadd = os.Getenv("HOSTADD")
	config.GroupID, _ = strconv.ParseInt(os.Getenv("GROUPID"), 10, 64)
	config.TargetId, _ = strconv.ParseInt(os.Getenv("TARGETID"), 10, 64)
	config.AiKEY = os.Getenv("AI_KEY")
	config.AiBaseUrl = os.Getenv("AI_BASEURL")
	config.AiPrompt = basePrompt + os.Getenv("AI_PROMPT")
	config.AiModel = os.Getenv("AI_MODEL")
	config.Character = os.Getenv("CHARACTER")

	// 调度器配置
	config.EnableNaturalScheduler = getBoolEnv("ENABLE_NATURAL_SCHEDULER", true)
	config.EnableEmotionalMemory = getBoolEnv("ENABLE_EMOTIONAL_MEMORY", true)
	config.ActiveHours = getIntArrayEnv("ACTIVE_HOURS", []int{9, 10, 11, 14, 15, 16, 19, 20, 21})
	config.SleepHours = getIntArrayEnv("SLEEP_HOURS", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 22, 23})
	config.BaseInterval = getIntEnv("BASE_INTERVAL", 45)
	config.RandomFactor = getFloatEnv("RANDOM_FACTOR", 0.5)

	// AI配置增强
	config.AiTemperature = float32(getFloatEnv("AI_TEMPERATURE", 1.0))
	config.AiMaxTokens = getIntEnv("AI_MAX_TOKENS", 2000)
	config.AiTimeout = getIntEnv("AI_TIMEOUT", 30)
	config.AiTopP = float32(getFloatEnv("AI_TOP_P", 0.9))

	// 状态管理配置
	config.StateIdleTimeout = getIntEnv("STATE_IDLE_TIMEOUT", 30)
	config.StateComfortThreshold = getIntEnv("STATE_COMFORT_THRESHOLD", 5)
	config.StateEncourageThreshold = getIntEnv("STATE_ENCOURAGE_THRESHOLD", 3)
	config.StateLongChatMinLength = getIntEnv("STATE_LONG_CHAT_MIN_LENGTH", 10)
	config.StateMaxConversationLen = getIntEnv("STATE_MAX_CONVERSATION_LEN", 50)

	// 日志配置
	config.LogLevel = getStringEnv("LOG_LEVEL", "INFO")
	config.LogToFile = getBoolEnv("LOG_TO_FILE", true)
	config.LogMaxFiles = getIntEnv("LOG_MAX_FILES", 7)
	config.LogMaxSize = getIntEnv("LOG_MAX_SIZE", 10)

	// 存储配置
	config.DataDir = getStringEnv("DATA_DIR", "./data")
	config.LogDir = getStringEnv("LOG_DIR", "./logs")
	config.BackupDir = getStringEnv("BACKUP_DIR", "./backup")

	// 功能开关
	config.EnableDebugMode = getBoolEnv("ENABLE_DEBUG_MODE", false)
	config.EnableHealthCheck = getBoolEnv("ENABLE_HEALTH_CHECK", true)
	config.EnableMetrics = getBoolEnv("ENABLE_METRICS", false)
	config.EnableAutoBackup = getBoolEnv("ENABLE_AUTO_BACKUP", true)
	config.EnableOnlyLongChat = getBoolEnv("ENABLE_ONLY_LONG_CHAT", false)

	cm, err := character.NewCharacterManager("./config/character", config.Character)
	if err != nil {
		log.Fatalf("Failed to create character manager: %v", err)
	}
	config.AiPrompt += cm.GetPrompt()
}

func GetConfig() *Config {
	return config
}

func getStringEnv(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
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
