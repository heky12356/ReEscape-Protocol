package assets

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"project-yume/internal/config"
)

const defaultRelevantAssetLimit = 4

type ImageAsset struct {
	ID          string   `json:"id"`
	File        string   `json:"file"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Enabled     bool     `json:"enabled"`
}

type imageAssetCatalog struct {
	Assets []ImageAsset `json:"assets"`
}

func LookupImageAsset(assetID string) (ImageAsset, error) {
	catalog, err := loadImageAssetCatalog()
	if err != nil {
		return ImageAsset{}, err
	}

	target := strings.TrimSpace(assetID)
	for _, asset := range catalog.Assets {
		if !asset.Enabled {
			continue
		}
		if strings.EqualFold(asset.ID, target) {
			return normalizeImageAsset(asset), nil
		}
	}
	return ImageAsset{}, fmt.Errorf("image asset not found: %s", assetID)
}

func ListImageAssets() ([]ImageAsset, error) {
	catalog, err := loadImageAssetCatalog()
	if err != nil {
		return nil, err
	}
	result := make([]ImageAsset, 0, len(catalog.Assets))
	for _, asset := range catalog.Assets {
		result = append(result, normalizeImageAsset(asset))
	}
	return result, nil
}

func ListRelevantImageAssets(query string, limit int) ([]ImageAsset, error) {
	catalog, err := loadImageAssetCatalog()
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = defaultRelevantAssetLimit
	}

	normalizedQuery := normalizeAssetLookupText(query)
	requestingImage := looksLikeImageRequest(normalizedQuery)

	type scoredAsset struct {
		asset ImageAsset
		score int
	}

	scored := make([]scoredAsset, 0, len(catalog.Assets))
	for _, asset := range catalog.Assets {
		if !asset.Enabled {
			continue
		}

		score := scoreImageAsset(asset, normalizedQuery)
		if score == 0 && !requestingImage {
			continue
		}
		scored = append(scored, scoredAsset{asset: asset, score: score})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].asset.ID < scored[j].asset.ID
		}
		return scored[i].score > scored[j].score
	})

	result := make([]ImageAsset, 0, minInt(limit, len(scored)))
	for _, item := range scored {
		result = append(result, item.asset)
		if len(result) >= limit {
			break
		}
	}
	return result, nil
}

func ResolveImageAssetCQFile(asset ImageAsset) (string, error) {
	file := strings.TrimSpace(asset.File)
	if file == "" {
		return "", fmt.Errorf("image asset file is empty: %s", asset.ID)
	}

	if strings.HasPrefix(file, "http://") || strings.HasPrefix(file, "https://") || strings.HasPrefix(file, "base64://") {
		return file, nil
	}

	if strings.HasPrefix(file, "file://") {
		return file, nil
	}

	resolved := file
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(currentImageAssetDir(), resolved)
	}
	resolved = filepath.Clean(resolved)

	if _, err := os.Stat(resolved); err != nil {
		return "", fmt.Errorf("image asset file not found: %s", resolved)
	}

	u := &url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(resolved),
	}
	return u.String(), nil
}

func loadImageAssetCatalog() (imageAssetCatalog, error) {
	path := strings.TrimSpace(config.GetConfig().ImageAssetIndexFile)
	if path == "" {
		path = "./assets/images/index.json"
	}

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return imageAssetCatalog{}, err
	}

	var catalog imageAssetCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return imageAssetCatalog{}, fmt.Errorf("parse image asset index failed: %w", err)
	}

	for i := range catalog.Assets {
		catalog.Assets[i] = normalizeImageAsset(catalog.Assets[i])
	}
	return catalog, nil
}

func scoreImageAsset(asset ImageAsset, normalizedQuery string) int {
	if normalizedQuery == "" {
		return 0
	}

	score := 0
	candidates := []string{asset.ID, asset.Title, asset.Description}
	candidates = append(candidates, asset.Tags...)
	for _, candidate := range candidates {
		value := normalizeAssetLookupText(candidate)
		if value == "" {
			continue
		}
		if strings.Contains(normalizedQuery, value) || strings.Contains(value, normalizedQuery) {
			score += 3
			continue
		}
		for _, token := range strings.Fields(normalizedQuery) {
			if token != "" && strings.Contains(value, token) {
				score++
			}
		}
	}
	return score
}

func normalizeImageAsset(asset ImageAsset) ImageAsset {
	asset.ID = strings.TrimSpace(asset.ID)
	asset.File = strings.TrimSpace(asset.File)
	asset.Title = strings.TrimSpace(asset.Title)
	asset.Description = strings.TrimSpace(asset.Description)
	if asset.Tags == nil {
		asset.Tags = []string{}
	}
	return asset
}

func currentImageAssetDir() string {
	dir := strings.TrimSpace(config.GetConfig().ImageAssetDir)
	if dir == "" {
		return "./assets/images"
	}
	return filepath.Clean(dir)
}

func looksLikeImageRequest(query string) bool {
	keywords := []string{"图片", "照片", "表情包", "来张", "来一张", "发张", "发一张", "看看图", "配图", "meme", "image", "photo"}
	for _, keyword := range keywords {
		if strings.Contains(query, keyword) {
			return true
		}
	}
	return false
}

func normalizeAssetLookupText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\t", " ")
	return strings.Join(strings.Fields(value), " ")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
