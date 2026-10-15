package storage

import (
	"archive/zip"
	"context"
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
	TransferManifestVersion  = 1
	MaxTransferArchiveBytes  = 128 << 20
	MaxTransferFiles         = 4096
	MaxTransferManifestBytes = 1 << 20
	MaxTransferJobs          = 100
)

type TransferJobStatus string

const (
	TransferPending   TransferJobStatus = "pending"
	TransferRunning   TransferJobStatus = "running"
	TransferCompleted TransferJobStatus = "completed"
	TransferFailed    TransferJobStatus = "failed"
	TransferCancelled TransferJobStatus = "cancelled"
)

type TransferJob struct {
	Stage             string            `json:"stage"`
	Report            *TransferReport   `json:"report,omitempty"`
	DownloadAvailable bool              `json:"downloadAvailable"`
	Applied           bool              `json:"applied"`
	NextAction        string            `json:"nextAction,omitempty"`
	ID                string            `json:"id"`
	Kind              string            `json:"kind"`
	Status            TransferJobStatus `json:"status"`
	Artifact          string            `json:"-"`
	Error             string            `json:"error,omitempty"`
	CreatedAt         time.Time         `json:"createdAt"`
	UpdatedAt         time.Time         `json:"updatedAt"`
	CompletedAt       *time.Time        `json:"completedAt,omitempty"`
}

type TransferReport struct {
	FileCount  int            `json:"fileCount"`
	TotalBytes int64          `json:"totalBytes"`
	Files      []transferFile `json:"files,omitempty"`
	Backup     *BackupReport  `json:"backup,omitempty"`
}

var ErrTransferBusy = errors.New("another transfer job is active")
var ErrTransferClosed = errors.New("transfer manager is closed")

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

	mu             sync.RWMutex
	jobs           map[string]*TransferJob
	protectedPaths []string
	activeID       string
	instanceID     string
	ctx            context.Context
	cancel         context.CancelFunc
	workers        sync.WaitGroup
	closed         bool
}

func NewTransferManager(dataDir string, protectedPaths ...string) (*TransferManager, error) {
	root := strings.TrimSpace(dataDir)
	if root == "" {
		root = "./data"
	}
	var err error
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := rejectSymlinks(root); err != nil {
		return nil, err
	}
	manager := &TransferManager{
		root:           root,
		exportDir:      filepath.Join(root, "exports"),
		importDir:      filepath.Join(root, "imports"),
		jobs:           make(map[string]*TransferJob),
		protectedPaths: append([]string(nil), protectedPaths...),
	}
	manager.ctx, manager.cancel = context.WithCancel(context.Background())
	manager.instanceID = newTransferID("transfers")
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
	job, err := m.newJob("export")
	if err != nil {
		return TransferJob{}, err
	}
	go func() { defer m.releaseJob(job.ID); m.runExport(job.ID) }()
	return job, nil
}

func (m *TransferManager) StartImport(source io.Reader) (TransferJob, error) {
	if m == nil {
		return TransferJob{}, errors.New("transfer manager is not configured")
	}
	if source == nil {
		return TransferJob{}, errors.New("import archive is empty")
	}
	job, err := m.newJob("import_validation")
	if err != nil {
		return TransferJob{}, err
	}
	started := false
	defer func() {
		if !started {
			m.releaseJob(job.ID)
		}
	}()
	m.updateJob(job.ID, func(job *TransferJob) { job.Status = TransferRunning; job.Stage = "staging" })
	archivePath := filepath.Join(m.importDir, ".upload-"+job.ID+".zip")
	file, err := os.OpenFile(archivePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		m.failJob(job.ID, err)
		return TransferJob{}, fmt.Errorf("create import staging file: %w", err)
	}
	limited := io.LimitReader(source, MaxTransferArchiveBytes+1)
	_, copyErr := io.Copy(file, &contextReader{ctx: m.ctx, reader: limited})
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
	started = true
	go func() { defer m.releaseJob(job.ID); m.runImport(job.ID, archivePath) }()
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
	job.DownloadAvailable = m.downloadable(job)
	return *job, true
}

