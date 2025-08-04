package aifunction

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"project-yume/internal/config"

	openai "github.com/sashabaranov/go-openai"
)

func Queryai(prompt string, msg string) (string, error) {
	filepath := "./public/aichatlog/log_" + time.Now().Format("06-01-02") + ".txt"
	file, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("Error opening file: %v\n", err)
		return "", err
	}
	defer file.Close()

	// 记录对话时间
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	file.WriteString(fmt.Sprintf("\n[%s] User:\n%s\n\n", timestamp, msg))

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
	_, err = file.WriteString(content)
	if err != nil {
		return "", fmt.Errorf("error in Queryai : WriteString error: %v", err)
	}
	return content, nil
}

func QueryaiWithChain(Conversation []openai.ChatCompletionMessage, filepath string) (NewConversation []openai.ChatCompletionMessage, result []string, err error) {
	// filepath := "../../public/aichatlog/log_" + time.Now().Format("06-01-02") + ".txt"
	file, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("Error opening file: %v\n", err)
		return nil, nil, err
	}
	defer file.Close()

	// 记录对话时间
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	file.WriteString(fmt.Sprintf("\n[%s] User:\n%s\n\n", timestamp, Conversation[len(Conversation)-1].Content))

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
		_, err = file.WriteString(content)
		if err != nil {
			return nil, nil, fmt.Errorf("error in QueryaiWithChain : WriteString error: %v", err)
		}
		Conversation = append(Conversation, chs.Message)
	}
	// fmt.Print(content)
	return Conversation, result, nil
}
