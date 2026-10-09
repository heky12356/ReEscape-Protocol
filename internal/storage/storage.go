package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"project-yume/internal/metrics"
	"project-yume/internal/utils"
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
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		metrics.IncCounter("bot_snapshot_save_total", "Total snapshot save attempts by result.", map[string]string{"result": "error"})
		return err
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		metrics.IncCounter("bot_snapshot_save_total", "Total snapshot save attempts by result.", map[string]string{"result": "error"})
		return fmt.Errorf("create snapshot temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		metrics.IncCounter("bot_snapshot_save_total", "Total snapshot save attempts by result.", map[string]string{"result": "error"})
		return fmt.Errorf("set snapshot temporary file permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		metrics.IncCounter("bot_snapshot_save_total", "Total snapshot save attempts by result.", map[string]string{"result": "error"})
		return fmt.Errorf("write snapshot temporary file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		metrics.IncCounter("bot_snapshot_save_total", "Total snapshot save attempts by result.", map[string]string{"result": "error"})
		return fmt.Errorf("sync snapshot temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		metrics.IncCounter("bot_snapshot_save_total", "Total snapshot save attempts by result.", map[string]string{"result": "error"})
		return fmt.Errorf("close snapshot temporary file: %w", err)
	}

	backupPath := path + ".bak"
	if _, err := os.Stat(path); err == nil {
		// Keep exactly one known-good predecessor. Rename is atomic within a directory.
		if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
			metrics.IncCounter("bot_snapshot_save_total", "Total snapshot save attempts by result.", map[string]string{"result": "error"})
			return fmt.Errorf("remove old snapshot backup: %w", err)
		}
		if err := os.Rename(path, backupPath); err != nil {
			metrics.IncCounter("bot_snapshot_save_total", "Total snapshot save attempts by result.", map[string]string{"result": "error"})
			return fmt.Errorf("backup snapshot: %w", err)
		}
		defer func() {
			if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
				if restoreErr := os.Rename(backupPath, path); restoreErr != nil {
					utils.Error("restore snapshot after failed replacement: %v", restoreErr)
				}
			}
		}()
	} else if !os.IsNotExist(err) {
		metrics.IncCounter("bot_snapshot_save_total", "Total snapshot save attempts by result.", map[string]string{"result": "error"})
		return fmt.Errorf("stat snapshot: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		metrics.IncCounter("bot_snapshot_save_total", "Total snapshot save attempts by result.", map[string]string{"result": "error"})
		return fmt.Errorf("replace snapshot: %w", err)
	}
	if err := syncDir(dir); err != nil {
		// The file is already safely replaced; surface the durability failure to the caller.
		metrics.IncCounter("bot_snapshot_save_total", "Total snapshot save attempts by result.", map[string]string{"result": "error"})
		return fmt.Errorf("sync snapshot directory: %w", err)
	}
	metrics.IncCounter("bot_snapshot_save_total", "Total snapshot save attempts by result.", map[string]string{"result": "ok"})
	return nil
}

func (s *FileSnapshotStore) Load(name string) ([]byte, error) {
	path := s.resolvePath(name)
	data, err := os.ReadFile(path)
	if err == nil && snapshotDataValid(name, data) {
		metrics.IncCounter("bot_snapshot_load_total", "Total snapshot load attempts by result.", map[string]string{"result": "ok"})
		return data, nil
	}

	primaryErr := err
	if err == nil {
		primaryErr = fmt.Errorf("snapshot contains invalid JSON")
	}
	backupPath := path + ".bak"
	backup, backupErr := os.ReadFile(backupPath)
	if backupErr == nil && snapshotDataValid(name, backup) {
		utils.Warn("snapshot %s is unavailable (%v); restored from backup", name, primaryErr)
		metrics.IncCounter("bot_snapshot_load_total", "Total snapshot load attempts by result.", map[string]string{"result": "backup"})
		return backup, nil
	}
	if os.IsNotExist(primaryErr) && os.IsNotExist(backupErr) {
		metrics.IncCounter("bot_snapshot_load_total", "Total snapshot load attempts by result.", map[string]string{"result": "missing"})
		return nil, nil
	}
	metrics.IncCounter("bot_snapshot_load_total", "Total snapshot load attempts by result.", map[string]string{"result": "error"})
	if backupErr != nil {
		return nil, fmt.Errorf("load snapshot %s: %v; backup %s: %w", path, primaryErr, backupPath, backupErr)
	}
	return nil, fmt.Errorf("load snapshot %s: %v; backup %s is invalid JSON", path, primaryErr, backupPath)
}

func snapshotDataValid(name string, data []byte) bool {
	if !strings.EqualFold(filepath.Ext(strings.TrimSpace(name)), ".json") {
		return true
	}
	return json.Valid(data)
}

func syncDir(path string) error {
	// Windows does not permit opening directories for fsync. The renamed file
	// is already flushed; directory durability is provided by the filesystem.
	if runtime.GOOS == "windows" {
		return nil
	}
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func (s *FileSnapshotStore) resolvePath(name string) string {
	cleanName := filepath.Clean(strings.TrimSpace(name))
	return filepath.Join(s.baseDir, cleanName)
}
