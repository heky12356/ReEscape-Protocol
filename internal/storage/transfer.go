package storage

import (
	"archive/zip"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	TransferManifestVersion = 1
	MaxTransferArchiveBytes = 128 << 20
)

type TransferJobStatus string

const (
	TransferPending   TransferJobStatus = "pending"
	TransferRunning   TransferJobStatus = "running"
	TransferCompleted TransferJobStatus = "completed"
	TransferFailed    TransferJobStatus = "failed"
)

type TransferJob struct {
	ID          string            `json:"id"`
	Kind        string            `json:"kind"`
	Status      TransferJobStatus `json:"status"`
	Artifact    string            `json:"-"`
	Error       string            `json:"error,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
	CompletedAt *time.Time        `json:"completedAt,omitempty"`
}

type transferManifest struct {
	Version   int            `json:"version"`
	CreatedAt time.Time      `json:"createdAt"`
	Files     []transferFile `json:"files"`
}

type transferFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type TransferManager struct {
	root      string
	exportDir string
	importDir string

	mu   sync.RWMutex
	jobs map[string]*TransferJob
}

func NewTransferManager(dataDir string) (*TransferManager, error) {
	root := strings.TrimSpace(dataDir)
	if root == "" {
		root = "./data"
	}
	root = filepath.Clean(root)
	manager := &TransferManager{
		root:      root,
		exportDir: filepath.Join(root, "exports"),
		importDir: filepath.Join(root, "imports"),
		jobs:      make(map[string]*TransferJob),
	}
	if err := os.MkdirAll(manager.exportDir, 0o755); err != nil {
		return nil, fmt.Errorf("create transfer export directory: %w", err)
	}
	if err := os.MkdirAll(manager.importDir, 0o755); err != nil {
		return nil, fmt.Errorf("create transfer import directory: %w", err)
	}
	return manager, nil
}

func (m *TransferManager) StartExport() (TransferJob, error) {
	if m == nil {
		return TransferJob{}, errors.New("transfer manager is not configured")
	}
	job := m.newJob("export")
	go m.runExport(job.ID)
	return job, nil
}

func (m *TransferManager) StartImport(source io.Reader) (TransferJob, error) {
	if m == nil {
		return TransferJob{}, errors.New("transfer manager is not configured")
	}
	if source == nil {
		return TransferJob{}, errors.New("import archive is empty")
	}
	job := m.newJob("import")
	archivePath := filepath.Join(m.importDir, ".upload-"+job.ID+".zip")
	file, err := os.OpenFile(archivePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		m.failJob(job.ID, err)
		return TransferJob{}, fmt.Errorf("create import staging file: %w", err)
	}
	limited := io.LimitReader(source, MaxTransferArchiveBytes+1)
	_, copyErr := io.Copy(file, limited)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(archivePath)
		m.failJob(job.ID, copyErr)
		return TransferJob{}, fmt.Errorf("stage import archive: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(archivePath)
		m.failJob(job.ID, closeErr)
		return TransferJob{}, fmt.Errorf("close import staging file: %w", closeErr)
	}
	if info, err := os.Stat(archivePath); err != nil {
		_ = os.Remove(archivePath)
		m.failJob(job.ID, err)
		return TransferJob{}, fmt.Errorf("stat import archive: %w", err)
	} else if info.Size() > MaxTransferArchiveBytes {
		_ = os.Remove(archivePath)
		m.failJob(job.ID, errors.New("import archive exceeds size limit"))
		return TransferJob{}, errors.New("import archive exceeds size limit")
	}
	go m.runImport(job.ID, archivePath)
	return job, nil
}

func (m *TransferManager) Get(id string) (TransferJob, bool) {
	if m == nil {
		return TransferJob{}, false
	}
	m.mu.RLock()
	job, ok := m.jobs[strings.TrimSpace(id)]
	if ok {
		job = cloneTransferJob(job)
	}
	m.mu.RUnlock()
	if !ok {
		return TransferJob{}, false
	}
	return *job, true
}

func (m *TransferManager) DownloadPath(id string) (string, bool) {
	job, ok := m.Get(id)
	if !ok || job.Kind != "export" || job.Status != TransferCompleted || job.Artifact == "" {
		return "", false
	}
	return job.Artifact, true
}

func (m *TransferManager) newJob(kind string) TransferJob {
	now := time.Now().UTC()
	job := &TransferJob{ID: newTransferID(kind), Kind: kind, Status: TransferPending, CreatedAt: now, UpdatedAt: now}
	m.mu.Lock()
	m.jobs[job.ID] = job
	m.mu.Unlock()
	return *job
}

func (m *TransferManager) updateJob(id string, update func(*TransferJob)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if job := m.jobs[id]; job != nil {
		update(job)
		job.UpdatedAt = time.Now().UTC()
	}
}

func (m *TransferManager) failJob(id string, err error) {
	m.updateJob(id, func(job *TransferJob) {
		job.Status = TransferFailed
		job.Error = err.Error()
	})
}

func (m *TransferManager) runExport(id string) {
	m.updateJob(id, func(job *TransferJob) { job.Status = TransferRunning })
	archivePath := filepath.Join(m.exportDir, id+".zip")
	if err := m.exportArchive(archivePath); err != nil {
		_ = os.Remove(archivePath)
		m.failJob(id, err)
		return
	}
	now := time.Now().UTC()
	m.updateJob(id, func(job *TransferJob) {
		job.Status = TransferCompleted
		job.Artifact = archivePath
		job.CompletedAt = &now
	})
}

func (m *TransferManager) runImport(id, archivePath string) {
	m.updateJob(id, func(job *TransferJob) { job.Status = TransferRunning })
	err := m.importArchive(archivePath)
	_ = os.Remove(archivePath)
	if err != nil {
		m.failJob(id, err)
		return
	}
	now := time.Now().UTC()
	m.updateJob(id, func(job *TransferJob) {
		job.Status = TransferCompleted
		job.CompletedAt = &now
	})
}

func (m *TransferManager) exportArchive(destination string) error {
	files, err := collectTransferFiles(m.root, m.exportDir, m.importDir)
	if err != nil {
		return err
	}
	manifest := transferManifest{Version: TransferManifestVersion, CreatedAt: time.Now().UTC(), Files: make([]transferFile, 0, len(files))}
	for _, file := range files {
		entry, err := hashTransferFile(m.root, file)
		if err != nil {
			return err
		}
		manifest.Files = append(manifest.Files, entry)
	}
	temporary := destination + ".tmp"
	file, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create export archive: %w", err)
	}
	archive := zip.NewWriter(file)
	manifestData, err := json.Marshal(manifest)
	if err == nil {
		var writer io.Writer
		writer, err = archive.Create("manifest.json")
		if err == nil {
			_, err = writer.Write(manifestData)
		}
	}
	for _, entry := range manifest.Files {
		if err != nil {
			break
		}
		writer, createErr := archive.Create(entry.Path)
		if createErr != nil {
			err = createErr
			break
		}
		input, openErr := os.Open(filepath.Join(m.root, filepath.FromSlash(entry.Path)))
		if openErr != nil {
			err = openErr
			break
		}
		_, err = io.Copy(writer, input)
		_ = input.Close()
	}
	if closeErr := archive.Close(); err == nil {
		err = closeErr
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("write export archive: %w", err)
	}
	if err := os.Rename(temporary, destination); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("publish export archive: %w", err)
	}
	return nil
}

func (m *TransferManager) importArchive(archivePath string) error {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open import archive: %w", err)
	}
	defer archive.Close()
	var manifest transferManifest
	var manifestEntry *zip.File
	entries := make(map[string]*zip.File)
	var totalUncompressed uint64
	for _, entry := range archive.File {
		rel, err := validateTransferPath(entry.Name)
		if err != nil {
			return err
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("import archive contains symbolic link: %s", entry.Name)
		}
		if rel == "manifest.json" {
			if manifestEntry != nil {
				return errors.New("duplicate import manifest")
			}
			manifestEntry = entry
			continue
		}
		if entry.FileInfo().IsDir() {
			continue
		}
		totalUncompressed += entry.UncompressedSize64
		if totalUncompressed > MaxTransferArchiveBytes {
			return errors.New("import archive uncompressed size exceeds limit")
		}
		if _, exists := entries[rel]; exists {
			return fmt.Errorf("duplicate import path: %s", rel)
		}
		entries[rel] = entry
	}
	if manifestEntry != nil {
		reader, openErr := manifestEntry.Open()
		if openErr != nil {
			return openErr
		}
		decodeErr := json.NewDecoder(io.LimitReader(reader, 1<<20)).Decode(&manifest)
		_ = reader.Close()
		if decodeErr != nil {
			return fmt.Errorf("decode import manifest: %w", decodeErr)
		}
	}
	if manifest.Version != TransferManifestVersion {
		return fmt.Errorf("unsupported import manifest version %d", manifest.Version)
	}
	if len(entries) != len(manifest.Files) {
		return errors.New("import manifest file list does not match archive")
	}
	for _, expected := range manifest.Files {
		rel, err := validateTransferPath(expected.Path)
		if err != nil {
			return err
		}
		if expected.Size < 0 || expected.Path != rel {
			return fmt.Errorf("import manifest path is not canonical: %s", expected.Path)
		}
		if isProtectedTransferPath(rel) {
			return fmt.Errorf("import path is protected: %s", rel)
		}
		entry := entries[rel]
		if entry == nil || entry.UncompressedSize64 != uint64(expected.Size) {
			return fmt.Errorf("import manifest size mismatch: %s", rel)
		}
		reader, openErr := entry.Open()
		if openErr != nil {
			return openErr
		}
		digest, size, hashErr := hashReader(reader)
		_ = reader.Close()
		if hashErr != nil {
			return hashErr
		}
		if size != expected.Size || !strings.EqualFold(digest, expected.SHA256) {
			return fmt.Errorf("import manifest checksum mismatch: %s", rel)
		}
	}
	for _, expected := range manifest.Files {
		if err := ensureNoSymlinkPath(m.root, expected.Path); err != nil {
			return err
		}
	}
	for _, expected := range manifest.Files {
		rel, _ := validateTransferPath(expected.Path)
		entry := entries[rel]
		if err := m.restoreTransferFile(entry, rel); err != nil {
			return err
		}
	}
	return nil
}

func (m *TransferManager) restoreTransferFile(entry *zip.File, rel string) error {
	if err := ensureNoSymlinkPath(m.root, rel); err != nil {
		return err
	}
	target := filepath.Join(m.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(target), ".restore-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	reader, err := entry.Open()
	if err == nil {
		_, err = io.Copy(temporary, reader)
		_ = reader.Close()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("stage import file %s: %w", rel, err)
	}
	if err := os.Rename(temporaryName, target); err != nil {
		return fmt.Errorf("restore import file %s: %w", rel, err)
	}
	return nil
}

func collectTransferFiles(root, exportDir, importDir string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == exportDir || current == importDir {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if isProtectedTransferPath(rel) {
			return nil
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("collect export files: %w", err)
	}
	sort.Strings(files)
	return files, nil
}

func hashTransferFile(root, rel string) (transferFile, error) {
	file, err := os.Open(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return transferFile{}, err
	}
	digest, size, hashErr := hashReader(file)
	_ = file.Close()
	if hashErr != nil {
		return transferFile{}, hashErr
	}
	return transferFile{Path: rel, Size: size, SHA256: digest}, nil
}

func hashReader(reader io.Reader) (string, int64, error) {
	hash := sha256.New()
	size, err := io.Copy(hash, reader)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

func validateTransferPath(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" || strings.Contains(raw, "\\") || strings.HasPrefix(raw, "/") {
		return "", fmt.Errorf("unsafe import path: %q", raw)
	}
	clean := path.Clean(raw)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("unsafe import path: %q", raw)
	}
	return clean, nil
}

func isProtectedTransferPath(rel string) bool {
	first := strings.Split(rel, "/")[0]
	return first == "exports" || first == "imports" || strings.EqualFold(rel, ".env") || strings.EqualFold(rel, "admin_session") ||
		strings.EqualFold(rel, "runtime.sqlite") || strings.EqualFold(rel, "runtime.sqlite-wal") ||
		strings.EqualFold(rel, "runtime.sqlite-shm") || strings.EqualFold(rel, "runtime.sqlite-journal")
}

func ensureNoSymlinkPath(root, rel string) error {
	current := root
	for _, part := range strings.Split(rel, "/") {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refuse to write through symbolic link: %s", rel)
		}
	}
	return nil
}

func newTransferID(kind string) string {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("%s-%d", kind, time.Now().UnixNano())
	}
	return kind + "-" + hex.EncodeToString(bytes)
}

func cloneTransferJob(job *TransferJob) *TransferJob {
	clone := *job
	return &clone
}