func (m *TransferManager) DownloadPath(id string) (string, bool) {
	job, ok := m.Get(id)
	if !ok || !job.DownloadAvailable {
		return "", false
	}
	return job.Artifact, true
}

func (m *TransferManager) newJob(kind string) (TransferJob, error) {
	now := time.Now().UTC()
	job := &TransferJob{ID: newTransferID(kind), Kind: kind, Status: TransferPending, Stage: "queued", CreatedAt: now, UpdatedAt: now}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return TransferJob{}, ErrTransferClosed
	}
	if m.activeID != "" {
		return TransferJob{}, ErrTransferBusy
	}
	for len(m.jobs) >= MaxTransferJobs {
		var oldest *TransferJob
		for _, candidate := range m.jobs {
			if oldest == nil || candidate.CreatedAt.Before(oldest.CreatedAt) {
				oldest = candidate
			}
		}
		delete(m.jobs, oldest.ID)
	}
	m.jobs[job.ID] = job
	m.activeID = job.ID
	m.workers.Add(1)
	return *job, nil
}

func (m *TransferManager) releaseJob(id string) {
	m.mu.Lock()
	if m.activeID == id {
		m.activeID = ""
	}
	m.mu.Unlock()
	m.workers.Done()
}
func (m *TransferManager) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.closed = true
	m.cancel()
	m.mu.Unlock()
	m.workers.Wait()
}

type TransferOverview struct {
	InstanceID       string        `json:"instanceId"`
	ActiveJobID      string        `json:"activeJobId,omitempty"`
	MaxArchiveBytes  int64         `json:"maxArchiveBytes"`
	MaxFiles         int           `json:"maxFiles"`
	MaxManifestBytes int           `json:"maxManifestBytes"`
	MaxJobs          int           `json:"maxJobs"`
	Jobs             []TransferJob `json:"jobs"`
}

func (m *TransferManager) Overview() TransferOverview {
	m.mu.RLock()
	result := TransferOverview{InstanceID: m.instanceID, ActiveJobID: m.activeID, MaxArchiveBytes: MaxTransferArchiveBytes, MaxFiles: MaxTransferFiles, MaxManifestBytes: MaxTransferManifestBytes, MaxJobs: MaxTransferJobs, Jobs: []TransferJob{}}
	for _, job := range m.jobs {
		clone := *job
		if job.Report != nil {
			report := *job.Report
			report.Files = nil
			if report.Backup != nil {
				backup := *report.Backup
				report.Backup = &backup
			}
			clone.Report = &report
		}
		result.Jobs = append(result.Jobs, clone)
	}
	m.mu.RUnlock()
	for i := range result.Jobs {
		result.Jobs[i].DownloadAvailable = m.downloadable(&result.Jobs[i])
	}
	sort.Slice(result.Jobs, func(i, j int) bool { return result.Jobs[i].CreatedAt.After(result.Jobs[j].CreatedAt) })
	return result
}
func (m *TransferManager) downloadable(job *TransferJob) bool {
	if (job.Kind != "export" && job.Kind != "sqlite_backup") || job.Status != TransferCompleted || job.Artifact == "" {
		return false
	}
	if rejectSymlinks(job.Artifact) != nil {
		return false
	}
	info, err := os.Stat(job.Artifact)
	return err == nil && info.Mode().IsRegular()
}

func (m *TransferManager) workContext() context.Context {
	if m.ctx == nil {
		return context.Background()
	}
	return m.ctx
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
		job.Stage = "failed"
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			job.Status = TransferCancelled
			job.Stage = "cancelled"
		}
		job.Error = err.Error()
		now := time.Now().UTC()
		job.CompletedAt = &now
	})
}

