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

func StripReplyDirectives(reply string) string {
	cleaned := imageDirectivePattern.ReplaceAllString(reply, "")
	cleaned = strings.ReplaceAll(cleaned, "  ", " ")
	cleaned = strings.ReplaceAll(cleaned, "\n\n\n", "\n\n")
	return strings.TrimSpace(cleaned)
}
