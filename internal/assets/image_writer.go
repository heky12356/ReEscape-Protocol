package assets

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"project-yume/internal/config"
	"project-yume/internal/webaccess"
)

const maxSavedImageBytes int64 = 10 << 20

const qqMultimediaImageHost = "multimedia.nt.qq.com.cn"

var imageAssetWriteMu sync.Mutex

type SaveImageAssetRequest struct {
	SourceURL   string
	ID          string
	Title       string
	Description string
	Tags        []string
	Overwrite   bool
}

type SaveImageAssetResult struct {
	Asset       ImageAsset `json:"asset"`
	Bytes       int64      `json:"bytes"`
	ContentType string     `json:"content_type"`
}

func SaveImageAsset(ctx context.Context, request SaveImageAssetRequest) (SaveImageAssetResult, error) {
	if err := validateImageAssetID(request.ID); err != nil {
		return SaveImageAssetResult{}, err
	}
	if strings.TrimSpace(request.SourceURL) == "" {
		return SaveImageAssetResult{}, fmt.Errorf("source url is required")
	}

	imageAssetWriteMu.Lock()
	defer imageAssetWriteMu.Unlock()

	catalog, err := loadOrCreateImageAssetCatalog()
	if err != nil {
		return SaveImageAssetResult{}, err
	}

	for _, existing := range catalog.Assets {
		if strings.EqualFold(existing.ID, request.ID) && !request.Overwrite {
			return SaveImageAssetResult{}, fmt.Errorf("image asset already exists: %s", request.ID)
		}
	}

	data, contentType, err := downloadImage(ctx, request.SourceURL)
	if err != nil {
		return SaveImageAssetResult{}, err
	}

	extension, err := imageExtension(contentType)
	if err != nil {
		return SaveImageAssetResult{}, err
	}

	dir := currentImageAssetDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return SaveImageAssetResult{}, fmt.Errorf("create image asset directory: %w", err)
	}

	fileName := request.ID + extension
	targetPath := filepath.Join(dir, fileName)
	if !request.Overwrite {
		if _, err := os.Stat(targetPath); err == nil {
			return SaveImageAssetResult{}, fmt.Errorf("image asset file already exists: %s", fileName)
		} else if !os.IsNotExist(err) {
			return SaveImageAssetResult{}, fmt.Errorf("check image asset file: %w", err)
		}
	}
	if err := writeFileAtomically(targetPath, data, request.Overwrite); err != nil {
		return SaveImageAssetResult{}, fmt.Errorf("write image asset file: %w", err)
	}

	asset := normalizeImageAsset(ImageAsset{
		ID:          request.ID,
		File:        fileName,
		Title:       request.Title,
		Description: request.Description,
		Tags:        request.Tags,
		Enabled:     true,
	})
	if asset.Title == "" {
		asset.Title = asset.ID
	}

	replaced := false
	for i := range catalog.Assets {
		if strings.EqualFold(catalog.Assets[i].ID, asset.ID) {
			catalog.Assets[i] = asset
			replaced = true
			break
		}
	}
	if !replaced {
		catalog.Assets = append(catalog.Assets, asset)
	}

	if err := saveImageAssetCatalog(catalog); err != nil {
		if !replaced {
			_ = os.Remove(targetPath)
		}
		return SaveImageAssetResult{}, err
	}

	return SaveImageAssetResult{
		Asset:       asset,
		Bytes:       int64(len(data)),
		ContentType: contentType,
	}, nil
}

func loadOrCreateImageAssetCatalog() (imageAssetCatalog, error) {
	catalog, err := loadImageAssetCatalog()
	if err == nil {
		return catalog, nil
	}
	if os.IsNotExist(err) {
		return imageAssetCatalog{Assets: []ImageAsset{}}, nil
	}
	return imageAssetCatalog{}, err
}

func saveImageAssetCatalog(catalog imageAssetCatalog) error {
	path := strings.TrimSpace(config.GetConfig().ImageAssetIndexFile)
	if path == "" {
		path = "./assets/images/index.json"
	}
	path = filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create image asset index directory: %w", err)
	}

	data, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal image asset index: %w", err)
	}
	data = append(data, '\n')
	if err := writeFileAtomically(path, data, true); err != nil {
		return fmt.Errorf("write image asset index: %w", err)
	}
	return nil
}

func downloadImage(ctx context.Context, rawURL string) ([]byte, string, error) {
	parsed, err := webaccess.ValidateOutboundURLWithAllowedHosts(ctx, rawURL, qqMultimediaImageHost)
	if err != nil {
		return nil, "", err
	}

	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			_, err := webaccess.ValidateOutboundURLWithAllowedHosts(req.Context(), req.URL.String(), qqMultimediaImageHost)
			return err
		},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, "", err
	}
	request.Header.Set("Accept", "image/jpeg,image/png,image/gif,image/webp;q=0.9,*/*;q=0.1")
	request.Header.Set("User-Agent", "ReEscapeProtocolBot/1.0")

	response, err := client.Do(request)
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, "", fmt.Errorf("image download returned status %d", response.StatusCode)
	}
	if response.ContentLength > maxSavedImageBytes {
		return nil, "", fmt.Errorf("image exceeds %d byte limit", maxSavedImageBytes)
	}

	data, err := io.ReadAll(io.LimitReader(response.Body, maxSavedImageBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("read image response: %w", err)
	}
	if int64(len(data)) > maxSavedImageBytes {
		return nil, "", fmt.Errorf("image exceeds %d byte limit", maxSavedImageBytes)
	}

	contentType := normalizeContentType(response.Header.Get("Content-Type"))
	detectedType := normalizeContentType(http.DetectContentType(data))
	if isAllowedImageType(detectedType) {
		contentType = detectedType
	}
	if !isAllowedImageType(contentType) {
		return nil, "", fmt.Errorf("unsupported image content type: %s", contentType)
	}
	return data, contentType, nil
}

func writeFileAtomically(path string, data []byte, overwrite bool) error {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, ".image-asset-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}

	if overwrite {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return os.Rename(tempPath, path)
}

func validateImageAssetID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("asset_id is required")
	}
	if len([]rune(id)) > 64 {
		return fmt.Errorf("asset_id must be at most 64 characters")
	}
	for _, char := range id {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '-' || char == '_' || char == '.' {
			continue
		}
		return fmt.Errorf("asset_id may contain only letters, numbers, dot, dash and underscore")
	}
	return nil
}

func normalizeContentType(value string) string {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(value))
	if err != nil {
		return strings.ToLower(strings.TrimSpace(value))
	}
	return strings.ToLower(strings.TrimSpace(mediaType))
}

func isAllowedImageType(value string) bool {
	switch value {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return true
	default:
		return false
	}
}

func imageExtension(contentType string) (string, error) {
	switch contentType {
	case "image/jpeg":
		return ".jpg", nil
	case "image/png":
		return ".png", nil
	case "image/gif":
		return ".gif", nil
	case "image/webp":
		return ".webp", nil
	default:
		return "", fmt.Errorf("unsupported image content type: %s", contentType)
	}
}
