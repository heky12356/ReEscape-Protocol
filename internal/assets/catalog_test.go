package assets

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"project-yume/internal/config"
)

func TestResolveImageAssetCQFileUsesAbsolutePathForRelativeAssetDir(t *testing.T) {
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}

	tempDir, err := os.MkdirTemp(workingDir, ".image-asset-test-")
	if err != nil {
		t.Fatalf("create temporary directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	fileName := "hamster_glasses.webp"
	if err := os.WriteFile(filepath.Join(tempDir, fileName), []byte("image"), 0o600); err != nil {
		t.Fatalf("create temporary image: %v", err)
	}

	cfg := config.GetConfig()
	previousDir := cfg.ImageAssetDir
	cfg.ImageAssetDir, err = filepath.Rel(workingDir, tempDir)
	if err != nil {
		t.Fatalf("make relative asset directory: %v", err)
	}
	t.Cleanup(func() { cfg.ImageAssetDir = previousDir })

	fileValue, err := ResolveImageAssetCQFile(ImageAsset{ID: "hamster_glasses", File: fileName})
	if err != nil {
		t.Fatalf("resolve image asset file: %v", err)
	}
	parsed, err := url.Parse(fileValue)
	if err != nil {
		t.Fatalf("parse file URL: %v", err)
	}
	if parsed.Scheme != "file" || !strings.Contains(filepath.FromSlash(parsed.Path), fileName) {
		t.Fatalf("unexpected file URL: %s", fileValue)
	}
	if !filepath.IsAbs(filepath.FromSlash(strings.TrimPrefix(parsed.Path, "/"))) {
		t.Fatalf("expected absolute file path in URL: %s", fileValue)
	}
}
