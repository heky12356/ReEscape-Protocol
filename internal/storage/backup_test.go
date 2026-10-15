package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSQLiteBackupIncludesWALAndRestoresOffline(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	database := filepath.Join(root, "nested", "custom.db")
	options := PersistenceOptions{Backend: "sqlite", DataDir: root, SQLitePath: database}
	p, err := OpenPersistence(options)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, err := p.Database().Exec("PRAGMA wal_autocheckpoint = 0"); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"sessions":{"private:42":{"user_id":42,"conversation":[{"role":"user","content":"remember"}],"last_updated":"2026-01-01T00:00:00Z"}},"intents":{"i":{"user_id":42,"session_id":"private:42","status":"pending"}},"open_loops":{"o":{"user_id":42,"session_id":"private:42","status":"open"}}}`)
	if err := p.Snapshots.Save("memory/conversation_history.json", data); err != nil {
		t.Fatal(err)
	}
	event := `{"id":"event","type":"reply_delivered","delivery_id":"d","session_id":"private:42","user_id":42,"created_at":"2026-01-01T00:00:00Z"}`
	tx, err := p.Database().Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec("INSERT INTO events(cursor,id,type,created_at,payload_json) VALUES(15,'event','reply_delivered','2026-01-01T00:00:00Z',?)", event); err != nil {
		t.Fatal(err)
	}
	if err := ProjectDeliveryEvent(ctx, tx, 15, []byte(event)); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if stat, err := os.Stat(database + "-wal"); err != nil || stat.Size() == 0 {
		t.Fatalf("expected uncheckpointed WAL: %v", err)
	}
	backup := filepath.Join(t.TempDir(), "copy with # spaces.sqlite")
	report, err := p.BackupSQLite(ctx, backup)
	if err != nil || report.SchemaVersion != 4 || report.Events != 1 || report.Snapshots != 1 || report.SHA256 == "" {
		t.Fatalf("backup: %+v %v", report, err)
	}
	if _, err := p.BackupSQLite(ctx, backup); err == nil {
		t.Fatal("existing backup overwritten")
	}
	if _, err := RestoreSQLiteOffline(ctx, options, backup); !errors.Is(err, ErrPersistenceBusy) {
		t.Fatalf("running restore allowed: %v", err)
	}
	// The SQLite path lock also works when another data directory is supplied.
	other := options
	other.DataDir = t.TempDir()
	if _, err := OpenPersistence(other); !errors.Is(err, ErrPersistenceBusy) {
		t.Fatalf("shared database opened: %v", err)
	}
	if err := p.Snapshots.Save("memory/conversation_history.json", []byte(`{"sessions":{}}`)); err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	restored, err := RestoreSQLiteOffline(ctx, options, backup)
	if err != nil || restored.RollbackPath == "" {
		t.Fatalf("restore: %+v %v", restored, err)
	}
	old, err := ValidateSQLiteBackup(ctx, restored.RollbackPath)
	if err != nil || old.Snapshots != 1 {
		t.Fatalf("preimage: %+v %v", old, err)
	}
	recovered, err := OpenPersistence(options)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	// Query before loading the snapshot: restore must preserve the standalone
	// derived message tables, not rely on Load silently repairing them.
	messages, err := NewSQLiteQueryStore(recovered.Database()).Messages(ctx, "private:42", 50, "")
	if err != nil || len(messages.Messages) != 1 || messages.Messages[0].Role != "user" {
		t.Fatalf("restored messages: %+v %v", messages, err)
	}
	loaded, err := recovered.Snapshots.Load("memory/conversation_history.json")
	if err != nil || string(loaded) != string(data) {
		t.Fatalf("restored snapshot mismatch: %s %v", loaded, err)
	}
	record, err := NewSQLiteQueryStore(recovered.Database()).Get(ctx, "deliveries", "d")
	if err != nil || record.Status != "delivered" {
		t.Fatalf("restored projection: %+v %v", record, err)
	}
	var cursor int64
	if err := recovered.Database().QueryRow("SELECT MAX(cursor) FROM events").Scan(&cursor); err != nil || cursor != 15 {
		t.Fatalf("cursor lost: %d %v", cursor, err)
	}
}

func TestSQLiteRestoreRejectsInvalidBackupWithoutChangingTarget(t *testing.T) {
	root := t.TempDir()
	options := PersistenceOptions{Backend: "sqlite", DataDir: root}
	p, err := OpenPersistence(options)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Snapshots.Save("state.json", []byte(`{"original":true}`)); err != nil {
		t.Fatal(err)
	}
	good := filepath.Join(t.TempDir(), "good.sqlite")
	if _, err := p.BackupSQLite(context.Background(), good); err != nil {
		t.Fatal(err)
	}
	p.Close()
	target := filepath.Join(root, "runtime.sqlite")
	before, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(t.TempDir(), "bad.sqlite")
	if err := os.WriteFile(bad, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := RestoreSQLiteOffline(context.Background(), options, bad); err == nil {
		t.Fatal("corrupt backup accepted")
	}
	after, _ := os.ReadFile(target)
	if string(before) != string(after) {
		t.Fatal("corrupt backup changed target")
	}
	// A newer/forged migration history is not a compatible restore source.
	db, err := openSQLiteDatabase(good, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO schema_migrations VALUES(99,'future','')"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if _, err := RestoreSQLiteOffline(context.Background(), options, good); err == nil {
		t.Fatal("future schema accepted")
	}
	after, _ = os.ReadFile(target)
	if string(before) != string(after) {
		t.Fatal("schema failure changed target")
	}
	if err := os.WriteFile(target+"-wal", []byte("unreconciled"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := RestoreSQLiteOffline(context.Background(), options, bad); err == nil {
		t.Fatal("outstanding WAL accepted")
	}
}

func TestRestoreUpgradesPrivateLegacyCandidateOnly(t *testing.T) {
	ctx := context.Background()
	source := filepath.Join(t.TempDir(), "old.sqlite")
	db, err := openSQLiteDatabase(source, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,name TEXT,applied_at TEXT)"); err != nil {
		t.Fatal(err)
	}
	for _, filename := range []string{"0001_initial.sql", "0002_event_user_id.sql"} {
		script, err := sqliteMigrations.ReadFile("migrations/" + filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(script)); err != nil {
			t.Fatal(err)
		}
		version, name, err := parseMigrationName(filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec("INSERT INTO schema_migrations VALUES(?,?,'')", version, name); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec("INSERT INTO sessions VALUES('s',42,'2026-01-01T00:00:00Z','{\"user_id\":42,\"conversation\":[]}')"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	validation, err := ValidateSQLiteBackup(ctx, source)
	if err != nil || validation.SchemaVersion != 2 {
		t.Fatalf("legacy validation: %+v %v", validation, err)
	}
	options := PersistenceOptions{DataDir: t.TempDir()}
	report, err := RestoreSQLiteOffline(ctx, options, source)
	if err != nil || report.SchemaVersion != 4 {
		t.Fatalf("upgraded restore: %+v %v", report, err)
	}
	after, err := os.ReadFile(source)
	if err != nil || string(before) != string(after) {
		t.Fatal("restore upgraded the original backup")
	}
}

func TestPersistenceLeaseUsesNormalizedPaths(t *testing.T) {
	root, dbRoot := t.TempDir(), t.TempDir()
	database := filepath.Join(dbRoot, "custom.db")
	p, err := OpenPersistence(PersistenceOptions{Backend: " sqlite ", DataDir: " " + root + " ", SQLitePath: " " + database + " "})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, err := OpenPersistence(PersistenceOptions{Backend: "sqlite", DataDir: t.TempDir(), SQLitePath: database}); !errors.Is(err, ErrPersistenceBusy) {
		t.Fatalf("normalized path bypassed database lock: %v", err)
	}
}
