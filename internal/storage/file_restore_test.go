package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func fileRestoreArchive(t *testing.T, files map[string]string) string {
	t.Helper()
	source := t.TempDir()
	for name, data := range files {
		writeMigrationSource(t, source, name, data)
	}
	manager, err := NewTransferManager(source)
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "restore.zip")
	if err := manager.exportArchive(archive); err != nil {
		t.Fatal(err)
	}
	return archive
}

func TestFileRestoreRollsBackMidCommitAndRejectsLiveProcess(t *testing.T) {
	root := t.TempDir()
	options := PersistenceOptions{DataDir: root}
	writeMigrationSource(t, root, "a.json", `{"old":true}`)
	archive := fileRestoreArchive(t, map[string]string{"a.json": `{"new":true}`, "nested/b.json": `{"new":true}`})
	p, err := OpenPersistence(options)
	if err != nil {
		t.Fatal(err)
	}
	worker := NewFlushWorker(time.Hour)
	worker.Register("late", func() error { return p.Snapshots.Save("a.json", []byte(`{"flushed":true}`)) })
	worker.MarkDirty("late")
	if _, err := RestoreFilesOffline(context.Background(), options, archive); !errors.Is(err, ErrPersistenceBusy) {
		t.Fatalf("live restore accepted: %v", err)
	}
	if err := worker.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	report, err := restoreFilesOffline(ctx, options, archive, func(index int) error {
		if index == 0 {
			cancel()
		}
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled apply: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "a.json"))
	if err != nil || string(data) != `{"flushed":true}` {
		t.Fatalf("rollback: %s %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(root, "nested/b.json")); !os.IsNotExist(err) {
		t.Fatal("rollback retained new target")
	}
	if report.RollbackDirectory == "" {
		t.Fatal("rollback report missing")
	}
	if _, err := RestoreFilesOffline(context.Background(), options, archive); err != nil {
		t.Fatal(err)
	}
	if err := worker.Stop(); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(root, "a.json"))
	if err != nil || string(data) != `{"new":true}` {
		t.Fatalf("old flush overwrote restore: %s %v", data, err)
	}
}

func TestStartupRecoversPreparedFileRestore(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, ".maintenance", "interrupted")
	writeMigrationSource(t, root, "memory/state.json", `{"partially_restored":true}`)
	writeMigrationSource(t, directory, "undo/memory/state.json", `{"original":true}`)
	journal := restoreJournal{State: "prepared", Entries: []restoreEntry{{Path: "memory/state.json", HadOriginal: true}, {Path: "not-yet-created/new.json"}}}
	if err := writeRestoreJournal(directory, journal); err != nil {
		t.Fatal(err)
	}
	p, err := OpenPersistence(PersistenceOptions{DataDir: root})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	data, err := p.Snapshots.Load("memory/state.json")
	if err != nil || string(data) != `{"original":true}` {
		t.Fatalf("startup recovery: %s %v", data, err)
	}
}

func TestRestoreRejectsInvalidJSONAndProtectedPathsBeforeApply(t *testing.T) {
	root := t.TempDir()
	writeMigrationSource(t, root, "a.json", `{"old":true}`)
	archive := fileRestoreArchive(t, map[string]string{"a.json": `{"new":true}`, "z.JSON": "invalid"})
	if _, err := RestoreFilesOffline(context.Background(), PersistenceOptions{DataDir: root}, archive); err == nil {
		t.Fatal("bad JSON accepted")
	}
	data, _ := os.ReadFile(filepath.Join(root, "a.json"))
	if string(data) != `{"old":true}` {
		t.Fatal("staging failure changed original")
	}
	// A custom SQLite name without a database extension is protected by config.
	protected := fileRestoreArchive(t, map[string]string{"a.json": `{"new":true}`, "custom.store": "database bytes"})
	options := PersistenceOptions{DataDir: root, SQLitePath: filepath.Join(root, "custom.store")}
	if _, err := RestoreFilesOffline(context.Background(), options, protected); err == nil {
		t.Fatal("custom database path accepted")
	}
	data, _ = os.ReadFile(filepath.Join(root, "a.json"))
	if string(data) != `{"old":true}` {
		t.Fatal("protected path failure changed original")
	}
}

func TestTransferProtectsCustomDatabaseSidecarsAndBackups(t *testing.T) {
	root := t.TempDir()
	database := filepath.Join(root, "nested", "custom.store")
	backupDirectory := filepath.Join(root, "private-backups")
	for _, name := range []string{"nested/custom.store", "nested/custom.store-wal", "nested/custom.store-shm", "nested/custom.store-journal", "private-backups/secret.bin", ".runtime.lock", ".env.prod", "state.json"} {
		writeMigrationSource(t, root, name, `{}`)
	}
	manager, err := NewTransferManager(root, database, backupDirectory)
	if err != nil {
		t.Fatal(err)
	}
	files, err := collectTransferFiles(root, manager.exportDir, manager.importDir, manager.protectedPaths...)
	if err != nil || len(files) != 1 || files[0] != "state.json" {
		t.Fatalf("protected export: %v %v", files, err)
	}
}

func TestFileRestoreRejectsSymlinkTarget(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	writeMigrationSource(t, outside, "state.json", `{"outside":true}`)
	if err := os.Symlink(outside, filepath.Join(root, "memory")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	archive := fileRestoreArchive(t, map[string]string{"memory/state.json": `{"new":true}`})
	if _, err := RestoreFilesOffline(context.Background(), PersistenceOptions{DataDir: root}, archive); err == nil {
		t.Fatal("symlink target accepted")
	}
	data, err := os.ReadFile(filepath.Join(outside, "state.json"))
	if err != nil || string(data) != `{"outside":true}` {
		t.Fatal("symlink restore changed outside target")
	}
}
