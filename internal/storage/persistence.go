package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// BackendKind identifies the persistent storage implementation selected for the
// process. File snapshots remain the default; SQLite is selected explicitly.
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
// the existing domain stores; admin queries receive this same instance.
type Persistence struct {
	Snapshots SnapshotStore
	status    PersistenceStatus
	database  *sql.DB

	closeOnce sync.Once
	closeErr  error
	closed    atomic.Bool
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
	status := p.status
	status.Ready = status.Ready && !p.closed.Load()
	return status
}

func (p *Persistence) Close() error {
	if p == nil {
		return nil
	}
	p.closeOnce.Do(func() {
		p.closed.Store(true)
		if p.database != nil {
			p.closeErr = p.database.Close()
		}
	})
	return p.closeErr
}

type ReadModelCapability struct {
	Kind        string   `json:"kind"`
	Filters     []string `json:"filters"`
	RefreshedAt string   `json:"refreshedAt,omitempty"`
}

type PersistenceInfo struct {
	Backend                      BackendKind           `json:"backend"`
	Ready                        bool                  `json:"ready"`
	SchemaVersion                int                   `json:"schemaVersion"`
	DefaultLimit                 int                   `json:"defaultLimit"`
	MaxLimit                     int                   `json:"maxLimit"`
	ReadModels                   []ReadModelCapability `json:"readModels"`
	Capabilities                 []string              `json:"capabilities"`
	StorageChangesRequireRestart bool                  `json:"storageChangesRequireRestart"`
}

// Info reports the running instance, never environment configuration or paths.
func (p *Persistence) Info(ctx context.Context) (PersistenceInfo, error) {
	status := p.Status()
	info := PersistenceInfo{Backend: status.Backend, Ready: status.Ready, DefaultLimit: DefaultQueryLimit,
		MaxLimit: MaxQueryLimit, ReadModels: []ReadModelCapability{}, Capabilities: []string{}, StorageChangesRequireRestart: true}
	if !status.Ready {
		return info, nil
	}
	info.Capabilities = append(info.Capabilities, "snapshots", "event_cursor")
	if status.Backend != BackendSQLite {
		return info, nil
	}
	tx, err := p.database.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return info, err
	}
	defer tx.Rollback()
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&info.SchemaVersion); err != nil {
		return info, err
	}
	rows, err := tx.QueryContext(ctx, "SELECT kind, refreshed_at FROM read_model_refreshes ORDER BY kind")
	if err != nil {
		return info, err
	}
	refreshed := map[string]string{}
	for rows.Next() {
		var kind, at string
		if err := rows.Scan(&kind, &at); err != nil {
			rows.Close()
			return info, err
		}
		refreshed[kind] = at
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return info, err
	}
	for _, kind := range []string{"sessions", "intents", "open_loops", "deliveries"} {
		filters := []string{"user_id", "session_id"}
		if kind != "sessions" {
			filters = append(filters, "status")
		}
		info.ReadModels = append(info.ReadModels, ReadModelCapability{Kind: kind, Filters: filters, RefreshedAt: refreshed[kind]})
	}
	info.Capabilities = append(info.Capabilities, "state_query", "state_detail", "delivery_projection_rebuild")
	return info, tx.Commit()
}

func (p *Persistence) Database() *sql.DB {
	if p == nil {
		return nil
	}
	return p.database
}
