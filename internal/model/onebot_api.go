package model

import "encoding/json"

type Message struct {
	Action string      `json:"action"`
	Params interface{} `json:"params"`
	Echo   string      `json:"echo"`
}

type UserMessageParams struct {
	User_id int64  `json:"user_id"`
	Message string `json:"message"`
}

type APIResponse struct {
	Status  string          `json:"status"`
	RetCode int             `json:"retcode"`
	Data    json.RawMessage `json:"data"`
	Echo    string          `json:"echo"`
}
