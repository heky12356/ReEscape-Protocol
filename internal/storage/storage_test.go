package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileSnapshotStoreSaveKeepsAtomicSnapshotAndBackup(t *testing.T) {
	store := NewFileSnapshotStore(t.TempDir())
	if err := store.Save("memory/state.json", []byte(`{"version":1}`)); err != nil {
		t.Fatalf("initial save: %v", err)
	}
	if err := store.Save("memory/state.json", []byte(`{"version":2}`)); err != nil {
		t.Fatalf("second save: %v", err)
	}

	data, err := store.Load("memory/state.json")
	if err != nil || string(data) != `{"version":2}` {
		t.Fatalf("load current snapshot = %q, %v", data, err)
	}
	backup, err := os.ReadFile(filepath.Join(store.baseDir, "memory", "state.json.bak"))
	if err != nil || string(backup) != `{"version":1}` {
		t.Fatalf("backup = %q, %v", backup, err)
	}
	entries, err := os.ReadDir(filepath.Join(store.baseDir, "memory"))
	if err != nil {
		t.Fatalf("read snapshot directory: %v", err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Fatalf("temporary file was left behind: %s", entry.Name())
		}
	}
}

func TestFileSnapshotStoreLoadFallsBackToBackup(t *testing.T) {
	baseDir := t.TempDir()
	store := NewFileSnapshotStore(baseDir)
	if err := store.Save("state.json", []byte(`{"ok":true}`)); err != nil {
		t.Fatalf("initial save: %v", err)
	}
	if err := store.Save("state.json", []byte(`{"ok":false}`)); err != nil {
		t.Fatalf("second save: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "state.json"), []byte("{"), 0o644); err != nil {
		t.Fatalf("corrupt primary snapshot: %v", err)
	}

	data, err := store.Load("state.json")
	if err != nil || string(data) != `{"ok":true}` {
		t.Fatalf("load from backup = %q, %v", data, err)
	}
}

func TestFileSnapshotStoreLoadReportsDoubleCorruption(t *testing.T) {
	baseDir := t.TempDir()
	store := NewFileSnapshotStore(baseDir)
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "state.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "state.json.bak"), []byte("["), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := store.Load("state.json")
	if err == nil || !strings.Contains(err.Error(), "invalid JSON") {
		t.Fatalf("expected invalid JSON error, got %v", err)
	}
}

func TestFileSnapshotStoreLoadMissingReturnsNil(t *testing.T) {
	store := NewFileSnapshotStore(t.TempDir())
	data, err := store.Load("missing.json")
	if err != nil || data != nil {
		t.Fatalf("missing snapshot = %q, %v", data, err)
	}
}
