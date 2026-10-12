package storage

import (
	"database/sql"
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
	database  *sql.DB

	closeOnce sync.Once
	closeErr  error
}

var (
	defaultDatabaseMu sync.RWMutex
	defaultDatabase   *sql.DB
)

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
		database, err := openSQLiteDatabase(sqlitePath, options.BusyTimeout)
		if err != nil {
			return nil, err
		}
		if err := migrateSQLite(database); err != nil {
			_ = database.Close()
			return nil, fmt.Errorf("migrate SQLite database: %w", err)
		}
		if err := os.MkdirAll(backupDir, 0o755); err != nil {
			_ = database.Close()
			return nil, fmt.Errorf("create SQLite backup directory: %w", err)
		}
		setDefaultDatabase(database)
		return &Persistence{
			Snapshots: NewSQLiteSnapshotStore(database),
			status: PersistenceStatus{
				Backend:         BackendSQLite,
				DataDir:         dataDir,
				DatabasePath:    sqlitePath,
				BackupDirectory: backupDir,
				Ready:           true,
			},
			database: database,
		}, nil
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create persistence data directory: %w", err)
	}
	setDefaultDatabase(nil)

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

func DefaultDatabase() *sql.DB {
	defaultDatabaseMu.RLock()
	defer defaultDatabaseMu.RUnlock()
	return defaultDatabase
}

func setDefaultDatabase(database *sql.DB) {
	defaultDatabaseMu.Lock()
	defaultDatabase = database
	defaultDatabaseMu.Unlock()
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
	p.closeOnce.Do(func() {
		if p.database != nil {
			p.closeErr = p.database.Close()
			defaultDatabaseMu.Lock()
			if defaultDatabase == p.database {
				defaultDatabase = nil
			}
			defaultDatabaseMu.Unlock()
		}
	})
	return p.closeErr
}

func (p *Persistence) Database() *sql.DB {
	if p == nil {
		return nil
	}
	return p.database
}
