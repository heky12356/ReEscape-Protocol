package model

import "encoding/json"

// goroutine之间消息
type Msg struct {
	Message     string        `json:"message"`
	Parts       []MessagePart `json:"parts,omitempty"`
	User_id     int64         `json:"user_id"`
	Group_id    int64         `json:"group_id"`
	MessageID   int64         `json:"message_id"`
	MessageIDs  []int64       `json:"message_ids,omitempty"`
	RawSegments []string      `json:"raw_segments,omitempty"`
	Aggregated  bool          `json:"aggregated,omitempty"`
	StartTime   int64         `json:"start_time,omitempty"`
	EndTime     int64         `json:"end_time,omitempty"`
	Time        int64
	Type        int // 0:群消息 1:私聊消息
}

type MessagePart struct {
	Type    string `json:"type"`
	Text    string `json:"text,omitempty"`
	URL     string `json:"url,omitempty"`
	File    string `json:"file,omitempty"`
	OCRText string `json:"ocr_text,omitempty"`
}

type Message struct {
	Action string      `json:"action"`
	Params interface{} `json:"params"`
	Echo   string      `json:"echo"`
}

type MessageParams struct {
	Group_id int64  `json:"group_id"`
	Message  string `json:"message"`
}

type UserMessageParams struct {
	User_id int64  `json:"user_id"`
	Message string `json:"message"`
}

type eMessageParams struct {
	Message_id int32 `json:"message_id"`
}

type gMessageParams struct {
	Group_id int64  `json:"group_id"`
	Content  string `json:"content"`
}

type EssenceMessage struct {
	Action string         `json:"action"`
	Params eMessageParams `json:"params"`
}

type Group_notice struct {
	Action string         `json:"action"`
	Params gMessageParams `json:"params"`
	Echo   string         `json:"echo"`
}

type SetMsgEmoji struct {
	Message_id int32 `json:"message_id"`
	Emoji_id   int32 `json:"emoji_id"`
	Set        bool  `json:"set"`
}

// 上报消息

type Sender struct {
	User_id  int64  `json:"user_id"`
	Nickname string `json:"nickname"`
	Card     string `json:"card"`
	Role     string `json:"role"`
}

type ReMessage struct {
	Type string            `json:"type"`
	Data map[string]string `json:"data"`
}

type Response struct {
	Self_id        int64       `json:"self_id"`
	User_id        int64       `json:"user_id"`
	Time           int64       `json:"time"`
	Message_id     int64       `json:"message_id"`
	Message_seq    int64       `json:"message_seq"`
	Real_id        int64       `json:"real_id"`
	Real_seq       string      `json:"real_seq"`
	Message_type   string      `json:"message_type"`
	Sender         Sender      `json:"sender"`
	Raw_message    string      `json:"raw_message"`
	Font           int64       `json:"font"`
	Subtype        string      `json:"sub_type"`
	Message        []ReMessage `json:"message"`
	Message_format string      `json:"message_format"`
	Post_type      string      `json:"post_type"`
	Group_id       int64       `json:"group_id"`
}

type APIResponse struct {
	Status  string          `json:"status"`
	RetCode int             `json:"retcode"`
	Data    json.RawMessage `json:"data"`
	Echo    string          `json:"echo"`
}

type GetImageData struct {
	File string `json:"file"`
	URL  string `json:"url"`
}

type OCRImageData struct {
	Texts []OCRTextItem `json:"texts"`
	Text  string        `json:"text"`
}

type OCRTextItem struct {
	Text string `json:"text"`
}
