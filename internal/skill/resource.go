package skill

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const DefaultResourceMaxBytes = 65536

func ReadResource(pkg Package, relativePath string, maxBytes int) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = DefaultResourceMaxBytes
	}
	relativePath = filepath.ToSlash(strings.TrimSpace(relativePath))
	if relativePath == "" || filepath.IsAbs(relativePath) || filepath.VolumeName(relativePath) != "" {
		return nil, fmt.Errorf("resource path must be relative")
	}

	parts := strings.Split(relativePath, "/")
	for _, part := range parts {
		if part == ".." || part == "" {
			if part == ".." {
				return nil, fmt.Errorf("resource path traversal is not allowed")
			}
		}
	}
	cleanPath := filepath.Clean(filepath.FromSlash(relativePath))
	cleanSlashPath := filepath.ToSlash(cleanPath)
	if cleanSlashPath == "." || strings.HasPrefix(cleanSlashPath, "../") || cleanSlashPath == ".." {
		return nil, fmt.Errorf("invalid resource path")
	}
	if cleanSlashPath != "SKILL.md" && !isAllowedResourcePath(cleanSlashPath) {
		return nil, fmt.Errorf("resource path is outside allowed skill resources")
	}
	if cleanSlashPath != "SKILL.md" && !isTextResourcePath(cleanSlashPath) {
		return nil, fmt.Errorf("resource type is not supported")
	}

	root, err := filepath.Abs(filepath.Clean(pkg.RootDir))
	if err != nil {
		return nil, fmt.Errorf("resolve skill root: %w", err)
	}
	target := filepath.Join(root, filepath.FromSlash(cleanSlashPath))
	if err := ensureWithinRoot(root, target); err != nil {
		return nil, err
	}

	info, err := os.Stat(target)
	if err != nil {
		return nil, fmt.Errorf("stat skill resource: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("skill resource is a directory")
	}
	if info.Size() > int64(maxBytes) {
		return nil, fmt.Errorf("skill resource exceeds %d bytes", maxBytes)
	}

	file, err := os.Open(target)
	if err != nil {
		return nil, fmt.Errorf("open skill resource: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(io.LimitReader(file, int64(maxBytes)+1))
	if err != nil {
		return nil, fmt.Errorf("read skill resource: %w", err)
	}
	if len(content) > maxBytes {
		return nil, fmt.Errorf("skill resource exceeds %d bytes", maxBytes)
	}
	return content, nil
}

func ListResources(pkg Package) ([]string, error) {
	root, err := filepath.Abs(filepath.Clean(pkg.RootDir))
	if err != nil {
		return nil, err
	}
	var result []string
	for _, directory := range []string{"references", "assets", "scripts"} {
		base := filepath.Join(root, directory)
		if _, statErr := os.Stat(base); os.IsNotExist(statErr) {
			continue
		} else if statErr != nil {
			return nil, statErr
		}

		err := filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			relative = filepath.ToSlash(relative)
			if isTextResourcePath(relative) {
				result = append(result, relative)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(result)
	return result, nil
}

func ensureWithinRoot(root, target string) error {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve skill root: %w", err)
	}
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		return fmt.Errorf("resolve skill resource: %w", err)
	}
	relative, err := filepath.Rel(resolvedRoot, resolvedTarget)
	if err != nil {
		return fmt.Errorf("check skill resource path: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("skill resource path escapes skill root")
	}
	return nil
}

func isAllowedResourcePath(path string) bool {
	return strings.HasPrefix(path, "references/") ||
		strings.HasPrefix(path, "assets/") ||
		strings.HasPrefix(path, "scripts/")
}

func isTextResourcePath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".txt", ".json", ".yaml", ".yml", ".toml", ".go", ".py", ".js", ".ts", ".sh", ".ps1", ".css", ".html", ".xml":
		return true
	default:
		return false
	}
}
