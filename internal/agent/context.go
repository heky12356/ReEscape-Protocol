package agent

import (
	"time"

	"project-yume/internal/model"
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
}

type TurnInput struct {
	RequestID                  string
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

func (t *TurnContext) RequestID() string {
	return t.requestID
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
