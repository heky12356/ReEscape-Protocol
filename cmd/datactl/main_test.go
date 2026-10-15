package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"project-yume/internal/storage"
)

func TestDatactlMigrationBackupRestore(t *testing.T) {
	root := t.TempDir()
	database := filepath.Join(t.TempDir(), "runtime.sqlite")
	backup := filepath.Join(t.TempDir(), "backup.sqlite")
	if err := os.MkdirAll(filepath.Join(root, "memory"), 0700); err != nil {
		t.Fatal(err)
	}
	original := `{"sessions":{"s":{"user_id":42,"last_updated":"2026-01-01T00:00:00Z"}}}`
	if err := os.WriteFile(filepath.Join(root, "memory/conversation_history.json"), []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"migrate-json", "--data-dir", root, "--sqlite-path", database},
		{"backup-sqlite", "--data-dir", root, "--sqlite-path", database, "--output", backup},
		{"validate-sqlite", "--source", backup},
		{"restore-sqlite", "--data-dir", root, "--sqlite-path", database, "--source", backup},
	} {
		var output bytes.Buffer
		if err := run(context.Background(), args, &output); err != nil {
			t.Fatalf("%s: %v", args[0], err)
		}
		var report storage.BackupReport
		if err := json.Unmarshal(output.Bytes(), &report); err != nil || report.SchemaVersion != 4 {
			t.Fatalf("output: %s %v", output.String(), err)
		}
	}
	for _, args := range [][]string{nil, {"unknown"}, {"restore-sqlite"}, {"backup-sqlite"}, {"validate-sqlite"}, {"restore-files"}, {"restore-files", "--unknown"}} {
		if err := run(context.Background(), args, &bytes.Buffer{}); err == nil {
			t.Fatalf("accepted invalid args: %v", args)
		}
	}
}
