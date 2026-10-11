package storage

import (
	"strings"
	"testing"
	"time"
)

func TestOpenPersistenceUsesFileBackendByDefault(t *testing.T) {
	persistence, err := OpenPersistence(PersistenceOptions{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("open persistence: %v", err)
	}
	defer persistence.Close()

	status := persistence.Status()
	if status.Backend != BackendFile || !status.Ready {
		t.Fatalf("unexpected persistence status: %+v", status)
	}
	if persistence.Snapshots == nil {
		t.Fatal("expected snapshot store")
	}
	if err := persistence.Snapshots.Save("probe.json", []byte(`{"ok":true}`)); err != nil {
		t.Fatalf("save through persistence boundary: %v", err)
	}
}

func TestOpenPersistenceRejectsUnsupportedBackend(t *testing.T) {
	_, err := OpenPersistence(PersistenceOptions{Backend: "mongo", DataDir: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "unsupported persistence backend") {
		t.Fatalf("unexpected unsupported backend error: %v", err)
	}
}

func TestOpenPersistenceMakesSQLiteOptInExplicit(t *testing.T) {
	_, err := OpenPersistence(PersistenceOptions{
		Backend:     string(BackendSQLite),
		DataDir:     t.TempDir(),
		BusyTimeout: 5 * time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "sqlite persistence backend is not enabled yet") {
		t.Fatalf("unexpected sqlite status: %v", err)
	}
}
