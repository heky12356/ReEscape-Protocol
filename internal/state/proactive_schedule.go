package state

import (
	"strings"
	"time"
)

const maxProactiveScheduleSummaryRunes = 120

type ProactiveSchedule struct {
	SessionID         string         `json:"session_id"`
	IntentID          string         `json:"intent_id,omitempty"`
	NextScheduledAt   time.Time      `json:"next_scheduled_at,omitempty"`
	LastProactiveAt   time.Time      `json:"last_proactive_at,omitempty"`
	LastInteractionAt time.Time      `json:"last_interaction_at,omitempty"`
	Summary           string         `json:"summary,omitempty"`
	Reason            string         `json:"reason,omitempty"`
	Meta              map[string]any `json:"meta,omitempty"`
	Manual            bool           `json:"manual"`
	UpdatedAt         time.Time      `json:"updated_at,omitempty"`
	UpdatedBy         string         `json:"updated_by,omitempty"`
}

func cloneScheduleMeta(meta map[string]any) map[string]any {
	if len(meta) == 0 {
		return nil
	}

	result := make(map[string]any, len(meta))
	for key, value := range meta {
		switch typed := value.(type) {
		case []string:
			result[key] = append([]string(nil), typed...)
		case []any:
			values := make([]string, 0, len(typed))
			ok := true
			for _, item := range typed {
				text, isString := item.(string)
				if !isString {
					ok = false
					break
				}
				values = append(values, text)
			}
			if ok {
				result[key] = values
			}
		default:
			result[key] = typed
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func normalizeScheduleSummary(summary string) string {
	summary = strings.TrimSpace(summary)
	runes := []rune(summary)
	if len(runes) <= maxProactiveScheduleSummaryRunes {
		return summary
	}
	return string(runes[:maxProactiveScheduleSummaryRunes])
}

func normalizeScheduleMeta(meta map[string]any) map[string]any {
	if len(meta) == 0 {
		return nil
	}

	result := make(map[string]any, len(meta))
	for key, value := range meta {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		switch typed := value.(type) {
		case string:
			result[key] = typed
		case bool:
			result[key] = typed
		case float64:
			result[key] = typed
		case float32:
			result[key] = typed
		case int:
			result[key] = typed
		case int64:
			result[key] = typed
		case int32:
			result[key] = typed
		case []string:
			result[key] = append([]string(nil), typed...)
		case []any:
			values := make([]string, 0, len(typed))
			ok := true
			for _, item := range typed {
				text, isString := item.(string)
				if !isString {
					ok = false
					break
				}
				values = append(values, text)
			}
			if ok {
				result[key] = values
			}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
