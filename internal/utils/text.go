package utils

import (
	"regexp"
	"strings"
)

// CleanThinkTag 清除 <think> 标签及其内容
func CleanThinkTag(content string) string {
	// 移除 <think>...</think> 标签及其内容（支持跨行）
	re := regexp.MustCompile(`(?s)<think>.*?</think>`)
	cleaned := re.ReplaceAllString(content, "")

	// 移除可能单独存在的标签（以防万一模型输出不完整）
	cleaned = strings.ReplaceAll(cleaned, "<think>", "")
	cleaned = strings.ReplaceAll(cleaned, "</think>", "")

	// 清理前后空白
	return strings.TrimSpace(cleaned)
}
