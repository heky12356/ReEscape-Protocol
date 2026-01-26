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

// 打开文件
func openLogFile(filepath string) (*os.File, error) {
	return os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
}

// 记录消息日志
func logInteraction(file *os.File, role, message string) {
	if role == "User" {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		_, _ = fmt.Fprintf(file, "\n[%s] User:\n%s\n\n", timestamp, message)
	} else {
		_, _ = fmt.Fprintf(file, "\n[%s] AI:\n%s\n\n", time.Now().Format("2006-01-02 15:04:05"), message)
	}
}

func Queryai(prompt string, msg string) (string, error) {
	filepath := "./public/aichatlog/log_" + time.Now().Format("06-01-02") + ".txt"
	file, err := openLogFile(filepath)
	if err != nil {
		log.Printf("Error opening file: %v\n", err)
		return "", err
	}
	defer file.Close()

	// 记录用户消息
	logInteraction(file, "User", msg)

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
	logInteraction(file, "AI", content)
	return content, nil
}

func QueryaiWithChain(Conversation []openai.ChatCompletionMessage, filepath string) (NewConversation []openai.ChatCompletionMessage, result []string, err error) {
	file, err := openLogFile(filepath)
	if err != nil {
		log.Printf("Error opening file: %v\n", err)
		return nil, nil, err
	}
	defer file.Close()

	// 记录用户消息
	logInteraction(file, "User", Conversation[len(Conversation)-1].Content)

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
		logInteraction(file, "AI", content)
		Conversation = append(Conversation, chs.Message)
	}
	return Conversation, result, nil
}