func (m *TransferManager) runExport(id string) {
	m.updateJob(id, func(job *TransferJob) { job.Status = TransferRunning; job.Stage = "archiving" })
	archivePath := filepath.Join(m.exportDir, id+".zip")
	if err := m.exportArchive(archivePath); err != nil {
		_ = os.Remove(archivePath)
		m.failJob(id, err)
		return
	}
	report, err := readTransferReport(archivePath)
	if err != nil {
		_ = os.Remove(archivePath)
		m.failJob(id, err)
		return
	}
	now := time.Now().UTC()
	m.updateJob(id, func(job *TransferJob) {
		job.Status = TransferCompleted
		job.Stage = "completed"
		job.Report = report
		job.Artifact = archivePath
		job.CompletedAt = &now
	})
}

func (m *TransferManager) runImport(id, archivePath string) {
	m.updateJob(id, func(job *TransferJob) { job.Status = TransferRunning; job.Stage = "validating" })
	err := m.validateArchive(archivePath)
	if err != nil {
		_ = os.Remove(archivePath)
		m.failJob(id, err)
		return
	}
	report, err := readTransferReport(archivePath)
	if err != nil {
		_ = os.Remove(archivePath)
		m.failJob(id, err)
		return
	}
	now := time.Now().UTC()
	m.updateJob(id, func(job *TransferJob) {
		job.Status = TransferCompleted
		job.Stage = "validated"
		job.Report = report
		job.Artifact = archivePath
		job.NextAction = "stop the bot and wait for its final flush; use datactl restore-files --archive <DATA_DIR>/imports/.upload-" + id + ".zip with the configured data/sqlite/backup paths for file data only; use restore-sqlite for SQLite"
		job.CompletedAt = &now
	})
}

func (m *TransferManager) exportArchive(destination string) error {
	ctx := m.workContext()
	files, err := collectTransferFilesContext(ctx, m.root, m.exportDir, m.importDir, m.protectedPaths...)
	if err != nil {
		return err
	}
	manifest := transferManifest{Version: TransferManifestVersion, CreatedAt: time.Now().UTC(), Files: make([]transferFile, 0, len(files))}
	if len(files) > MaxTransferFiles {
		return errors.New("export exceeds 4096 files")
	}
	var totalBytes int64
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		input, err := os.Open(filepath.Join(m.root, filepath.FromSlash(file)))
		if err != nil {
			return err
		}
		digest, size, err := hashReader(&contextReader{ctx: ctx, reader: io.LimitReader(input, MaxTransferArchiveBytes-totalBytes+1)})
		input.Close()
		if err != nil {
			return err
		}
		if size > MaxTransferArchiveBytes-totalBytes {
			return errors.New("export uncompressed size exceeds limit")
		}
		totalBytes += size
		entry := transferFile{Path: file, Size: size, SHA256: digest}
		manifest.Files = append(manifest.Files, entry)
	}
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	if len(manifestData) > MaxTransferManifestBytes {
		return errors.New("export manifest exceeds size limit")
	}
	temporary := destination + ".tmp"
	file, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create export archive: %w", err)
	}
	archive := zip.NewWriter(file)
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
		hash := sha256.New()
		var size int64
		size, err = io.Copy(io.MultiWriter(writer, hash), &contextReader{ctx: ctx, reader: io.LimitReader(input, entry.Size+1)})
		_ = input.Close()
		if err == nil && (size != entry.Size || hex.EncodeToString(hash.Sum(nil)) != entry.SHA256) {
			err = fmt.Errorf("export source changed while archiving: %s", entry.Path)
		}
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
	if err := ctx.Err(); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	if info, err := os.Stat(temporary); err != nil || info.Size() > MaxTransferArchiveBytes {
		_ = os.Remove(temporary)
		return errors.New("export archive exceeds size limit or is unavailable")
	}
	if err := os.Rename(temporary, destination); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("publish export archive: %w", err)
	}
	return nil
}

