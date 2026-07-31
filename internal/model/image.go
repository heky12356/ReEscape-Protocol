package model

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
