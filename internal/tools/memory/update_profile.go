package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"project-yume/internal/eventlog"
	domainmemory "project-yume/internal/memory"
	"project-yume/internal/tools"
)

type UpdateProfileTool struct{}

type updateProfileInput struct {
	PreferredTone     string   `json:"preferred_tone"`
	ReplyStyle        string   `json:"reply_style"`
	RelationshipStyle string   `json:"relationship_style"`
	Likes             []string `json:"likes"`
	Dislikes          []string `json:"dislikes"`
	Taboos            []string `json:"taboos"`
	Reason            string   `json:"reason"`
}

func NewUpdateProfileTool() *UpdateProfileTool {
	return &UpdateProfileTool{}
}

func (t *UpdateProfileTool) Name() string {
	return "update_profile"
}

func (t *UpdateProfileTool) Description() string {
	return "更新用户明确表达的长期画像偏好，例如希望的回复语气、关系风格、喜欢/不喜欢、禁忌和以后如何互动。"
}

func (t *UpdateProfileTool) Schema() tools.Schema {
	return tools.Schema{
		Type: "object",
		Properties: map[string]tools.Property{
			"preferred_tone": {
				Type:        "string",
				Description: "用户偏好的回复语气。",
			},
			"reply_style": {
				Type:        "string",
				Description: "用户偏好的回复长短或表达方式。",
			},
			"relationship_style": {
				Type:        "string",
				Description: "用户偏好的关系互动方式。",
			},
			"likes": {
				Type:        "array",
				Description: "用户喜欢的对象或主题。",
				Items: &tools.Property{
					Type: "string",
				},
			},
			"dislikes": {
				Type:        "array",
				Description: "用户不喜欢的对象或主题。",
				Items: &tools.Property{
					Type: "string",
				},
			},
			"taboos": {
				Type:        "array",
				Description: "用户要求避免的表达、话题或行为。",
				Items: &tools.Property{
					Type: "string",
				},
			},
			"reason": {
				Type:        "string",
				Description: "更新原因，便于事件审计。",
			},
		},
		AdditionalProperties: false,
	}
}

func (t *UpdateProfileTool) ReadOnly() bool {
	return false
}

func (t *UpdateProfileTool) Execute(ctx context.Context, turn tools.TurnView, input json.RawMessage) (tools.ToolResult, error) {
	select {
	case <-ctx.Done():
		return tools.ToolResult{}, ctx.Err()
	default:
	}

	var args updateProfileInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &args); err != nil {
			return tools.ToolResult{}, err
		}
	}

	patch := domainmemory.ProfilePatch{
		PreferredTone:     strings.TrimSpace(args.PreferredTone),
		ReplyStyle:        strings.TrimSpace(args.ReplyStyle),
		RelationshipStyle: strings.TrimSpace(args.RelationshipStyle),
		Likes:             cleanStringSlice(args.Likes),
		Dislikes:          cleanStringSlice(args.Dislikes),
		Taboos:            cleanStringSlice(args.Taboos),
	}
	if patch.PreferredTone == "" &&
		patch.ReplyStyle == "" &&
		patch.RelationshipStyle == "" &&
		len(patch.Likes) == 0 &&
		len(patch.Dislikes) == 0 &&
		len(patch.Taboos) == 0 {
		return tools.ToolResult{}, fmt.Errorf("at least one profile field is required")
	}

	domainmemory.GetProfileManager().ApplyPatch(turn.UserID(), patch)
	profile := domainmemory.GetProfileManager().GetProfile(turn.UserID())
	reason := strings.TrimSpace(args.Reason)
	if reason == "" {
		reason = "profile updated by agent tool"
	}

	event := eventlog.Event{
		Type:      "memory_profile_updated",
		SessionID: turn.SessionID(),
		UserID:    turn.UserID(),
		Actor:     turn.Actor(),
		Tool:      t.Name(),
		Message:   reason,
		Data: map[string]any{
			"preferred_tone":     patch.PreferredTone,
			"reply_style":        patch.ReplyStyle,
			"relationship_style": patch.RelationshipStyle,
			"likes":              patch.Likes,
			"dislikes":           patch.Dislikes,
			"taboos":             patch.Taboos,
		},
	}

	return tools.ToolResult{
		Content: "profile updated",
		Data:    profile,
		Events:  []eventlog.Event{event},
		Mutated: true,
	}, nil
}

func cleanStringSlice(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}
