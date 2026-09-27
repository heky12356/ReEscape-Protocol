package app

import (
	"encoding/json"
	"testing"

	"project-yume/internal/model"
)

func TestBuildIncomingMessagePartsAcceptsMixedImageDataTypes(t *testing.T) {
	var response model.Response

	const payload = `{
		"message": [{
			"type": "image",
			"data": {
				"file": "E1F85A362A512787A2D300063B043221.jpg",
				"sub_type": 1,
				"url": "https://example.com/image.jpg",
				"file_size": 11727
			}
		}]
	}`

	if err := json.Unmarshal([]byte(payload), &response); err != nil {
		t.Fatalf("unmarshal mixed image data: %v", err)
	}

	parts := buildIncomingMessageParts(response)
	if len(parts) != 1 {
		t.Fatalf("expected one image part, got %d", len(parts))
	}
	if parts[0].Type != "image" {
		t.Fatalf("expected image part, got %q", parts[0].Type)
	}
	if parts[0].File != "E1F85A362A512787A2D300063B043221.jpg" {
		t.Errorf("unexpected image file: %q", parts[0].File)
	}
	if parts[0].URL != "https://example.com/image.jpg" {
		t.Errorf("unexpected image URL: %q", parts[0].URL)
	}
}