func (m *TransferManager) validateArchive(archivePath string) error {
	ctx := m.workContext()
	if err := ctx.Err(); err != nil {
		return err
	}
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
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := validateTransferPath(entry.Name)
		if err != nil {
			return err
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("import archive contains symbolic link: %s", entry.Name)
		}
		if !entry.FileInfo().IsDir() && rel != entry.Name {
			return errors.New("import entry path is not canonical")
		}
		if !entry.FileInfo().IsDir() {
			if entry.UncompressedSize64 > MaxTransferArchiveBytes || totalUncompressed > MaxTransferArchiveBytes-entry.UncompressedSize64 {
				return errors.New("import archive uncompressed size exceeds limit")
			}
			totalUncompressed += entry.UncompressedSize64
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
		if len(entries) >= MaxTransferFiles {
			return errors.New("import archive exceeds 4096 files")
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
		manifestData, readErr := io.ReadAll(&contextReader{ctx: ctx, reader: io.LimitReader(reader, MaxTransferManifestBytes+1)})
		_ = reader.Close()
		if readErr != nil {
			return readErr
		}
		if len(manifestData) > MaxTransferManifestBytes {
			return errors.New("import manifest exceeds size limit")
		}
		if err := json.Unmarshal(manifestData, &manifest); err != nil {
			return fmt.Errorf("decode import manifest: %w", err)
		}
	}
	if manifest.Version != TransferManifestVersion {
		return fmt.Errorf("unsupported import manifest version %d", manifest.Version)
	}
	if len(entries) != len(manifest.Files) {
		return errors.New("import manifest file list does not match archive")
	}
	manifestPaths := make(map[string]struct{}, len(manifest.Files))
	for _, expected := range manifest.Files {
		rel, err := validateTransferPath(expected.Path)
		if err != nil {
			return err
		}
		if expected.Size < 0 || expected.Path != rel {
			return fmt.Errorf("import manifest path is not canonical: %s", expected.Path)
		}
		if _, exists := manifestPaths[rel]; exists {
			return fmt.Errorf("duplicate import manifest path: %s", rel)
		}
		manifestPaths[rel] = struct{}{}
		if m.isProtected(rel) {
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
		digest, size, hashErr := hashReader(&contextReader{ctx: ctx, reader: io.LimitReader(reader, expected.Size+1)})
		_ = reader.Close()
		if hashErr != nil {
			return hashErr
		}
		if size != expected.Size || !strings.EqualFold(digest, expected.SHA256) {
			return fmt.Errorf("import manifest checksum mismatch: %s", rel)
		}
	}
	if len(manifestPaths) != len(entries) {
		return errors.New("import archive contains files missing from manifest")
	}
	for _, expected := range manifest.Files {
		if err := ensureNoSymlinkPath(m.root, expected.Path); err != nil {
			return err
		}
	}
	return nil
}

func collectTransferFiles(root, exportDir, importDir string, protectedPaths ...string) ([]string, error) {
	return collectTransferFilesContext(context.Background(), root, exportDir, importDir, protectedPaths...)
}

func collectTransferFilesContext(ctx context.Context, root, exportDir, importDir string, protectedPaths ...string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(current string, entry os.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
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
		rel, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if protectedTransferPath(root, rel, protectedPaths...) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		files = append(files, rel)
		if len(files) > MaxTransferFiles {
			return errors.New("export exceeds 4096 files")
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("collect export files: %w", err)
	}
	sort.Strings(files)
	return files, nil
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
	if strings.TrimSpace(raw) == "" || strings.ContainsAny(raw, "\\:\x00") || strings.HasPrefix(raw, "/") {
		return "", fmt.Errorf("unsafe import path: %q", raw)
	}
	clean := path.Clean(raw)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("unsafe import path: %q", raw)
	}
	for _, component := range strings.Split(clean, "/") {
		if strings.TrimRight(component, ". ") != component {
			return "", fmt.Errorf("non-portable import path")
		}
		base := strings.ToUpper(strings.SplitN(component, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9') {
			return "", fmt.Errorf("reserved import path")
		}
	}
	return clean, nil
}

func isProtectedTransferPath(rel string) bool {
	first := strings.ToLower(strings.Split(rel, "/")[0])
	base := strings.ToLower(path.Base(rel))
	if strings.HasPrefix(base, ".sqlite-backup-") || strings.HasPrefix(base, ".sqlite-restore-") || strings.HasPrefix(base, ".json-migration-") {
		return true
	}
	if extension := strings.ToLower(path.Ext(base)); extension == ".sqlite" || extension == ".sqlite3" || extension == ".db" {
		return true
	}
	if first == "backups" || first == ".maintenance" || strings.HasPrefix(base, ".env") || strings.HasSuffix(base, ".runtime.lock") || strings.Contains(base, ".pre-restore-") {
		return true
	}
	return first == "exports" || first == "imports" || strings.EqualFold(rel, ".env") || strings.EqualFold(rel, "admin_session") ||
		strings.EqualFold(rel, "runtime.sqlite") || strings.EqualFold(rel, "runtime.sqlite-wal") ||
		strings.EqualFold(rel, "runtime.sqlite-shm") || strings.EqualFold(rel, "runtime.sqlite-journal")
}

func (m *TransferManager) isProtected(rel string) bool {
	return protectedTransferPath(m.root, rel, m.protectedPaths...)
}

func protectedTransferPath(root, rel string, paths ...string) bool {
	if isProtectedTransferPath(rel) {
		return true
	}
	target, _ := filepath.Abs(filepath.Join(root, filepath.FromSlash(rel)))
	for _, name := range paths {
		if strings.TrimSpace(name) == "" {
			continue
		}
		absolute, err := filepath.Abs(name)
		if err != nil {
			return true
		}
		if strings.EqualFold(target, absolute) || strings.EqualFold(target, absolute+"-wal") || strings.EqualFold(target, absolute+"-shm") || strings.EqualFold(target, absolute+"-journal") {
			return true
		}
		relative, err := filepath.Rel(absolute, target)
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func (m *TransferManager) StartSQLiteBackup(p *Persistence) (TransferJob, error) {
	if m == nil {
		return TransferJob{}, errors.New("transfer manager is not configured")
	}
	if p == nil || !p.Status().Ready || p.Status().Backend != BackendSQLite {
		return TransferJob{}, errors.New("active SQLite persistence is required")
	}
	job, err := m.newJob("sqlite_backup")
	if err != nil {
		return TransferJob{}, err
	}
	go func() {
		defer m.releaseJob(job.ID)
		m.updateJob(job.ID, func(job *TransferJob) { job.Status = TransferRunning; job.Stage = "backing_up" })
		ctx, cancel := context.WithTimeout(m.ctx, 10*time.Minute)
		defer cancel()
		destination := filepath.Join(p.Status().BackupDirectory, job.ID+".sqlite")
		report, err := p.BackupSQLite(ctx, destination)
		if err != nil {
			m.failJob(job.ID, err)
			return
		}
		now := time.Now().UTC()
		m.updateJob(job.ID, func(job *TransferJob) {
			job.Status = TransferCompleted
			job.Stage = "completed"
			job.Report = &TransferReport{Backup: &report}
			job.Artifact = destination
			job.CompletedAt = &now
		})
	}()
	return job, nil
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
	if job.Report != nil {
		report := *job.Report
		report.Files = append([]transferFile(nil), job.Report.Files...)
		if report.Backup != nil {
			backup := *report.Backup
			report.Backup = &backup
		}
		clone.Report = &report
	}
	return &clone
}

// The archive is already bounded/validated by the caller; reports never expose
// server absolute paths or restore rollback locations.
func readTransferReport(name string) (*TransferReport, error) {
	archive, err := zip.OpenReader(name)
	if err != nil {
		return nil, err
	}
	defer archive.Close()
	for _, entry := range archive.File {
		if entry.Name == "manifest.json" {
			reader, err := entry.Open()
			if err != nil {
				return nil, err
			}
			defer reader.Close()
			var manifest transferManifest
			if err := json.NewDecoder(io.LimitReader(reader, MaxTransferManifestBytes+1)).Decode(&manifest); err != nil {
				return nil, err
			}
			report := &TransferReport{FileCount: len(manifest.Files), Files: manifest.Files}
			for _, file := range manifest.Files {
				report.TotalBytes += file.Size
			}
			return report, nil
		}
	}
	return nil, errors.New("manifest missing")
}
