package config

import (
	"os"
	"strconv"
	"strings"

	"project-yume/internal/character"
	"project-yume/internal/utils"

	"github.com/joho/godotenv"
)

type Config struct {
	// 基础配置
	Hostadd      string
	WsPort       string
	HttpPort     string
	TargetId     int64
	AiKEY        string
	AiBaseUrl    string
	AiPrompt     string
	AiModel      string
	AiProfile    string
	AiConfigFile string
	Character    string
	Token        string

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

	// 日志配置
	LogLevel       string // 日志级别
	LogToFile      bool   // 是否记录到文件
	LogFormat      string // 日志格式，text/json
	LogEnableColor bool   // 控制台日志颜色

	// 可观测性配置
	RequestIDHeader      string // 请求 ID Header
	EnableHealthEndpoint bool   // 启用健康检查接口
	EnableMetrics        bool   // 启用指标接口
	MetricsPath          string // 指标接口路径

	// 存储配置
	DataDir string // 数据目录
	LogDir  string // 日志目录

	// 功能开关
	EnableOnlyLongChat           bool // 仅长对话模式
	MessageAggregateIdleWindowMs int  // 消息聚合空闲窗口(毫秒)
	MessageAggregateMaxWindowMs  int  // 消息聚合最大窗口(毫秒)
	MessageAggregateMaxMessages  int  // 单次消息聚合最大条数
}

var config = &Config{}

var (
	cm               *character.CharacterManager
	systemBasePrompt string
)

func init() {
	envFile := os.Getenv("ENV_FILE")
	if envFile == "" {
		envFile = ".env"
	}
	if err := godotenv.Load(envFile); err != nil {
		utils.Warn("env file not loaded (%s), fallback to system env only: %v", envFile, err)
	}

	// basePrompt
	var basePrompt string = `
	【技术指令】
	0. 注意将无意义的乱码去掉
	1. 回复时使用 $ 作为分段标记，每段内容应该简短自然
	2. 每段长度控制在10-20字以内
	3. 分段应该符合语义完整性
	4. 避免在一个完整的句子中间分段
	5. 不要使用表情

	【回复格式示例】
	第一段内容$第二段内容$第三段内容

	【角色】
	`

	// 基础配置
	config.Hostadd = os.Getenv("HOSTADD")
	config.WsPort = os.Getenv("WsPort")
	config.HttpPort = os.Getenv("HttpPort")
	config.TargetId, _ = strconv.ParseInt(os.Getenv("TARGETID"), 10, 64)
	config.AiKEY = os.Getenv("AI_KEY")
	config.AiBaseUrl = os.Getenv("AI_BASEURL")
	config.AiPrompt = basePrompt + os.Getenv("AI_PROMPT")
	systemBasePrompt = basePrompt
	config.AiModel = os.Getenv("AI_MODEL")
	config.AiProfile = getStringEnv("AI_PROFILE", "default")
	config.AiConfigFile = GetAIConfigFilePath()
	config.Character = os.Getenv("CHARACTER")
	config.Token = os.Getenv("Token")

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
	config.AiRetryCount = getIntEnv("AI_RETRY_COUNT", 3)
	config.AiRateLimit = getIntEnv("AI_RATE_LIMIT", 20)
	config.AiTopP = float32(getFloatEnv("AI_TOP_P", 0.9))

	if err := loadActiveAIProfileIntoConfig(); err != nil {
		utils.Warn("load ai profile config failed, fallback env values: %v", err)
	}

	// 日志配置
	config.LogLevel = getStringEnv("LOG_LEVEL", "INFO")
	config.LogToFile = getBoolEnv("LOG_TO_FILE", true)
	config.LogFormat = getStringEnv("LOG_FORMAT", "text")
	config.LogEnableColor = getBoolEnv("LOG_ENABLE_COLOR", true)
	config.RequestIDHeader = getStringEnv("REQUEST_ID_HEADER", "X-Request-ID")
	config.EnableHealthEndpoint = getBoolEnv("ENABLE_HEALTH_ENDPOINT", true)
	config.EnableMetrics = getBoolEnv("ENABLE_METRICS", true)
	config.MetricsPath = getStringEnv("METRICS_PATH", "/metrics")

	// 存储配置
	config.DataDir = getStringEnv("DATA_DIR", "./data")
	config.LogDir = getStringEnv("LOG_DIR", "./logs")

	// 功能开关
	config.EnableOnlyLongChat = getBoolEnv("ENABLE_ONLY_LONG_CHAT", false)
	config.MessageAggregateIdleWindowMs = getIntEnv("MESSAGE_AGGREGATE_IDLE_WINDOW_MS", 2000)
	config.MessageAggregateMaxWindowMs = getIntEnv("MESSAGE_AGGREGATE_MAX_WINDOW_MS", 10000)
	config.MessageAggregateMaxMessages = getIntEnv("MESSAGE_AGGREGATE_MAX_MESSAGES", 5)

	cm, err := character.NewCharacterManager("./config/character", config.Character)
	if err != nil {
		utils.Error("Failed to create character manager: %v", err)
		os.Exit(1)
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
