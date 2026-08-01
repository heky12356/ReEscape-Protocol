package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"project-yume/internal/aifunction"
	"project-yume/internal/config"
	"project-yume/internal/eventlog"
	"project-yume/internal/state"
	"project-yume/internal/tools"
	"project-yume/internal/utils"

	openai "github.com/sashabaranov/go-openai"
)

type Runtime struct {
	registry *tools.Registry
	executor *tools.Executor
	budget   Budget
}

func NewRuntime(registry *tools.Registry, executor *tools.Executor, budget Budget) *Runtime {
	return &Runtime{
		registry: registry,
		executor: executor,
		budget:   budget,
	}
}

func (r *Runtime) RunTurn(ctx context.Context, turn *TurnContext) (TurnResult, error) {
	if turn == nil {
		return TurnResult{}, fmt.Errorf("turn context is nil")
	}
	budget := r.budget
	if budget.MaxSteps <= 0 || budget.TotalTimeout <= 0 {
		budget = BudgetFromConfig()
	}

	runCtx, cancel := context.WithTimeout(ctx, budget.TotalTimeout)
	defer cancel()

	state.GetManager().EnsureSession(turn.SessionID(), turn.UserID(), turn.GroupID(), turn.ChatType())
	messages := r.buildMessages(turn)
	toolPolicy := r.toolPolicy()
	openAITools := tools.ToOpenAIToolsWithPolicy(r.registry, toolPolicy)
	trace := Trace{RequestID: turn.RequestID(), SessionID: turn.SessionID()}
	events := make([]eventlog.Event, 0)

	for step := 0; step < budget.MaxSteps; step++ {
		req := openai.ChatCompletionRequest{
			Model:             config.GetConfig().AiModel,
			Messages:          messages,
			Tools:             openAITools,
			ToolChoice:        "auto",
			ParallelToolCalls: false,
			Stream:            false,
			MaxTokens:         config.GetConfig().AiMaxTokens,
			Temperature:       config.GetConfig().AiTemperature,
			TopP:              config.GetConfig().AiTopP,
			N:                 1,
		}

		resp, err := aifunction.Chat(runCtx, req)
		if err != nil {
			return TurnResult{Trace: trace, Events: events}, err
		}
		if len(resp.Choices) == 0 {
			return TurnResult{Trace: trace, Events: events}, ErrEmptyModelResponse
		}

		message := resp.Choices[0].Message
		if len(message.ToolCalls) == 0 {
			reply := strings.TrimSpace(utils.CleanThinkTag(message.Content))
			if reply == "" {
				return TurnResult{Trace: trace, Events: events}, ErrEmptyModelResponse
			}
			events = append(events, eventlog.Event{
				Type:      "agent_final_reply",
				SessionID: turn.SessionID(),
				UserID:    turn.UserID(),
				Actor:     turn.Actor(),
				Message:   reply,
				CreatedAt: time.Now(),
			})
			return TurnResult{
				Handled:    true,
				ShouldSend: true,
				FinalReply: FinalReply{Content: reply},
				Trace:      trace,
				Events:     events,
			}, nil
		}

		messages = append(messages, message)
		for _, call := range message.ToolCalls {
			execution := r.executor.Execute(runCtx, turn, call)
			stepTrace := stepTraceFromExecution(step, call, execution)
			trace.Steps = append(trace.Steps, stepTrace)
			events = append(events, eventFromStepTrace(turn, stepTrace))
			events = append(events, execution.Result.Events...)
			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Name:       execution.ToolName,
				ToolCallID: call.ID,
				Content:    execution.Content,
			})
		}
	}

	return TurnResult{Trace: trace, Events: events}, ErrMaxStepsExceeded
}

func (r *Runtime) buildMessages(turn *TurnContext) []openai.ChatCompletionMessage {
	toolNames := r.availableToolNames()

	conversation := selectRecentMessages(state.GetManager().GetConversation(turn.SessionID()), config.GetConfig().ContextRecentTurns)
	messages := make([]openai.ChatCompletionMessage, 0, len(conversation)+3)
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: BuildSystemPrompt(toolNames),
	})
	if runtimeContext := buildRuntimeContext(turn); runtimeContext != "" {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: runtimeContext,
		})
	}
	messages = append(messages, conversation...)

	if turn.trigger == TriggerProactive {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: "请基于当前时间、会话状态和记忆，生成一条自然的主动私聊消息。不要解释，不要输出 JSON。",
		})
	}
	return messages
}

func (r *Runtime) availableToolNames() []string {
	if r == nil || r.registry == nil {
		return nil
	}
	toolsList := r.registry.ListAvailable(r.toolPolicy())
	toolNames := make([]string, 0, len(toolsList))
	for _, tool := range toolsList {
		toolNames = append(toolNames, tool.Name())
	}
	return toolNames
}

func (r *Runtime) toolPolicy() tools.Policy {
	if r == nil || r.executor == nil {
		return tools.Policy{}
	}
	return r.executor.Policy()
}

func selectRecentMessages(conversation []openai.ChatCompletionMessage, recentTurns int) []openai.ChatCompletionMessage {
	if recentTurns <= 0 {
		recentTurns = 8
	}

	start := 0
	seenUserTurns := 0
	for i := len(conversation) - 1; i >= 0; i-- {
		if conversation[i].Role != openai.ChatMessageRoleUser {
			continue
		}
		seenUserTurns++
		if seenUserTurns == recentTurns {
			start = i
			break
		}
	}

	result := make([]openai.ChatCompletionMessage, 0, len(conversation)-start)
	for _, msg := range conversation[start:] {
		if msg.Role != openai.ChatMessageRoleUser && msg.Role != openai.ChatMessageRoleAssistant {
			continue
		}
		if strings.TrimSpace(msg.Content) == "" && len(msg.MultiContent) == 0 {
			continue
		}
		result = append(result, msg)
	}
	return result
}

func stepTraceFromExecution(step int, call openai.ToolCall, execution tools.ExecutionResult) StepTrace {
	traceMode := config.GetConfig().ReactTraceMode
	item := StepTrace{
		Index:      step,
		ToolName:   execution.ToolName,
		ToolCallID: execution.ToolCallID,
		Error:      execution.Error,
		Denied:     execution.Denied,
		Duration:   time.Duration(execution.DurationMs) * time.Millisecond,
		FinishedAt: time.Now(),
	}
	if traceMode == "full" {
		item.Arguments = call.Function.Arguments
		item.Observation = execution.Content
		return item
	}
	if traceMode == "basic" {
		item.Observation = truncateTraceText(execution.Content, 160)
	}
	return item
}

func truncateTraceText(input string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(strings.TrimSpace(input))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit]) + "..."
}

func eventFromStepTrace(turn *TurnContext, step StepTrace) eventlog.Event {
	data := map[string]any{
		"step":        step.Index,
		"tool_call":   step.ToolCallID,
		"duration_ms": step.Duration.Milliseconds(),
		"denied":      step.Denied,
	}
	if step.Error != "" {
		data["error"] = step.Error
	}
	if step.Arguments != "" {
		data["arguments"] = step.Arguments
	}
	if step.Observation != "" {
		data["observation"] = step.Observation
	}
	return eventlog.Event{
		Type:      "tool_called",
		SessionID: turn.SessionID(),
		UserID:    turn.UserID(),
		Actor:     turn.Actor(),
		Tool:      step.ToolName,
		Data:      data,
		CreatedAt: step.FinishedAt,
	}
}
