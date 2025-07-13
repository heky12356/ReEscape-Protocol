package aifunction

import (
	"project-yume/internal/config"

	openai "github.com/sashabaranov/go-openai"
)

var client *openai.Client

func init() {
	aiconfig := openai.DefaultConfig(config.GetConfig().AiKEY)
	aiconfig.BaseURL = config.GetConfig().AiBaseUrl
	client = openai.NewClientWithConfig(aiconfig)
}
