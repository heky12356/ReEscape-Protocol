package service

import (
	"encoding/json"
	"strings"

	"project-yume/internal/config"
	"project-yume/internal/connect"
	"project-yume/internal/model"
	"project-yume/internal/utils"

	"github.com/gorilla/websocket"
)

func EnrichMessageParts(conn *websocket.Conn, parts []model.MessagePart) []model.MessagePart {
	if len(parts) == 0 {
		return nil
	}

	result := make([]model.MessagePart, 0, len(parts))
	for _, part := range parts {
		enriched := part
		if enriched.Type != "image" {
			result = append(result, enriched)
			continue
		}

		if strings.TrimSpace(enriched.URL) == "" && strings.TrimSpace(enriched.File) != "" {
			if imageURL, err := resolveOneBotImageURL(conn, enriched.File); err == nil {
				enriched.URL = imageURL
			} else {
				utils.Warn("resolve image url failed: %v", err)
			}
		}

		cfg := config.GetConfig()
		if cfg.EnableImageOCRFallback && !cfg.EnableVisionInput {
			if ocrText, err := ocrOneBotImage(conn, enriched); err == nil && strings.TrimSpace(ocrText) != "" {
				enriched.OCRText = ocrText
			} else if err != nil {
				utils.Warn("ocr image failed: %v", err)
			}
		}

		result = append(result, enriched)
	}
	return result
}

func resolveOneBotImageURL(conn *websocket.Conn, file string) (string, error) {
	resp, err := connect.CallAPI(conn, "get_image", map[string]string{
		"file": file,
	})
	if err != nil {
		return "", err
	}

	var data model.GetImageData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return "", err
	}
	return strings.TrimSpace(data.URL), nil
}

func ocrOneBotImage(conn *websocket.Conn, part model.MessagePart) (string, error) {
	image := strings.TrimSpace(part.URL)
	if image == "" {
		image = strings.TrimSpace(part.File)
	}
	if image == "" {
		return "", nil
	}

	resp, err := connect.CallAPI(conn, "ocr_image", map[string]string{
		"image": image,
	})
	if err != nil {
		return "", err
	}

	var data model.OCRImageData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return "", err
	}

	if strings.TrimSpace(data.Text) != "" {
		return strings.TrimSpace(data.Text), nil
	}

	lines := make([]string, 0, len(data.Texts))
	for _, item := range data.Texts {
		text := strings.TrimSpace(item.Text)
		if text != "" {
			lines = append(lines, text)
		}
	}
	return strings.Join(lines, "\n"), nil
}
