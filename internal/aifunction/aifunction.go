package aifunction

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"project-yume/internal/config"
)

type ChatResponse struct {
	ID      string    `json:"id"`
	Object  string    `json:"object"`
	Created int64     `json:"created"`
	Model   string    `json:"model"`
	Choices []Choice  `json:"choices"`
	Usage   UsageInfo `json:"usage"`
}

type Choice struct {
	Index        int     `json:"index"`
	FinishReason string  `json:"finish_reason"`
	Message      Message `json:"message"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type UsageInfo struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func Queryai(msg string) (string, error) {
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

	// 构建请求体
	payload := fmt.Sprintf(`{
            "model": "deepseek-chat",
            "messages": [
                {
					"role": "system",
					"content": "%s"
				},
				{
                    "role": "user",
                    "content": "%s"
                }
            ],
            "stream": false,
            "max_tokens": 2048,
            "temperature": 0.7
        }`, config.Config.AiPrompt, msg)

	// 发送请求
	req, _ := http.NewRequest("POST", url, strings.NewReader(payload))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+config.Config.AiApi) // 替换为你的 API Key

	// 获取响应
	res, _ := http.DefaultClient.Do(req)
	log.Println(res.StatusCode)
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	var response ChatResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}

	if len(response.Choices) > 0 {
		content := response.Choices[0].Message.Content
		fmt.Print(content)
		file.WriteString(content)
		return content, nil
	}

	return "", nil
}
