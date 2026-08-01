package webaccess

import (
	"html"
	"regexp"
	"strings"
)

var (
	scriptStyleRE = regexp.MustCompile(`(?is)<(script|style|noscript)\b[^>]*>.*?</(script|style|noscript)>`)
	titleRE       = regexp.MustCompile(`(?is)<title\b[^>]*>(.*?)</title>`)
	tagRE         = regexp.MustCompile(`(?s)<[^>]+>`)
	spaceRE       = regexp.MustCompile(`\s+`)
)

func htmlToText(raw string) string {
	text := scriptStyleRE.ReplaceAllString(raw, " ")
	text = tagRE.ReplaceAllString(text, " ")
	text = html.UnescapeString(text)
	return normalizeWhitespace(text)
}

func extractHTMLTitle(raw string) string {
	matches := titleRE.FindStringSubmatch(raw)
	if len(matches) < 2 {
		return ""
	}
	return normalizeWhitespace(html.UnescapeString(tagRE.ReplaceAllString(matches[1], " ")))
}

func normalizeWhitespace(raw string) string {
	return strings.TrimSpace(spaceRE.ReplaceAllString(raw, " "))
}

func truncateRunes(raw string, limit int) (string, bool) {
	if limit <= 0 {
		return "", raw != ""
	}
	runes := []rune(raw)
	if len(runes) <= limit {
		return raw, false
	}
	return string(runes[:limit]), true
}
