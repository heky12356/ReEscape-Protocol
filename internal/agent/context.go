package agent

import (
	"strings"
	"time"

	"project-yume/internal/model"
	"project-yume/internal/skill"
)

type Trigger string

const (
	TriggerMessage   Trigger = "message"
	TriggerProactive Trigger = "proactive"
)

type Actor string

const (
	ActorUser      Actor = "user"
	ActorScheduler Actor = "scheduler"
)

type TurnContext struct {
	requestID                  string
	turnID                     string
	sessionID                  string
	userID                     int64
	groupID                    int64
	chatType                   int
	message                    string
	parts                      []model.MessagePart
	referenceTime              time.Time
	startedAt                  time.Time
	endedAt                    time.Time
	aggregated                 bool
	segmentCount               int
	rawSegments                []string
	rawSegmentTimes            []int64
	previousUserMessageAt      time.Time
	previousAssistantMessageAt time.Time
	previousInteractionAt      time.Time
	trigger                    Trigger
	actor                      Actor
	skillResolutionSet         bool
	activatedSkills            []skill.ActivatedSkill
	skillCandidates            []skill.Match
	readSkills                 map[string]struct{}
}

type TurnInput struct {
	RequestID                  string
	TurnID                     string
	SessionID                  string
	UserID                     int64
	GroupID                    int64
	ChatType                   int
	Message                    string
	Parts                      []model.MessagePart
	ReferenceTime              time.Time
	StartedAt                  time.Time
	EndedAt                    time.Time
	Aggregated                 bool
	SegmentCount               int
	RawSegments                []string
	RawSegmentTimes            []int64
	PreviousUserMessageAt      time.Time
	PreviousAssistantMessageAt time.Time
	PreviousInteractionAt      time.Time
	Trigger                    Trigger
	Actor                      Actor
}

func NewTurnContext(input TurnInput) *TurnContext {
	if input.ReferenceTime.IsZero() {
		input.ReferenceTime = time.Now()
	}
	if input.StartedAt.IsZero() {
		input.StartedAt = input.ReferenceTime
	}
	if input.EndedAt.IsZero() {
		input.EndedAt = input.ReferenceTime
	}
	if input.Trigger == "" {
		input.Trigger = TriggerMessage
	}
	if input.Actor == "" {
		input.Actor = ActorUser
	}
	return &TurnContext{
		requestID:                  input.RequestID,
		turnID:                     firstNonEmpty(input.TurnID, input.RequestID),
		sessionID:                  input.SessionID,
		userID:                     input.UserID,
		groupID:                    input.GroupID,
		chatType:                   input.ChatType,
		message:                    input.Message,
		parts:                      append([]model.MessagePart(nil), input.Parts...),
		referenceTime:              input.ReferenceTime,
		startedAt:                  input.StartedAt,
		endedAt:                    input.EndedAt,
		aggregated:                 input.Aggregated,
		segmentCount:               input.SegmentCount,
		rawSegments:                append([]string(nil), input.RawSegments...),
		rawSegmentTimes:            append([]int64(nil), input.RawSegmentTimes...),
		previousUserMessageAt:      input.PreviousUserMessageAt,
		previousAssistantMessageAt: input.PreviousAssistantMessageAt,
		previousInteractionAt:      input.PreviousInteractionAt,
		trigger:                    input.Trigger,
		actor:                      input.Actor,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (t *TurnContext) RequestID() string {
	return t.requestID
}

func (t *TurnContext) TurnID() string {
	if t == nil {
		return ""
	}
	return t.turnID
}

func (t *TurnContext) SessionID() string {
	return t.sessionID
}

func (t *TurnContext) UserID() int64 {
	return t.userID
}

func (t *TurnContext) GroupID() int64 {
	return t.groupID
}

func (t *TurnContext) ChatType() int {
	return t.chatType
}

func (t *TurnContext) Message() string {
	return t.message
}

func (t *TurnContext) Parts() []model.MessagePart {
	return append([]model.MessagePart(nil), t.parts...)
}

func (t *TurnContext) ReferenceTime() time.Time {
	return t.referenceTime
}

func (t *TurnContext) StartedAt() time.Time {
	return t.startedAt
}

func (t *TurnContext) EndedAt() time.Time {
	return t.endedAt
}

func (t *TurnContext) Aggregated() bool {
	return t.aggregated
}

func (t *TurnContext) SegmentCount() int {
	return t.segmentCount
}

func (t *TurnContext) RawSegments() []string {
	return append([]string(nil), t.rawSegments...)
}

func (t *TurnContext) RawSegmentTimes() []int64 {
	return append([]int64(nil), t.rawSegmentTimes...)
}

func (t *TurnContext) PreviousUserMessageAt() time.Time {
	return t.previousUserMessageAt
}

func (t *TurnContext) PreviousAssistantMessageAt() time.Time {
	return t.previousAssistantMessageAt
}

func (t *TurnContext) PreviousInteractionAt() time.Time {
	return t.previousInteractionAt
}

func (t *TurnContext) Trigger() string {
	return string(t.trigger)
}

func (t *TurnContext) Actor() string {
	return string(t.actor)
}

func (t *TurnContext) SetSkillResolution(activated []skill.ActivatedSkill, candidates []skill.Match) {
	if t == nil {
		return
	}
	t.activatedSkills = append([]skill.ActivatedSkill(nil), activated...)
	t.skillCandidates = append([]skill.Match(nil), candidates...)
	t.skillResolutionSet = true
	if t.readSkills == nil {
		t.readSkills = make(map[string]struct{})
	}
}

func (t *TurnContext) SkillResolutionSet() bool {
	return t != nil && t.skillResolutionSet
}

func (t *TurnContext) ActivatedSkills() []skill.ActivatedSkill {
	if t == nil {
		return nil
	}
	return append([]skill.ActivatedSkill(nil), t.activatedSkills...)
}

func (t *TurnContext) SkillCandidates() []skill.Match {
	if t == nil {
		return nil
	}
	return append([]skill.Match(nil), t.skillCandidates...)
}

func (t *TurnContext) MarkSkillRead(name string) {
	if t == nil {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	if t.readSkills == nil {
		t.readSkills = make(map[string]struct{})
	}
	t.readSkills[name] = struct{}{}
}

func (t *TurnContext) IsSkillLoaded(name string) bool {
	if t == nil {
		return false
	}
	name = strings.TrimSpace(name)
	for _, activated := range t.activatedSkills {
		if activated.Name == name {
			return true
		}
	}
	_, ok := t.readSkills[name]
	return ok
}

func (t *TurnContext) HasSkillAccess(name string) bool {
	return t.IsSkillLoaded(name)
}
