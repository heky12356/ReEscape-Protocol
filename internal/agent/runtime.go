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
	cfg := config.GetConfig()
	correlation := eventlog.Correlation{
		RequestID: turn.RequestID(), TurnID: turn.TurnID(), SessionID: turn.SessionID(),
		UserID: turn.UserID(), Actor: turn.Actor(), Model: cfg.AiModel,
	}
	events := make([]eventlog.Event, 0)
	for _, event := range resolveTurnSkills(turn) {
		events = append(events, event.WithCorrelation(correlation))
	}
	scheduleManaged := false

	for step := 0; step < budget.MaxSteps; step++ {
		req := openai.ChatCompletionRequest{
			Model:             cfg.AiModel,
			Messages:          messages,
			Tools:             openAITools,
			ToolChoice:        r.toolChoiceForStep(turn, step),
			ParallelToolCalls: false,
			Stream:            false,
			MaxTokens:         config.GetConfig().AiMaxTokens,
			Temperature:       config.GetConfig().AiTemperature,
			TopP:              config.GetConfig().AiTopP,
			N:                 1,
		}

		resp, err := aifunction.Chat(runCtx, req)
		if err != nil {
			return TurnResult{Trace: trace, Events: events, ScheduleManaged: scheduleManaged}, err
		}
		if len(resp.Choices) == 0 {
			return TurnResult{Trace: trace, Events: events, ScheduleManaged: scheduleManaged}, ErrEmptyModelResponse
		}

		message := resp.Choices[0].Message
		if len(message.ToolCalls) == 0 {
			reply := strings.TrimSpace(utils.CleanThinkTag(message.Content))
			if reply == "" {
				return TurnResult{Trace: trace, Events: events, ScheduleManaged: scheduleManaged}, ErrEmptyModelResponse
			}
			events = append(events, eventlog.Event{
				Type:      "agent_final_reply",
				SessionID: turn.SessionID(),
				UserID:    turn.UserID(),
				Actor:     turn.Actor(),
				Message:   reply,
				CreatedAt: time.Now(),
			}.WithCorrelation(correlation))
			return TurnResult{
				Handled:         true,
				ShouldSend:      true,
				FinalReply:      FinalReply{Content: reply},
				ScheduleManaged: scheduleManaged,
				Trace:           trace,
				Events:          events,
			}, nil
		}

		messages = append(messages, message)
		for _, call := range message.ToolCalls {
			execution := r.executor.Execute(runCtx, turn, call)
			if isScheduleManagingTool(execution) {
				scheduleManaged = true
			}
			stepTrace := stepTraceFromExecution(step, call, execution)
			trace.Steps = append(trace.Steps, stepTrace)
			events = append(events, eventFromStepTrace(turn, stepTrace))
			for _, event := range execution.Result.Events {
				events = append(events, event.WithCorrelation(correlation))
			}
			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Name:       execution.ToolName,
				ToolCallID: call.ID,
				Content:    execution.Content,
			})
		}
	}

	return TurnResult{Trace: trace, Events: events, ScheduleManaged: scheduleManaged}, ErrMaxStepsExceeded
}

func (r *Runtime) toolChoiceForStep(turn *TurnContext, step int) any {
	cfg := config.GetConfig()
	if cfg == nil ||
		!cfg.SkillForceReadOnCandidate ||
		step != 0 ||
		turn == nil ||
		len(turn.ActivatedSkills()) > 0 ||
		len(turn.SkillCandidates()) != 1 ||
		!containsTool(r.availableToolNames(), "read_skill") {
		return "auto"
	}
	return openai.ToolChoice{
		Type: openai.ToolTypeFunction,
		Function: openai.ToolFunction{
			Name: "read_skill",
		},
	}
}

func isScheduleManagingTool(execution tools.ExecutionResult) bool {
	return execution.ToolName == "update_proactive_schedule" && execution.Result.Mutated
}

func (r *Runtime) buildMessages(turn *TurnContext) []openai.ChatCompletionMessage {
	toolNames := r.availableToolNames()

	conversation, currentTurnDeduplicated := selectConversationBeforeTurn(state.GetManager().GetConversation(turn.SessionID()), turn)
	conversation = selectRecentMessages(conversation, config.GetConfig().ContextRecentTurns)
	messages := make([]openai.ChatCompletionMessage, 0, len(conversation)+2)
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: BuildSystemPrompt(toolNames),
	})
	messages = append(messages, conversation...)
	if currentTurnMessage, ok := buildCurrentTurnMessage(turn, currentTurnDeduplicated || turn.trigger == TriggerProactive); ok {
		messages = append(messages, currentTurnMessage)
	}
	return messages
}

