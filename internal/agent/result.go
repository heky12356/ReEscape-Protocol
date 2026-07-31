package agent

import "project-yume/internal/eventlog"

type FinalReply struct {
	Content string `json:"content"`
}

type TurnResult struct {
	Handled    bool             `json:"handled"`
	ShouldSend bool             `json:"should_send"`
	FinalReply FinalReply       `json:"final_reply"`
	Trace      Trace            `json:"trace"`
	Events     []eventlog.Event `json:"events"`
}
