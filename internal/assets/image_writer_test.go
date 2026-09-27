package assets

import (
	"context"
	"strings"
	"testing"
)

func TestValidateImageAssetID(t *testing.T) {
	for _, test := range []struct {
		name  string
		value string
		valid bool
	}{
		{name: "valid", value: "good-night_cat.01", valid: true},
		{name: "empty", value: "", valid: false},
		{name: "path traversal", value: "../cat", valid: false},
		{name: "space", value: "cat image", valid: false},
		{name: "unicode", value: "猫", valid: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateImageAssetID(test.value)
			if test.valid && err != nil {
				t.Fatalf("expected valid id, got %v", err)
			}
			if !test.valid && err == nil {
				t.Fatal("expected invalid id")
			}
		})
	}
}

func TestSaveImageAssetRejectsMissingSource(t *testing.T) {
	_, err := SaveImageAsset(context.Background(), SaveImageAssetRequest{ID: "cat"})
	if err == nil || !strings.Contains(err.Error(), "source url is required") {
		t.Fatalf("expected missing source error, got %v", err)
	}
}

func TestDownloadImageRejectsNonHTTPURL(t *testing.T) {
	_, _, err := downloadImage(context.Background(), "file:///tmp/cat.png")
	if err == nil || !strings.Contains(err.Error(), "url scheme must be http or https") {
		t.Fatalf("expected scheme validation error, got %v", err)
	}
}

func TestImageExtension(t *testing.T) {
	for contentType, expected := range map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"image/gif":  ".gif",
		"image/webp": ".webp",
	} {
		extension, err := imageExtension(contentType)
		if err != nil || extension != expected {
			t.Fatalf("imageExtension(%q) = %q, %v; want %q", contentType, extension, err, expected)
		}
	}
}
