package llm

import (
	"context"
	"fmt"
	"strings"

	"project-yume/internal/config"
	"project-yume/internal/utils"

	openai "github.com/sashabaranov/go-openai"
)

type Result struct {
	Response  openai.ChatCompletionResponse
	Task      Task
	Provider  string
	Model     string
	RequestID string
}

type Orchestrator struct {
	providers []Provider
	models    map[Task]string
}

func NewOrchestrator(providers []Provider, models map[Task]string) (*Orchestrator, error) {
	validProviders := make([]Provider, 0, len(providers))
	for _, provider := range providers {
		if provider == nil || strings.TrimSpace(provider.Name()) == "" {
			continue
		}
		validProviders = append(validProviders, provider)
	}
	if len(validProviders) == 0 {
		return nil, fmt.Errorf("at least one LLM provider is required")
	}
	copyModels := make(map[Task]string, len(models))
	for task, model := range models {
		copyModels[normalizeTask(task)] = strings.TrimSpace(model)
	}
	return &Orchestrator{providers: validProviders, models: copyModels}, nil
}

func NewDefaultOrchestrator() *Orchestrator {
	cfg := config.GetConfig()
	models := map[Task]string{
		TaskReply:   cfg.AiReplyModel,
		TaskMemory:  cfg.AiMemoryModel,
		TaskPlanner: cfg.AiPlannerModel,
	}
	orchestrator, err := NewOrchestrator([]Provider{NewAIFunctionProvider()}, models)
	if err != nil {
		panic(err)
	}
	return orchestrator
}

func (o *Orchestrator) Model(task Task) string {
	if o == nil {
		return ""
	}
	task = normalizeTask(task)
	if model := strings.TrimSpace(o.models[task]); model != "" {
		return model
	}
	return strings.TrimSpace(config.GetConfig().AiModel)
}

func (o *Orchestrator) Complete(ctx context.Context, task Task, request openai.ChatCompletionRequest) (Result, error) {
	if o == nil || len(o.providers) == 0 {
		return Result{}, fmt.Errorf("LLM orchestrator is not configured")
	}
	task = normalizeTask(task)
	if strings.TrimSpace(request.Model) == "" {
		request.Model = o.Model(task)
	}
	requestID := utils.NewRequestID("model")
	for _, provider := range o.providers {
		response, err := provider.Complete(ctx, request)
		if err != nil {
			return Result{Task: task, Provider: provider.Name(), Model: request.Model, RequestID: requestID}, err
		}
		model := strings.TrimSpace(response.Model)
		if model == "" {
			model = request.Model
		}
		return Result{Response: response, Task: task, Provider: provider.Name(), Model: model, RequestID: requestID}, nil
	}
	return Result{}, fmt.Errorf("no LLM provider available for task %s", task)
}
