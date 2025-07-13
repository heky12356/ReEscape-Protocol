package aifunction

import (
	"project-yume/internal/config"

	openai "github.com/sashabaranov/go-openai"
)

var client *openai.Client

func init() {
	aiconfig := openai.DefaultConfig(config.Config.AiKEY)
	aiconfig.BaseURL = config.Config.AiBaseUrl
	client = openai.NewClientWithConfig(aiconfig)
}
