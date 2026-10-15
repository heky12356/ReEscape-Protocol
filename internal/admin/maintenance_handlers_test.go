package admin

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"project-yume/internal/storage"
)

func awaitMaintenanceJob(t *testing.T, engine http.Handler, id string) storage.TransferJob {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		response := stateRequest(engine, "GET", "/api/admin/transfers/"+id, true)
		var job storage.TransferJob
		if err := json.Unmarshal(response.Body.Bytes(), &job); err != nil {
			t.Fatal(err)
		}
		if job.Status == storage.TransferCompleted || job.Status == storage.TransferFailed {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("maintenance job timed out")
	return storage.TransferJob{}
}

func TestOnlineImportValidatesWithoutApplying(t *testing.T) {
	p, engine := setupStateAdmin(t, "file")
	oversized := httptest.NewRequest("POST", "/api/admin/transfers/import", nil)
	oversized.ContentLength = storage.MaxTransferArchiveBytes + (1 << 20) + 1
	oversized.Header.Set("Authorization", "Bearer query-test-key")
	rejected := httptest.NewRecorder()
	engine.ServeHTTP(rejected, oversized)
	if rejected.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized multipart accepted: %d", rejected.Code)
	}
	if err := p.Snapshots.Save("state.json", []byte(`{"original":true}`)); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"restored":true}`)
	manifest := map[string]any{"version": 1, "files": []any{map[string]any{"path": "state.json", "size": len(data), "sha256": fmt.Sprintf("%x", sha256.Sum256(data))}}}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	entry, _ := writer.Create("manifest.json")
	entry.Write(encoded)
	entry, _ = writer.Create("state.json")
	entry.Write(data)
	writer.Close()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	upload, _ := form.CreateFormFile("archive", "restore.zip")
	upload.Write(archive.Bytes())
	form.Close()
	request := httptest.NewRequest("POST", "/api/admin/transfers/import", &body)
	request.Header.Set("Content-Type", form.FormDataContentType())
	request.Header.Set("Authorization", "Bearer query-test-key")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != 202 {
		t.Fatalf("upload: %d %s", response.Code, response.Body)
	}
	var job storage.TransferJob
	if err := json.Unmarshal(response.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	job = awaitMaintenanceJob(t, engine, job.ID)
	if job.Status != storage.TransferCompleted || job.Kind != "import_validation" || job.Applied || job.NextAction == "" {
		t.Fatalf("job: %+v", job)
	}
	loaded, err := p.Snapshots.Load("state.json")
	if err != nil || string(loaded) != `{"original":true}` {
		t.Fatalf("online import applied data: %s %v", loaded, err)
	}
	staged, err := filepath.Glob(filepath.Join(p.Status().DataDir, "imports", ".upload-*.zip"))
	if err != nil || len(staged) != 1 {
		t.Fatal("validated upload was not retained for offline restore")
	}
}

func TestSQLiteBackupJobIsAuthenticatedAndDownloadable(t *testing.T) {
	p, engine := setupStateAdmin(t, "sqlite")
	if err := p.Snapshots.Save("state.json", []byte(`{"backup":true}`)); err != nil {
		t.Fatal(err)
	}
	if response := stateRequest(engine, "POST", "/api/admin/persistence/backup", false); response.Code != 401 {
		t.Fatal(response.Code)
	}
	response := stateRequest(engine, "POST", "/api/admin/persistence/backup", true)
	if response.Code != 202 {
		t.Fatalf("backup: %d %s", response.Code, response.Body)
	}
	var job storage.TransferJob
	if err := json.Unmarshal(response.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	job = awaitMaintenanceJob(t, engine, job.ID)
	if job.Status != storage.TransferCompleted || job.Kind != "sqlite_backup" {
		t.Fatalf("backup job: %+v", job)
	}
	download := stateRequest(engine, "GET", "/api/admin/transfers/"+job.ID+"/download", true)
	if download.Code != 200 || download.Header().Get("Content-Type") != "application/octet-stream" {
		t.Fatalf("backup download: %d %s", download.Code, download.Body)
	}
	name := filepath.Join(t.TempDir(), "download.sqlite")
	if err := os.WriteFile(name, download.Body.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := storage.ValidateSQLiteBackup(context.Background(), name)
	if err != nil || report.Snapshots != 1 {
		t.Fatalf("download validation: %+v %v", report, err)
	}
}
