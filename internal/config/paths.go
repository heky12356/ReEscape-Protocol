package config

import (
	"os"
	"path/filepath"
)

func resolveProjectPath(rel string) string {
	candidates := []string{
		filepath.Clean(rel),
		filepath.Join("..", rel),
		filepath.Join("..", "..", rel),
		filepath.Join("..", "..", "..", rel),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return filepath.Clean(candidate)
		}
	}

	return filepath.Clean(rel)
}

func getCharacterConfigDir() string {
	return resolveProjectPath(filepath.Join("config", "character"))
}
