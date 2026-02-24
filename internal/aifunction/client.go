package aifunction

import (
	"sync"

	"project-yume/internal/config"

	openai "github.com/sashabaranov/go-openai"
)

var (
	client   *openai.Client
	clientMu sync.RWMutex
)

func init() {
	ReloadClient()
}

func ReloadClient() {
	cfg := config.GetConfig()
	openAIConfig := openai.DefaultConfig(cfg.AiKEY)
	openAIConfig.BaseURL = cfg.AiBaseUrl

	clientMu.Lock()
	client = openai.NewClientWithConfig(openAIConfig)
	clientMu.Unlock()
}

func getClient() *openai.Client {
	clientMu.RLock()
	defer clientMu.RUnlock()
	return client
}
