package inbound

import (
	"errors"
	"fmt"

	"project-yume/internal/handler"
	"project-yume/internal/metrics"
)

type Stage interface {
	Name() string
	Process(ctx *handler.MessageContext) error
}

type Pipeline struct {
	stages []Stage
}

func NewPipeline(stages ...Stage) *Pipeline {
	return &Pipeline{stages: stages}
}

func (p *Pipeline) Run(ctx *handler.MessageContext) error {
	for _, stage := range p.stages {
		if err := stage.Process(ctx); err != nil {
			var skipErr *SkipError
			if errors.As(err, &skipErr) {
				metrics.IncCounter(
					"bot_inbound_stage_total",
					"Total inbound pipeline stage executions by result.",
					map[string]string{"stage": stage.Name(), "result": "drop"},
				)
				if ctx.DropReason == "" {
					ctx.DropReason = fmt.Sprintf("%s: %s", stage.Name(), skipErr.Reason)
				}
				return skipErr
			}
			metrics.IncCounter(
				"bot_inbound_stage_total",
				"Total inbound pipeline stage executions by result.",
				map[string]string{"stage": stage.Name(), "result": "error"},
			)
			return fmt.Errorf("%s failed: %w", stage.Name(), err)
		}
		metrics.IncCounter(
			"bot_inbound_stage_total",
			"Total inbound pipeline stage executions by result.",
			map[string]string{"stage": stage.Name(), "result": "pass"},
		)
	}
	return nil
}

type SkipError struct {
	Reason string
}

func (e *SkipError) Error() string {
	return e.Reason
}

func skip(reason string) error {
	return &SkipError{Reason: reason}
}
