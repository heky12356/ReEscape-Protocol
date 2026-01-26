package aifunction

import (
	"context"
	"fmt"

	"project-yume/internal/config"

	openai "github.com/sashabaranov/go-openai"
)


func Queryai(prompt string, msg string) (string, error) {
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: config.GetConfig().AiModel,
			Messages: []openai.ChatCompletionMessage{
				{Role: "system", Content: prompt},
				{Role: "user", Content: msg},
			},
			Stream:      false,
			MaxTokens:   config.GetConfig().AiMaxTokens,
			Temperature: config.GetConfig().AiTemperature,
			TopP:        config.GetConfig().AiTopP,
		},
	)
	if err != nil {
		return "", fmt.Errorf("error in Queryai : ChatCompletion error: %v", err)
	}

	content := resp.Choices[0].Message.Content
	return content, nil
}

func QueryaiWithChain(Conversation []openai.ChatCompletionMessage) (NewConversation []openai.ChatCompletionMessage, result []string, err error) {


	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:       config.GetConfig().AiModel,
			Messages:    Conversation,
			Stream:      false,
			MaxTokens:   config.GetConfig().AiMaxTokens,
			Temperature: config.GetConfig().AiTemperature,
			TopP:        config.GetConfig().AiTopP,
			N:           1,
		},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("error in QueryaiWithChain : ChatCompletion error: %v", err)
	}

	for _, chs := range resp.Choices {
		content := chs.Message.Content
		result = append(result, content)
		Conversation = append(Conversation, chs.Message)
	}
	return Conversation, result, nil
}
