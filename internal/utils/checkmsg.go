package utils

import (
	"html"
	"strings"
)

// 判断是否为cq码
func IsCQCode(msg string) bool {
	return strings.HasPrefix(msg, "[CQ:")
}

func IsCQImage(msg string) bool {
	return strings.HasPrefix(msg, "[CQ:image")
}

func ParseCQParams(msg string) map[string]string {
	if !IsCQCode(msg) {
		return nil
	}

	trimmed := strings.TrimSuffix(strings.TrimPrefix(msg, "["), "]")
	parts := strings.Split(trimmed, ",")
	if len(parts) == 0 {
		return nil
	}

	result := make(map[string]string, len(parts))
	result["type"] = strings.TrimPrefix(parts[0], "CQ:")
	for _, part := range parts[1:] {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		result[strings.TrimSpace(key)] = html.UnescapeString(strings.TrimSpace(value))
	}
	return result
}

// 提取图片url
func ExtractImageURL(msg string) string {
	params := ParseCQParams(msg)
	if params == nil {
		return ""
	}
	return strings.TrimSpace(params["url"])
}

func ExtractImageFile(msg string) string {
	params := ParseCQParams(msg)
	if params == nil {
		return ""
	}
	return strings.TrimSpace(params["file"])
}
