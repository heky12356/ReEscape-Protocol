package aifunction

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"project-yume/internal/config"
)

func Queryai(prompt string, msg string) (string, error) {
	filepath := "../../public/aichatlog/log_" + time.Now().Format("06-01-02") + ".txt"
	file, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("Error opening file: %v\n", err)
		return "", err
	}
	defer file.Close()

	url := config.Config.AiBaseUrl
	// 记录对话时间
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	file.WriteString(fmt.Sprintf("\n[%s] User:\n%s\n\n", timestamp, msg))

	chatreq := ChatRequest{
		Model: "deepseek-chat",
		Messages: []Message{
			{Role: "system", Content: prompt},
			{Role: "user", Content: msg},
		},
		Stream:      false,
		MaxTokens:   2048,
		Temperature: 0.7,
		TopP:        1,
	}

	// 构建请求体
	payload, err := json.Marshal(chatreq)
	if err != nil {
		return "", err
	}
	// 发送请求
	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+config.Config.AiApi) // 替换为你的 API Key

	// 获取响应
	res, _ := http.DefaultClient.Do(req)
	log.Print("请求ai状态码：", res.StatusCode)
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("请求失败：HTTP %d\n响应内容：%s", res.StatusCode, string(body))
	}

	var response ChatResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}

	if len(response.Choices) > 0 {
		content := response.Choices[0].Message.Content
		// fmt.Print(content)
		file.WriteString(content)
		return content, nil
	}

	return "", nil
}

func QueryaiWithChain(Conversation []Message, filepath string) (NewConversation []Message, result string, err error) {
	// filepath := "../../public/aichatlog/log_" + time.Now().Format("06-01-02") + ".txt"
	file, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("Error opening file: %v\n", err)
		return nil, "", err
	}
	defer file.Close()

	url := config.Config.AiBaseUrl
	// 记录对话时间
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	file.WriteString(fmt.Sprintf("\n[%s] User:\n%s\n\n", timestamp, Conversation[len(Conversation)-1].Content))

	chatreq := ChatRequest{
		Model:       "deepseek-chat",
		Messages:    Conversation,
		Stream:      false,
		MaxTokens:   2048,
		Temperature: 0.7,
		TopP:        1,
	}

	// 构建请求体
	payload, err := json.Marshal(chatreq)
	if err != nil {
		return nil, "", err
	}
	// 发送请求
	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return nil, "", err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+config.Config.AiApi) // 替换为你的 API Key

	// 获取响应
	res, _ := http.DefaultClient.Do(req)
	log.Print("请求ai状态码：", res.StatusCode)
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, "", err
	}

	if res.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("请求失败：HTTP %d\n响应内容：%s", res.StatusCode, string(body))
	}

	var response ChatResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, "", err
	}

	if len(response.Choices) > 0 {
		content := response.Choices[0].Message.Content
		// fmt.Print(content)
		file.WriteString(content)
		Conversation = append(Conversation, response.Choices[0].Message)
		return Conversation, content, nil
	}

	return nil, "", nil
}
