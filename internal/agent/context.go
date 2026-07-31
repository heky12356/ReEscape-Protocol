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
	requestID     string
	sessionID     string
	userID        int64
	groupID       int64
	chatType      int
	message       string
	parts         []model.MessagePart
	referenceTime time.Time
	trigger       Trigger
	actor         Actor
}

type TurnInput struct {
	RequestID     string
	SessionID     string
	UserID        int64
	GroupID       int64
	ChatType      int
	Message       string
	Parts         []model.MessagePart
	ReferenceTime time.Time
	Trigger       Trigger
	Actor         Actor
}

func NewTurnContext(input TurnInput) *TurnContext {
	if input.ReferenceTime.IsZero() {
		input.ReferenceTime = time.Now()
	}
	if input.Trigger == "" {
		input.Trigger = TriggerMessage
	}
	if input.Actor == "" {
		input.Actor = ActorUser
	}
	return &TurnContext{
		requestID:     input.RequestID,
		sessionID:     input.SessionID,
		userID:        input.UserID,
		groupID:       input.GroupID,
		chatType:      input.ChatType,
		message:       input.Message,
		parts:         append([]model.MessagePart(nil), input.Parts...),
		referenceTime: input.ReferenceTime,
		trigger:       input.Trigger,
		actor:         input.Actor,
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

func (t *TurnContext) Trigger() string {
	return string(t.trigger)
}

func (t *TurnContext) Actor() string {
	return string(t.actor)
}
