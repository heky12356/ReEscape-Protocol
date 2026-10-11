package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// BackendKind identifies the persistent storage implementation selected for the
// process. File snapshots remain the default until the SQLite schema is ready.
type BackendKind string

const (
	BackendFile   BackendKind = "file"
	BackendSQLite BackendKind = "sqlite"
)

type PersistenceOptions struct {
	Backend         string
	DataDir         string
	SQLitePath      string
	SQLiteBackupDir string
	BusyTimeout     time.Duration
}

type PersistenceStatus struct {
	Backend         BackendKind
	DataDir         string
	DatabasePath    string
	BackupDirectory string
	Ready           bool
}

// Persistence owns the selected backend boundary. Callers use Snapshots for
// the existing domain stores and can inspect Status before future query-model
// backends are enabled.
type Persistence struct {
	Snapshots SnapshotStore
	status    PersistenceStatus

	closeOnce sync.Once
	closeErr  error
}

func OpenPersistence(options PersistenceOptions) (*Persistence, error) {
	backend := BackendKind(strings.ToLower(strings.TrimSpace(options.Backend)))
	if backend == "" {
		backend = BackendFile
	}
	if backend != BackendFile && backend != BackendSQLite {
		return nil, fmt.Errorf("unsupported persistence backend %q", options.Backend)
	}

	dataDir := strings.TrimSpace(options.DataDir)
	if dataDir == "" {
		dataDir = "./data"
	}
	sqlitePath := strings.TrimSpace(options.SQLitePath)
	if sqlitePath == "" {
		sqlitePath = filepath.Join(dataDir, "runtime.sqlite")
	}
	backupDir := strings.TrimSpace(options.SQLiteBackupDir)
	if backupDir == "" {
		backupDir = filepath.Join(dataDir, "backups")
	}

	if backend == BackendSQLite {
		return nil, fmt.Errorf("sqlite persistence backend is not enabled yet; use PERSISTENCE_BACKEND=file")
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create persistence data directory: %w", err)
	}

	return &Persistence{
		Snapshots: NewFileSnapshotStore(dataDir),
		status: PersistenceStatus{
			Backend:         BackendFile,
			DataDir:         dataDir,
			DatabasePath:    sqlitePath,
			BackupDirectory: backupDir,
			Ready:           true,
		},
	}, nil
}

func (p *Persistence) Status() PersistenceStatus {
	if p == nil {
		return PersistenceStatus{}
	}
	return p.status
}

func (p *Persistence) Close() error {
	if p == nil {
		return nil
	}
	p.closeOnce.Do(func() {})
	return p.closeErr
}
