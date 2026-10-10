package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"project-yume/internal/metrics"

	openai "github.com/sashabaranov/go-openai"
)

type Executor struct {
	registry *Registry
	policy   Policy
	timeout  time.Duration
}

func NewExecutor(registry *Registry, policy Policy, timeout time.Duration) *Executor {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &Executor{
		registry: registry,
		policy:   policy,
		timeout:  timeout,
	}
}

func (e *Executor) Policy() Policy {
	if e == nil {
		return Policy{}
	}
	return e.policy
}

func (e *Executor) Execute(ctx context.Context, turn TurnView, call openai.ToolCall) ExecutionResult {
	startedAt := time.Now()
	name := strings.TrimSpace(call.Function.Name)
	result := ExecutionResult{
		ToolCallID: call.ID,
		ToolName:   name,
	}

	tool, ok := e.registry.Get(name)
	if !ok {
		result.Error = fmt.Sprintf("unknown tool: %s", name)
		result.Content = result.Error
		result.DurationMs = time.Since(startedAt).Milliseconds()
		recordToolMetrics(name, "unknown", "unknown", time.Since(startedAt))
		return result
	}
	source := toolSource(tool)

	if err := e.policy.Check(tool); err != nil {
		result.Error = err.Error()
		result.Content = result.Error
		result.Denied = true
		result.DurationMs = time.Since(startedAt).Milliseconds()
		recordToolMetrics(name, source, "denied", time.Since(startedAt))
		return result
	}

	rawArgs := json.RawMessage(call.Function.Arguments)
	if len(rawArgs) == 0 {
		rawArgs = json.RawMessage(`{}`)
	}
	if !json.Valid(rawArgs) {
		result.Error = fmt.Sprintf("invalid tool arguments: %s", call.Function.Arguments)
		result.Content = result.Error
		result.DurationMs = time.Since(startedAt).Milliseconds()
		recordToolMetrics(name, source, "invalid_arguments", time.Since(startedAt))
		return result
	}

	toolCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	toolResult, err := e.safeExecute(toolCtx, turn, tool, rawArgs)
	result.DurationMs = time.Since(startedAt).Milliseconds()
	result.Result = toolResult
	if err != nil {
		result.Error = err.Error()
		result.Content = err.Error()
		recordToolMetrics(name, source, "error", time.Since(startedAt))
		return result
	}

	result.Content = summarizeToolResult(toolResult)
	recordToolMetrics(name, source, "success", time.Since(startedAt))
	return result
}

type sourceAwareTool interface {
	Source() string
}

func toolSource(tool Tool) string {
	if sourceTool, ok := tool.(sourceAwareTool); ok {
		if source := strings.TrimSpace(sourceTool.Source()); source != "" {
			return source
		}
	}
	return "builtin"
}

func recordToolMetrics(name, source, result string, duration time.Duration) {
	if strings.TrimSpace(name) == "" {
		name = "unknown"
	}
	if strings.TrimSpace(source) == "" {
		source = "unknown"
	}
	metrics.IncCounter("bot_tool_executions_total", "Tool execution outcomes.", map[string]string{
		"tool": name, "source": source, "result": result,
	})
	metrics.ObserveDuration("bot_tool_duration", "Tool execution duration.", duration, map[string]string{
		"tool": name, "source": source,
	})
}

func (e *Executor) safeExecute(ctx context.Context, turn TurnView, tool Tool, input json.RawMessage) (result ToolResult, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("tool panic: %v", recovered)
		}
	}()
	return tool.Execute(ctx, turn, input)
}

func summarizeToolResult(result ToolResult) string {
	content := strings.TrimSpace(result.Content)
	if content != "" {
		return content
	}
	if result.Data == nil {
		return "{}"
	}
	data, err := json.Marshal(result.Data)
	if err != nil {
		return "{}"
	}
	return string(data)
}
