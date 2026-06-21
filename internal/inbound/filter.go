package inbound

import (
	"strings"

	"project-yume/internal/config"
	"project-yume/internal/handler"
)

type FilterStage struct{}

func NewFilterStage() *FilterStage {
	return &FilterStage{}
}

func (s *FilterStage) Name() string {
	return "filter"
}

func (s *FilterStage) Process(ctx *handler.MessageContext) error {
	cfg := config.GetConfig()

	if ctx.ChatType != 1 {
		return skip("non-private message")
	}
	if ctx.UserID != cfg.TargetId {
		return skip("non-target user")
	}
	if strings.TrimSpace(ctx.RawMessage) == "" {
		return skip("empty message")
	}
	if ctx.RawMessage == "exit();" {
		return skip("control command")
	}

	return nil
}
