package service

import (
	"regexp"
	"strings"
)

var imageDirectivePattern = regexp.MustCompile(`\[\[image:([a-zA-Z0-9._-]+)\]\]`)

type replyChunk struct {
	Text         string
	ImageAssetID string
}

// OutboundMessage is one stable unit sent through the OneBot API. Text
// segments and image directives share the same delivery lifecycle.
type OutboundMessage struct {
	Index      int
	Kind       string // text or image
	Content    string
	AssetID    string
	OneBotText string
}

func ParseReplyChunks(reply string) []replyChunk {
	matches := imageDirectivePattern.FindAllStringSubmatchIndex(reply, -1)
	if len(matches) == 0 {
		return []replyChunk{{Text: reply}}
	}

	chunks := make([]replyChunk, 0, len(matches)*2+1)
	last := 0
	for _, match := range matches {
		if match[0] > last {
			chunks = append(chunks, replyChunk{Text: reply[last:match[0]]})
		}
		chunks = append(chunks, replyChunk{ImageAssetID: reply[match[2]:match[3]]})
		last = match[1]
	}
	if last < len(reply) {
		chunks = append(chunks, replyChunk{Text: reply[last:]})
	}
	return chunks
}

// ParseOutboundMessages expands reply directives and segment delimiters into
// the units used by the delivery service. Empty text fragments are omitted.
func ParseOutboundMessages(reply string) []OutboundMessage {
	chunks := ParseReplyChunks(reply)
	items := make([]OutboundMessage, 0, len(chunks))
	for _, chunk := range chunks {
		if text := strings.TrimSpace(chunk.Text); text != "" {
			for _, segment := range splitReplySegments(text) {
				segment = strings.TrimSpace(segment)
				if segment == "" {
					continue
				}
				items = append(items, OutboundMessage{
					Index:      len(items),
					Kind:       "text",
					Content:    segment,
					OneBotText: segment,
				})
			}
		}
		if assetID := strings.TrimSpace(chunk.ImageAssetID); assetID != "" {
			items = append(items, OutboundMessage{
				Index:   len(items),
				Kind:    "image",
				AssetID: assetID,
			})
		}
	}
	return items
}

func StripReplyDirectives(reply string) string {
	cleaned := imageDirectivePattern.ReplaceAllString(reply, "")
	cleaned = strings.ReplaceAll(cleaned, "  ", " ")
	cleaned = strings.ReplaceAll(cleaned, "\n\n\n", "\n\n")
	return strings.TrimSpace(cleaned)
}