func buildCurrentTurnMessage(turn *TurnContext, includeUserMessage bool) (openai.ChatCompletionMessage, bool) {
	if turn == nil {
		return openai.ChatCompletionMessage{}, false
	}
	if turn.trigger == TriggerProactive {
		return openai.ChatCompletionMessage{
			Role: openai.ChatMessageRoleUser,
			Content: buildCurrentTurnEnvelope(turn,
				"【Proactive Task】\n请基于当前时间、会话状态和记忆，生成一条自然的主动私聊消息。不要解释，不要输出 JSON。"),
		}, true
	}

	task := ""
	if includeUserMessage {
		task = buildUserMessageSection(turn.Message())
	}
	envelope := buildCurrentTurnEnvelope(turn, task)
	parts := buildCurrentTurnMultiContent(turn, envelope)
	if len(parts) > 0 {
		return openai.ChatCompletionMessage{
			Role:         openai.ChatMessageRoleUser,
			MultiContent: parts,
		}, true
	}
	if strings.TrimSpace(envelope) == "" {
		return openai.ChatCompletionMessage{}, false
	}
	return openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: envelope,
	}, true
}

func buildUserMessageSection(message string) string {
	if strings.TrimSpace(message) == "" {
		return "【User Message】"
	}
	return "【User Message】\n" + message
}

func buildCurrentTurnMultiContent(turn *TurnContext, text string) []openai.ChatMessagePart {
	cfg := config.GetConfig()
	if turn == nil || cfg == nil || !cfg.EnableVisionInput {
		return nil
	}

	parts := make([]openai.ChatMessagePart, 0, len(turn.Parts())+1)
	if strings.TrimSpace(text) != "" {
		parts = append(parts, openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeText,
			Text: text,
		})
	}

	detail := normalizeRuntimeVisionImageDetail(cfg.VisionImageDetail)
	hasImage := false
	for _, part := range turn.Parts() {
		if part.Type != "image" || strings.TrimSpace(part.URL) == "" {
			continue
		}
		hasImage = true
		parts = append(parts, openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeImageURL,
			ImageURL: &openai.ChatMessageImageURL{
				URL:    strings.TrimSpace(part.URL),
				Detail: detail,
			},
		})
	}
	if !hasImage {
		return nil
	}
	return parts
}

func normalizeRuntimeVisionImageDetail(raw string) openai.ImageURLDetail {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(openai.ImageURLDetailHigh):
		return openai.ImageURLDetailHigh
	case string(openai.ImageURLDetailLow):
		return openai.ImageURLDetailLow
	default:
		return openai.ImageURLDetailAuto
	}
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

func selectConversationBeforeTurn(conversation []openai.ChatCompletionMessage, turn *TurnContext) ([]openai.ChatCompletionMessage, bool) {
	result := append([]openai.ChatCompletionMessage(nil), conversation...)
	if turn == nil || turn.trigger == TriggerProactive || len(result) == 0 {
		return result, false
	}

	lastIndex := len(result) - 1
	if isCurrentTurnConversationMessage(result[lastIndex], turn) {
		return result[:lastIndex], true
	}
	return result, false
}

func isCurrentTurnConversationMessage(message openai.ChatCompletionMessage, turn *TurnContext) bool {
	if turn == nil || message.Role != openai.ChatMessageRoleUser {
		return false
	}
	if len(message.MultiContent) > 0 {
		return multiContentMatchesTurn(message.MultiContent, turn)
	}
	return strings.TrimSpace(message.Content) == strings.TrimSpace(turn.Message())
}

func multiContentMatchesTurn(parts []openai.ChatMessagePart, turn *TurnContext) bool {
	textParts := make([]string, 0, len(parts))
	imageURLs := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case openai.ChatMessagePartTypeText:
			if strings.TrimSpace(part.Text) != "" {
				textParts = append(textParts, part.Text)
			}
		case openai.ChatMessagePartTypeImageURL:
			if part.ImageURL != nil && strings.TrimSpace(part.ImageURL.URL) != "" {
				imageURLs = append(imageURLs, strings.TrimSpace(part.ImageURL.URL))
			}
		}
	}

	if strings.TrimSpace(strings.Join(textParts, "\n")) != strings.TrimSpace(turn.Message()) {
		return false
	}

	expectedImageURLs := make([]string, 0, len(turn.Parts()))
	for _, part := range turn.Parts() {
		if part.Type == "image" && strings.TrimSpace(part.URL) != "" {
			expectedImageURLs = append(expectedImageURLs, strings.TrimSpace(part.URL))
		}
	}
	if len(imageURLs) != len(expectedImageURLs) {
		return false
	}
	for i := range imageURLs {
		if imageURLs[i] != expectedImageURLs[i] {
			return false
		}
	}
	return true
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
		Type:       "tool_called",
		SessionID:  turn.SessionID(),
		UserID:     turn.UserID(),
		Actor:      turn.Actor(),
		Tool:       step.ToolName,
		ToolName:   step.ToolName,
		ToolCallID: step.ToolCallID,
		Data:       data,
		CreatedAt:  step.FinishedAt,
	}.WithCorrelation(eventlog.Correlation{
		RequestID: turn.RequestID(), TurnID: turn.TurnID(), SessionID: turn.SessionID(),
		UserID: turn.UserID(), Actor: turn.Actor(), ToolName: step.ToolName,
		ToolCallID: step.ToolCallID, Model: config.GetConfig().AiModel,
	})
}
