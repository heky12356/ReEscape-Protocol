package model

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
