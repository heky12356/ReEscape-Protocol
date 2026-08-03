package skill

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
)

func ParseSkillMarkdown(path string, content []byte) (Package, error) {
	lines := bytes.SplitAfter(content, []byte("\n"))
	if len(lines) == 0 {
		return Package{}, fmt.Errorf("skill file is empty")
	}

	firstLine := strings.TrimSpace(strings.TrimPrefix(string(lines[0]), "\ufeff"))
	if firstLine != "---" {
		return Package{}, fmt.Errorf("skill file must start with YAML frontmatter")
	}

	closingIndex := -1
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(string(lines[i]))
		if line == "---" || line == "..." {
			closingIndex = i
			break
		}
	}
	if closingIndex < 0 {
		return Package{}, fmt.Errorf("skill frontmatter is not closed")
	}

	frontmatterBytes := bytes.Join(lines[1:closingIndex], nil)
	frontmatter := make(map[string]any)
	if len(bytes.TrimSpace(frontmatterBytes)) > 0 {
		if err := yaml.Unmarshal(frontmatterBytes, &frontmatter); err != nil {
			return Package{}, fmt.Errorf("parse skill frontmatter: %w", err)
		}
	}

	name, err := requiredString(frontmatter, "name")
	if err != nil {
		return Package{}, err
	}
	description, err := requiredString(frontmatter, "description")
	if err != nil {
		return Package{}, err
	}

	body := bytes.Join(lines[closingIndex+1:], nil)
	if len(body) > 0 && body[0] == '\r' {
		body = bytes.TrimPrefix(body, []byte("\r"))
	}
	if len(body) > 0 && body[0] == '\n' {
		body = body[1:]
	}

	enabled := true
	if value, ok := frontmatter["enabled"]; ok {
		switch typed := value.(type) {
		case bool:
			enabled = typed
		case string:
			enabled = !strings.EqualFold(strings.TrimSpace(typed), "false")
		}
	}

	return Package{
		Name:        name,
		Description: description,
		RootDir:     path,
		Body:        string(body),
		Frontmatter: frontmatter,
		Enabled:     enabled,
	}, nil
}

func requiredString(frontmatter map[string]any, key string) (string, error) {
	value, ok := frontmatter[key]
	if !ok {
		return "", fmt.Errorf("skill frontmatter field %q is required", key)
	}
	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("skill frontmatter field %q must be a non-empty string", key)
	}
	return strings.TrimSpace(text), nil
}
