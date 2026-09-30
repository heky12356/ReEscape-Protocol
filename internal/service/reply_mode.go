package service

type ReplyMode string

const (
	ReplyModeNoReply   ReplyMode = "no_reply"
	ReplyModeLightAck  ReplyMode = "light_ack"
	ReplyModeFullReply ReplyMode = "full_reply"
)
