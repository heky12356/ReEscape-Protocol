package storage

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTransferManagerExportsAndImportsManifestedFiles(t *testing.T) {
	sourceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, "state.json"), []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, ".env"), []byte("SECRET=hidden"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager, err := NewTransferManager(sourceDir)
	if err != nil {
		t.Fatalf("new transfer manager: %v", err)
	}
	exportJob, err := manager.StartExport()
	if err != nil {
		t.Fatalf("start export: %v", err)
	}
	exportJob = waitTransferJob(t, manager, exportJob.ID)
	if exportJob.Status != TransferCompleted {
		t.Fatalf("export status = %s, error = %s", exportJob.Status, exportJob.Error)
	}
	archiveData, err := os.ReadFile(exportJob.Artifact)
	if err != nil {
		t.Fatalf("read export archive: %v", err)
	}
	archive, err := zip.NewReader(bytes.NewReader(archiveData), int64(len(archiveData)))
	if err != nil {
		t.Fatalf("open export archive: %v", err)
	}
	var names []string
	for _, entry := range archive.File {
		names = append(names, entry.Name)
	}
	if len(names) != 2 || names[0] != "manifest.json" || names[1] != "state.json" {
		t.Fatalf("unexpected archive entries: %v", names)
	}

	targetDir := t.TempDir()
	target, err := NewTransferManager(targetDir)
	if err != nil {
		t.Fatalf("new target transfer manager: %v", err)
	}
	importJob, err := target.StartImport(bytes.NewReader(archiveData))
	if err != nil {
		t.Fatalf("start import: %v", err)
	}
	importJob = waitTransferJob(t, target, importJob.ID)
	if importJob.Status != TransferCompleted {
		t.Fatalf("import status = %s, error = %s", importJob.Status, importJob.Error)
	}
	if importJob.Applied || importJob.NextAction == "" {
		t.Fatal("upload must only validate")
	}
	if _, err := os.Stat(filepath.Join(targetDir, "state.json")); !os.IsNotExist(err) {
		t.Fatal("online validation changed files")
	}
	if _, err := RestoreFilesOffline(context.Background(), PersistenceOptions{DataDir: targetDir}, importJob.Artifact); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(targetDir, "state.json"))
	if err != nil || string(data) != `{"ok":true}` {
		t.Fatalf("restored state = %q, error = %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, ".env")); !os.IsNotExist(err) {
		t.Fatalf("protected .env was exported or restored")
	}
}

func TestTransferManagerRejectsTraversalArchive(t *testing.T) {
	dataDir := t.TempDir()
	manager, err := NewTransferManager(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	manifest := transferManifest{
		Version:   TransferManifestVersion,
		CreatedAt: time.Now().UTC(),
		Files:     []transferFile{{Path: "../escape.txt", Size: 4, SHA256: "bad"}},
	}
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	manifestData, _ := json.Marshal(manifest)
	manifestWriter, _ := archive.Create("manifest.json")
	_, _ = manifestWriter.Write(manifestData)
	fileWriter, _ := archive.Create("../escape.txt")
	_, _ = fileWriter.Write([]byte("safe"))
	_ = archive.Close()

	job, err := manager.StartImport(bytes.NewReader(buffer.Bytes()))
	if err != nil {
		t.Fatalf("start import: %v", err)
	}
	job = waitTransferJob(t, manager, job.ID)
	if job.Status != TransferFailed {
		t.Fatalf("status = %s, want failed", job.Status)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dataDir), "escape.txt")); !os.IsNotExist(err) {
		t.Fatalf("traversal archive wrote outside data directory")
	}
}

func waitTransferJob(t *testing.T, manager *TransferManager, id string) TransferJob {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		job, ok := manager.Get(id)
		if ok && (job.Status == TransferCompleted || job.Status == TransferFailed) {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	job, _ := manager.Get(id)
	return job
}
