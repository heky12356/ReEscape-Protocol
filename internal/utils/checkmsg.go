package utils

import (
	"html"
	"strings"
)

// 判断是否为cq码
func IsCQCode(msg string) bool {
	return strings.HasPrefix(msg, "[CQ:")
}

// 提取图片url
func ExtractImageURL(msg string) string {
	if !IsCQCode(msg) {
		return ""
	}
	// 提取图片url
	parts := strings.Split(msg, ",")
	if len(parts) < 3 {
		return ""
	}
	url := html.UnescapeString(parts[2])
	return url
}
