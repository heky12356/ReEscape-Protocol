package storage

import (
	"path/filepath"
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

func TestOpenPersistenceOpensSQLiteAndRunsMigrations(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "runtime.sqlite")
	persistence, err := OpenPersistence(PersistenceOptions{
		Backend:     string(BackendSQLite),
		DataDir:     t.TempDir(),
		SQLitePath:  databasePath,
		BusyTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("open SQLite persistence: %v", err)
	}
	defer persistence.Close()
	if status := persistence.Status(); status.Backend != BackendSQLite || !status.Ready {
		t.Fatalf("unexpected SQLite status: %+v", status)
	}
	for _, table := range []string{"schema_migrations", "snapshots", "sessions", "intents", "open_loops", "deliveries", "events"} {
		var count int
		if err := persistence.Database().QueryRow("SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&count); err != nil {
			t.Fatalf("check table %s: %v", table, err)
		}
		if count != 1 {
			t.Fatalf("table %s is missing", table)
		}
	}
	var journalMode string
	if err := persistence.Database().QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("read journal mode: %v", err)
	}
	if strings.ToLower(journalMode) != "wal" {
		t.Fatalf("journal mode = %q, want wal", journalMode)
	}
}

func TestSQLiteSnapshotStoreRoundTripsAndKeepsBackup(t *testing.T) {
	persistence, err := OpenPersistence(PersistenceOptions{Backend: string(BackendSQLite), DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("open SQLite persistence: %v", err)
	}
	defer persistence.Close()

	if err := persistence.Snapshots.Save("state.json", []byte(`{"version":1}`)); err != nil {
		t.Fatalf("save initial snapshot: %v", err)
	}
	if err := persistence.Snapshots.Save("state.json", []byte(`{"version":2}`)); err != nil {
		t.Fatalf("save updated snapshot: %v", err)
	}
	data, err := persistence.Snapshots.Load("state.json")
	if err != nil || string(data) != `{"version":2}` {
		t.Fatalf("load current snapshot = %q, %v", data, err)
	}

	store, ok := persistence.Snapshots.(*SQLiteSnapshotStore)
	if !ok {
		t.Fatal("expected SQLite snapshot store")
	}
	if _, err := store.database.Exec("UPDATE snapshots SET data = ? WHERE name = ?", []byte("{"), "state.json"); err != nil {
		t.Fatalf("corrupt current snapshot: %v", err)
	}
	data, err = persistence.Snapshots.Load("state.json")
	if err != nil || string(data) != `{"version":1}` {
		t.Fatalf("load backup snapshot = %q, %v", data, err)
	}
}

func TestSQLiteMigrationIsIdempotent(t *testing.T) {
	persistence, err := OpenPersistence(PersistenceOptions{Backend: string(BackendSQLite), DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("open SQLite persistence: %v", err)
	}
	database := persistence.Database()
	if err := migrateSQLite(database); err != nil {
		t.Fatalf("rerun SQLite migrations: %v", err)
	}
	var count int
	if err := database.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if count != 1 {
		t.Fatalf("migration count = %d, want 1", count)
	}
	if err := persistence.Close(); err != nil {
		t.Fatalf("close SQLite persistence: %v", err)
	}
	if err := persistence.Close(); err != nil {
		t.Fatalf("second close returned unexpected error: %v", err)
	}
}
