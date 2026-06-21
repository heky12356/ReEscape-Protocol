package storage

import (
	"os"
	"path/filepath"
	"strings"
)

type SnapshotStore interface {
	Save(name string, data []byte) error
	Load(name string) ([]byte, error)
}

type DirtyMarker interface {
	MarkDirty(name string)
}

type FileSnapshotStore struct {
	baseDir string
}

func NewFileSnapshotStore(baseDir string) *FileSnapshotStore {
	trimmed := strings.TrimSpace(baseDir)
	if trimmed == "" {
		trimmed = "./data"
	}
	return &FileSnapshotStore{baseDir: trimmed}
}

func (s *FileSnapshotStore) Save(name string, data []byte) error {
	path := s.resolvePath(name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (s *FileSnapshotStore) Load(name string) ([]byte, error) {
	path := s.resolvePath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return data, nil
}

func (s *FileSnapshotStore) resolvePath(name string) string {
	cleanName := filepath.Clean(strings.TrimSpace(name))
	return filepath.Join(s.baseDir, cleanName)
}
