package llm

import (
	"context"
	"errors"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

type testProvider struct {
	name   string
	model  string
	called bool
	err    error
}

func (p *testProvider) Name() string { return p.name }

func (p *testProvider) Complete(_ context.Context, request openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	p.called = true
	if p.err != nil {
		return openai.ChatCompletionResponse{}, p.err
	}
	return openai.ChatCompletionResponse{Model: p.model, ID: request.Model}, nil
}

func TestParseTask(t *testing.T) {
	for _, test := range []struct {
		raw  string
		want Task
		ok   bool
	}{{"reply", TaskReply, true}, {" MEMORY ", TaskMemory, true}, {"unknown", "", false}} {
		got, ok := ParseTask(test.raw)
		if got != test.want || ok != test.ok {
			t.Fatalf("ParseTask(%q) = %q, %v; want %q, %v", test.raw, got, ok, test.want, test.ok)
		}
	}
}

func TestOrchestratorRoutesTaskToConfiguredModel(t *testing.T) {
	provider := &testProvider{name: "fake", model: "reply-result"}
	orchestrator, err := NewOrchestrator([]Provider{provider}, map[Task]string{TaskReply: "reply-model", TaskMemory: "memory-model"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := orchestrator.Complete(context.Background(), TaskMemory, openai.ChatCompletionRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !provider.called || result.Task != TaskMemory || result.Provider != "fake" || result.Model != "reply-result" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Response.ID != "memory-model" {
		t.Fatalf("provider received wrong model: %q", result.Response.ID)
	}
}

func TestOrchestratorPreservesProviderError(t *testing.T) {
	want := errors.New("provider unavailable")
	orchestrator, err := NewOrchestrator([]Provider{&testProvider{name: "fake", err: want}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := orchestrator.Complete(context.Background(), TaskPlanner, openai.ChatCompletionRequest{Model: "planner-model"})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	if result.Failure == nil || result.Failure.Provider != "fake" || result.Failure.Error == "" {
		t.Fatalf("expected sanitized failure snapshot, got %#v", result.Failure)
	}
}
