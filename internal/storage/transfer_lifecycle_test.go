package storage

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type transferReaderFunc func([]byte) (int, error)

func (reader transferReaderFunc) Read(buffer []byte) (int, error) { return reader(buffer) }

func TestTransferExcludesConcurrentJobsAndCancelsStaging(t *testing.T) {
	m, err := NewTransferManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	entered, release := make(chan struct{}), make(chan struct{})
	result := make(chan TransferJob, 1)
	go func() {
		job, _ := m.StartImport(transferReaderFunc(func([]byte) (int, error) { close(entered); <-release; return 0, io.EOF }))
		result <- job
	}()
	<-entered
	if _, err := m.StartExport(); !errors.Is(err, ErrTransferBusy) {
		t.Fatalf("concurrent export: %v", err)
	}
	p := queryTestPersistence(t)
	if _, err := m.StartSQLiteBackup(p); !errors.Is(err, ErrTransferBusy) {
		t.Fatalf("concurrent backup: %v", err)
	}
	before := m.Overview()
	if before.ActiveJobID == "" || len(before.Jobs) != 1 || before.Jobs[0].Stage != "staging" {
		t.Fatalf("staging overview: %+v", before)
	}
	closed := make(chan struct{})
	go func() { m.Close(); close(closed) }()
	<-m.ctx.Done()
	close(release)
	<-result
	<-closed
	job, ok := m.Get(before.ActiveJobID)
	if !ok || job.Status != TransferCancelled || job.CompletedAt == nil || m.Overview().ActiveJobID != "" {
		t.Fatalf("cancelled job: %+v", job)
	}
	files, _ := filepath.Glob(filepath.Join(m.importDir, "*"))
	if len(files) != 0 {
		t.Fatalf("cancelled upload leaked: %v", files)
	}
	if _, err := m.StartExport(); !errors.Is(err, ErrTransferClosed) {
		t.Fatalf("closed manager accepted job: %v", err)
	}
}

func TestTransferReportsAndArtifactRetention(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "state.json"), []byte(`{"ok":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := NewTransferManager(root)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	job, err := m.StartExport()
	if err != nil {
		t.Fatal(err)
	}
	job = waitTransferJob(t, m, job.ID)
	m.workers.Wait()
	if job.Status != TransferCompleted || job.Report == nil || job.Report.FileCount != 1 || len(job.Report.Files) != 1 || !job.DownloadAvailable || job.Stage != "completed" {
		t.Fatalf("export result: %+v", job)
	}
	job.Report.Files[0].Path = "mutated"
	copy, _ := m.Get(job.ID)
	if copy.Report.Files[0].Path != "state.json" {
		t.Fatal("report escaped manager ownership")
	}
	overview := m.Overview()
	if len(overview.Jobs) != 1 || len(overview.Jobs[0].Report.Files) != 0 {
		t.Fatal("list should omit large file manifest")
	}
	for i := 0; i < MaxTransferJobs; i++ {
		next, err := m.newJob("export")
		if err != nil {
			t.Fatal(err)
		}
		m.failJob(next.ID, errors.New("test terminal job"))
		m.releaseJob(next.ID)
	}
	if _, ok := m.Get(job.ID); ok {
		t.Fatal("old metadata was not evicted")
	}
	if len(m.Overview().Jobs) != MaxTransferJobs {
		t.Fatal("unbounded job records")
	}
	if _, err := os.Stat(job.Artifact); err != nil {
		t.Fatal("metadata eviction deleted retained artifact", err)
	}
}

func TestTransferValidationFailuresLeaveNoStagingOrAppliedData(t *testing.T) {
	for _, name := range []string{"checksum", "size", "protected", "symbolic_link", "version", "duplicate"} {
		t.Run(name, func(t *testing.T) {
			m, err := NewTransferManager(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer m.Close()
			content := []byte("safe")
			entry := transferFile{Path: "state.json", Size: 4, SHA256: fmt.Sprintf("%x", sha256.Sum256(content))}
			manifest := transferManifest{Version: TransferManifestVersion, Files: []transferFile{entry}}
			switch name {
			case "checksum":
				manifest.Files[0].SHA256 = "bad"
			case "size":
				manifest.Files[0].Size = 2
			case "protected":
				manifest.Files[0].Path = ".env"
			case "version":
				manifest.Version = 99
			case "duplicate":
				manifest.Files = append(manifest.Files, entry)
			}
			var buffer bytes.Buffer
			archive := zip.NewWriter(&buffer)
			writer, _ := archive.Create("manifest.json")
			data, _ := json.Marshal(manifest)
			writer.Write(data)
			header := &zip.FileHeader{Name: manifest.Files[0].Path, Method: zip.Deflate}
			if name == "symbolic_link" {
				header.SetMode(os.ModeSymlink | 0600)
			}
			writer, _ = archive.CreateHeader(header)
			writer.Write(content)
			archive.Close()
			job, err := m.StartImport(bytes.NewReader(buffer.Bytes()))
			if err != nil {
				t.Fatal(err)
			}
			job = waitTransferJob(t, m, job.ID)
			if job.Status != TransferFailed || job.Stage != "failed" || job.Applied || job.Error == "" || job.CompletedAt == nil {
				t.Fatalf("failed validation: %+v", job)
			}
			staged, _ := filepath.Glob(filepath.Join(m.importDir, "*"))
			if len(staged) > 0 {
				t.Fatalf("failed upload leaked: %v", staged)
			}
			if _, err := os.Stat(filepath.Join(m.root, "state.json")); !os.IsNotExist(err) {
				t.Fatal("validation applied content")
			}
		})
	}
}

func TestTransferDownloadDetectsRemovedArtifactAndRejectsSymlink(t *testing.T) {
	m, err := NewTransferManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	job, err := m.StartExport()
	if err != nil {
		t.Fatal(err)
	}
	job = waitTransferJob(t, m, job.ID)
	if _, ok := m.DownloadPath(job.ID); !ok {
		t.Fatal("completed artifact not downloadable")
	}
	if err := os.Remove(job.Artifact); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.DownloadPath(job.ID); ok {
		t.Fatal("missing artifact offered")
	}
	outside := filepath.Join(t.TempDir(), "secret")
	os.WriteFile(outside, []byte("secret"), 0600)
	if err := os.Symlink(outside, job.Artifact); err != nil {
		t.Skip("symlinks unavailable")
	}
	if _, ok := m.DownloadPath(job.ID); ok {
		t.Fatal("symlink artifact offered")
	}
}

func TestTransferCancelledExportCleansTemporaryArchive(t *testing.T) {
	m, err := NewTransferManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	m.cancel()
	job, err := m.StartExport()
	if err != nil {
		t.Fatal(err)
	}
	m.workers.Wait()
	job, _ = m.Get(job.ID)
	if job.Status != TransferCancelled || !strings.Contains(job.Error, "canceled") {
		t.Fatalf("cancelled export: %+v", job)
	}
	files, _ := os.ReadDir(m.exportDir)
	if len(files) != 0 {
		t.Fatal("partial export leaked")
	}
}
