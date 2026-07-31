package model

// Msg is the normalized message exchanged between receiver, aggregator, and processor goroutines.
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
	Type        int // 0: group message, 1: private message
}

type MessagePart struct {
	Type    string `json:"type"`
	Text    string `json:"text,omitempty"`
	URL     string `json:"url,omitempty"`
	File    string `json:"file,omitempty"`
	OCRText string `json:"ocr_text,omitempty"`
}
