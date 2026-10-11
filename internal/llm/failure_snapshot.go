package llm

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"project-yume/internal/utils"

	openai "github.com/sashabaranov/go-openai"
)

// FailureSnapshot is a safe, replayable description of a failed model
// request. It deliberately does not retain prompt text, tool arguments or
// image URLs.
type FailureSnapshot struct {
	ID        string         `json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	Task      string         `json:"task"`
	Provider  string         `json:"provider,omitempty"`
	Model     string         `json:"model,omitempty"`
	RequestID string         `json:"request_id,omitempty"`
	Error     string         `json:"error"`
	Request   FailureRequest `json:"request"`
}

type FailureRequest struct {
	Model       string           `json:"model"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Temperature float32          `json:"temperature,omitempty"`
	TopP        float32          `json:"top_p,omitempty"`
	N           int              `json:"n,omitempty"`
	Messages    []FailureMessage `json:"messages"`
	ToolNames   []string         `json:"tool_names,omitempty"`
}

type FailureMessage struct {
	Role        string            `json:"role"`
	Name        string            `json:"name,omitempty"`
	ToolCallID  string            `json:"tool_call_id,omitempty"`
	Content     string            `json:"content,omitempty"`
	ContentHash string            `json:"content_hash,omitempty"`
	ToolCalls   []FailureToolCall `json:"tool_calls,omitempty"`
	Parts       []FailurePart     `json:"parts,omitempty"`
}

type FailureToolCall struct {
	ID            string `json:"id"`
	Type          string `json:"type,omitempty"`
	Name          string `json:"name,omitempty"`
	ArgumentsHash string `json:"arguments_hash,omitempty"`
}

type FailurePart struct {
	Type      string `json:"type"`
	TextHash  string `json:"text_hash,omitempty"`
	ImageHash string `json:"image_hash,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

var sensitiveErrorPattern = regexp.MustCompile(`(?i)(api[-_ ]?key|authorization|bearer|token|secret|password)\s*[:=]\s*[^\s,;]+`)

func NewFailureSnapshot(task Task, provider, model, requestID string, request openai.ChatCompletionRequest, err error) FailureSnapshot {
	message := "unknown model failure"
	if err != nil {
		message = sanitizeError(err.Error())
	}
	snapshot := FailureSnapshot{
		ID:        utils.NewRequestID("llm-failure"),
		CreatedAt: time.Now(),
		Task:      normalizeTask(task).String(),
		Provider:  strings.TrimSpace(provider),
		Model:     strings.TrimSpace(model),
		RequestID: strings.TrimSpace(requestID),
		Error:     message,
		Request:   sanitizeRequest(request),
	}
	return snapshot
}

// ReplayRequest creates a request containing only placeholders. It never
// invokes a provider; callers must explicitly replace placeholders before any
// manual, isolated replay.
func (s FailureSnapshot) ReplayRequest() (openai.ChatCompletionRequest, error) {
	if strings.TrimSpace(s.Model) == "" && strings.TrimSpace(s.Request.Model) == "" {
		return openai.ChatCompletionRequest{}, fmt.Errorf("failure snapshot has no model")
	}
	model := s.Model
	if strings.TrimSpace(model) == "" {
		model = s.Request.Model
	}
	messages := make([]openai.ChatCompletionMessage, 0, len(s.Request.Messages))
	for _, message := range s.Request.Messages {
		content := message.Content
		if content == "" && message.ContentHash != "" {
			content = "[REDACTED CONTENT]"
		}
		calls := make([]openai.ToolCall, 0, len(message.ToolCalls))
		for _, call := range message.ToolCalls {
			calls = append(calls, openai.ToolCall{
				ID: call.ID, Type: openai.ToolType(call.Type),
				Function: openai.FunctionCall{Name: call.Name, Arguments: "{}"},
			})
		}
		messages = append(messages, openai.ChatCompletionMessage{
			Role: message.Role, Name: message.Name, ToolCallID: message.ToolCallID,
			Content: content, ToolCalls: calls,
		})
	}
	return openai.ChatCompletionRequest{
		Model: model, MaxTokens: s.Request.MaxTokens, Temperature: s.Request.Temperature,
		TopP: s.Request.TopP, N: s.Request.N, Stream: false, Messages: messages,
	}, nil
}

func sanitizeRequest(request openai.ChatCompletionRequest) FailureRequest {
	result := FailureRequest{
		Model: request.Model, MaxTokens: request.MaxTokens, Temperature: request.Temperature,
		TopP: request.TopP, N: request.N,
		Messages:  make([]FailureMessage, 0, len(request.Messages)),
		ToolNames: make([]string, 0, len(request.Tools)),
	}
	for _, tool := range request.Tools {
		if tool.Function != nil && strings.TrimSpace(tool.Function.Name) != "" {
			result.ToolNames = append(result.ToolNames, tool.Function.Name)
		}
	}
	for _, message := range request.Messages {
		item := FailureMessage{Role: message.Role, Name: message.Name, ToolCallID: message.ToolCallID}
		if strings.TrimSpace(message.Content) != "" {
			item.ContentHash = digest(message.Content)
		}
		for _, call := range message.ToolCalls {
			item.ToolCalls = append(item.ToolCalls, FailureToolCall{
				ID: call.ID, Type: string(call.Type), Name: call.Function.Name,
				ArgumentsHash: digest(call.Function.Arguments),
			})
		}
		for _, part := range message.MultiContent {
			failurePart := FailurePart{Type: string(part.Type)}
			failurePart.TextHash = digest(part.Text)
			if part.ImageURL != nil {
				failurePart.ImageHash = digest(part.ImageURL.URL)
				failurePart.Detail = string(part.ImageURL.Detail)
			}
			item.Parts = append(item.Parts, failurePart)
		}
		result.Messages = append(result.Messages, item)
	}
	return result
}

func digest(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}

func sanitizeError(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown model failure"
	}
	return sensitiveErrorPattern.ReplaceAllString(value, "$1=[REDACTED]")
}
