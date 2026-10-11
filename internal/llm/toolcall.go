package llm

import (
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// ToolCallValidationError describes a malformed assistant/tool message
// sequence. Index is the zero-based message index when the error comes from
// a history, and -1 identifies a standalone model response.
type ToolCallValidationError struct {
	Index  int
	Code   string
	CallID string
}

func (e *ToolCallValidationError) Error() string {
	if e == nil {
		return "invalid tool call sequence"
	}
	location := "model response"
	if e.Index >= 0 {
		location = fmt.Sprintf("message %d", e.Index)
	}
	if e.CallID != "" {
		return fmt.Sprintf("invalid tool call sequence at %s: %s (call_id=%s)", location, e.Code, e.CallID)
	}
	return fmt.Sprintf("invalid tool call sequence at %s: %s", location, e.Code)
}

// ValidateToolCallHistory validates a complete request history. A history
// cannot end with an assistant tool call because the corresponding results
// must be present before it is sent to the model again.
func ValidateToolCallHistory(messages []openai.ChatCompletionMessage) error {
	pending := make(map[string]int)
	order := make([]string, 0)
	seenCalls := make(map[string]struct{})
	completed := make(map[string]struct{})

	for index, message := range messages {
		role := strings.TrimSpace(message.Role)
		switch role {
		case openai.ChatMessageRoleAssistant:
			if len(pending) > 0 {
				return &ToolCallValidationError{Index: index, Code: "previous tool call is missing a result"}
			}
			ids, err := validateAssistantToolCalls(message, index)
			if err != nil {
				return err
			}
			for _, id := range ids {
				if _, exists := seenCalls[id]; exists {
					return &ToolCallValidationError{Index: index, Code: "duplicate tool_call_id", CallID: id}
				}
				seenCalls[id] = struct{}{}
				pending[id] = index
				order = append(order, id)
			}

		case openai.ChatMessageRoleTool:
			callID := strings.TrimSpace(message.ToolCallID)
			if callID == "" {
				return &ToolCallValidationError{Index: index, Code: "tool result has no call_id"}
			}
			if _, ok := completed[callID]; ok {
				return &ToolCallValidationError{Index: index, Code: "duplicate tool result", CallID: callID}
			}
			if len(pending) == 0 {
				return &ToolCallValidationError{Index: index, Code: "orphan tool result", CallID: callID}
			}
			if _, ok := pending[callID]; !ok {
				return &ToolCallValidationError{Index: index, Code: "unknown tool_call_id", CallID: callID}
			}
			if order[0] != callID {
				return &ToolCallValidationError{Index: index, Code: "tool result is out of order", CallID: callID}
			}
			delete(pending, callID)
			completed[callID] = struct{}{}
			order = order[1:]

		default:
			if len(pending) > 0 {
				return &ToolCallValidationError{Index: index, Code: "tool result must follow its assistant tool call"}
			}
		}
	}

	if len(pending) > 0 {
		return &ToolCallValidationError{Index: len(messages) - 1, Code: "tool call is missing a result", CallID: order[0]}
	}
	return nil
}

// ValidateToolCallResponse validates a model response against a complete
// history. The response is not part of the history yet, so assistant tool
// calls are checked without requiring their results until the caller executes
// them and appends the tool messages.
func ValidateToolCallResponse(history []openai.ChatCompletionMessage, response openai.ChatCompletionMessage) error {
	if err := ValidateToolCallHistory(history); err != nil {
		return err
	}
	if role := strings.TrimSpace(response.Role); role != "" && role != openai.ChatMessageRoleAssistant {
		return &ToolCallValidationError{Index: -1, Code: "model response must be an assistant message"}
	}
	ids, err := validateAssistantToolCalls(response, -1)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{})
	for _, message := range history {
		for _, call := range message.ToolCalls {
			seen[strings.TrimSpace(call.ID)] = struct{}{}
		}
	}
	for _, id := range ids {
		if _, exists := seen[id]; exists {
			return &ToolCallValidationError{Index: -1, Code: "duplicate tool_call_id", CallID: id}
		}
	}
	return nil
}

func validateAssistantToolCalls(message openai.ChatCompletionMessage, index int) ([]string, error) {
	if len(message.ToolCalls) == 0 {
		return nil, nil
	}

	ids := make([]string, 0, len(message.ToolCalls))
	seen := make(map[string]struct{}, len(message.ToolCalls))
	for _, call := range message.ToolCalls {
		callID := strings.TrimSpace(call.ID)
		if callID == "" {
			return nil, &ToolCallValidationError{Index: index, Code: "assistant tool call has no call_id"}
		}
		if _, exists := seen[callID]; exists {
			return nil, &ToolCallValidationError{Index: index, Code: "duplicate tool_call_id", CallID: callID}
		}
		if strings.TrimSpace(string(call.Type)) != "" && call.Type != openai.ToolTypeFunction {
			return nil, &ToolCallValidationError{Index: index, Code: "unsupported tool call type", CallID: callID}
		}
		if strings.TrimSpace(call.Function.Name) == "" {
			return nil, &ToolCallValidationError{Index: index, Code: "assistant tool call has no function name", CallID: callID}
		}
		seen[callID] = struct{}{}
		ids = append(ids, callID)
	}
	return ids, nil
}
