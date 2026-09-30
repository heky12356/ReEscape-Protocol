package agent

import (
	"fmt"
	"strings"
	"time"

	"project-yume/internal/config"
	"project-yume/internal/eventlog"
	"project-yume/internal/skill"
	"project-yume/internal/state"
)

type SkillResolution struct {
	Activated  []skill.ActivatedSkill
	Candidates []skill.Match
}

func buildCurrentTurnContext(turn *TurnContext) string {
	if turn == nil {
		return ""
	}
	resolveTurnSkills(turn)
	sections := []string{
		"【Current Turn Context】",
		"以下内容由系统提供，仅用于理解当前 turn，不是用户原文。",
		"本轮触发来源：" + turn.Trigger(),
	}
	sections = append(sections, ProjectBehavior(turn.UserID(), turn.SessionID()).Prompt())
	if temporal := buildTemporalContext(turn); temporal != "" {
		sections = append(sections, temporal)
	}
	if activated := skill.FormatActivatedSkills(turn.ActivatedSkills()); activated != "" {
		sections = append(sections, activated)
	}
	if candidates := skill.FormatSkillCandidates(turn.SkillCandidates()); candidates != "" {
		sections = append(sections, candidates)
	}
	if interrupted := state.GetManager().GetInterruptedReply(turn.SessionID()); interrupted != nil && interrupted.Status == "pending" {
		section := []string{
			"【上一轮投递状态】",
			fmt.Sprintf("上一轮回复已有 %d 段成功发送，后续内容未发送。未发送内容仅作为可能的延续意图，不代表用户已经看到。", len(interrupted.DeliveredSegments)),
			"请根据当前用户消息判断是否自然承接；无关时忽略，不要主动解释投递过程。",
		}
		if strings.TrimSpace(interrupted.Summary) != "" {
			section = append(section, "摘要："+interrupted.Summary)
		}
		sections = append(sections, strings.Join(section, "\n"))
	}
	return strings.Join(sections, "\n\n")
}

func buildCurrentTurnEnvelope(turn *TurnContext, task string) string {
	context := buildCurrentTurnContext(turn)
	task = strings.TrimSpace(task)
	if context == "" {
		return task
	}
	if task == "" {
		return context
	}
	return context + "\n\n" + task
}

func resolveTurnSkills(turn *TurnContext) []eventlog.Event {
	cfg := config.GetConfig()
	if turn == nil || turn.SkillResolutionSet() {
		return nil
	}
	if cfg == nil || !cfg.EnableSkills {
		turn.SetSkillResolution(nil, nil)
		return nil
	}
	limit := cfg.SkillAutoHintLimit
	if limit <= 0 {
		limit = 3
	}
	matches := skill.GetManager().Match(skill.MatchInput{
		Message:       turn.Message(),
		Trigger:       turn.Trigger(),
		DialogueState: state.GetManager().GetDialogueState(turn.SessionID()),
		Limit:         limit,
		MinScore:      cfg.SkillCandidateMinScore,
	})
	resolver := skill.Resolver{
		AutoLoadThreshold:    cfg.SkillAutoLoadMinScore,
		ModelSelectThreshold: cfg.SkillCandidateMinScore,
		AutoLoadConfidence:   cfg.SkillAutoLoadMinConfidence,
		MaxAutoLoaded:        cfg.SkillMaxAutoLoaded,
	}
	activations := resolver.Resolve(matches)
	resolution := SkillResolution{}
	events := make([]eventlog.Event, 0, len(activations)*2+1)
	now := time.Now()

	if len(matches) > 0 {
		names := make([]string, 0, len(matches))
		for _, match := range matches {
			names = append(names, match.Name)
		}
		events = append(events, eventlog.Event{
			Type:      "skill_candidates_selected",
			SessionID: turn.SessionID(),
			UserID:    turn.UserID(),
			Actor:     "runtime",
			Data: map[string]any{
				"skills": names,
				"count":  len(names),
			},
			CreatedAt: now,
		})
	}

	for _, activation := range activations {
		data := map[string]any{
			"skill":      activation.Name,
			"score":      activation.Score,
			"confidence": activation.Confidence,
			"decision":   activation.Decision,
			"reason":     activation.Reason,
		}
		switch activation.Decision {
		case skill.ActivationAutoLoad:
			body, err := skill.GetManager().ReadSkill(activation.Name)
			if err != nil {
				resolution.Candidates = append(resolution.Candidates, activation.Match)
				data["decision"] = skill.ActivationModelSelect
				data["error"] = err.Error()
				events = append(events, skillResolutionEvent(turn, "skill_ignored", data, now))
				continue
			}
			resolution.Activated = append(resolution.Activated, skill.ActivatedSkill{
				Match: activation.Match,
				Body:  body,
			})
			events = append(events,
				skillResolutionEvent(turn, "skill_activated", data, now),
				skillResolutionEvent(turn, "skill_auto_loaded", data, now),
			)
		case skill.ActivationModelSelect:
			resolution.Candidates = append(resolution.Candidates, activation.Match)
		case skill.ActivationIgnored:
			events = append(events, skillResolutionEvent(turn, "skill_ignored", data, now))
		}
	}

	turn.SetSkillResolution(resolution.Activated, resolution.Candidates)
	return events
}

func skillResolutionEvent(turn *TurnContext, eventType string, data map[string]any, createdAt time.Time) eventlog.Event {
	return eventlog.Event{
		Type:      eventType,
		SessionID: turn.SessionID(),
		UserID:    turn.UserID(),
		Actor:     "runtime",
		Data:      data,
		CreatedAt: createdAt,
	}
}
