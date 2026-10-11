package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var sqliteMigrations embed.FS

func openSQLiteDatabase(path string, busyTimeout time.Duration) (*sql.DB, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("SQLite database path is empty")
	}
	if err := ensureSQLiteParent(path); err != nil {
		return nil, err
	}
	database, err := sql.Open("sqlite", filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("open SQLite database: %w", err)
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)

	if busyTimeout <= 0 {
		busyTimeout = 5 * time.Second
	}
	pragmaStatements := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA foreign_keys = ON",
		fmt.Sprintf("PRAGMA busy_timeout = %d", busyTimeout.Milliseconds()),
	}
	for _, statement := range pragmaStatements {
		if _, err := database.Exec(statement); err != nil {
			_ = database.Close()
			return nil, fmt.Errorf("configure SQLite %q: %w", statement, err)
		}
	}
	if err := database.Ping(); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("ping SQLite database: %w", err)
	}
	return database, nil
}

func ensureSQLiteParent(path string) error {
	parent := filepath.Dir(filepath.Clean(path))
	if err := ensureDirectory(parent); err != nil {
		return fmt.Errorf("create SQLite database directory: %w", err)
	}
	return nil
}

func ensureDirectory(path string) error {
	if strings.TrimSpace(path) == "" || path == "." {
		return nil
	}
	return os.MkdirAll(path, 0o755)
}

func migrateSQLite(database *sql.DB) error {
	if database == nil {
		return fmt.Errorf("SQLite database is nil")
	}
	if _, err := database.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
        version INTEGER PRIMARY KEY,
        name TEXT NOT NULL,
        applied_at TEXT NOT NULL
    )`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := sqliteMigrations.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read SQLite migrations: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		version, name, err := parseMigrationName(entry.Name())
		if err != nil {
			return err
		}
		var applied bool
		if err := database.QueryRow("SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = ?)", version).Scan(&applied); err != nil {
			return fmt.Errorf("check SQLite migration %d: %w", version, err)
		}
		if applied {
			continue
		}
		migration, err := sqliteMigrations.ReadFile(path.Join("migrations", entry.Name()))
		if err != nil {
			return fmt.Errorf("read SQLite migration %s: %w", entry.Name(), err)
		}
		transaction, err := database.Begin()
		if err != nil {
			return fmt.Errorf("begin SQLite migration %d: %w", version, err)
		}
		if _, err := transaction.ExecContext(context.Background(), string(migration)); err != nil {
			_ = transaction.Rollback()
			return fmt.Errorf("apply SQLite migration %d: %w", version, err)
		}
		if _, err := transaction.Exec("INSERT INTO schema_migrations(version, name, applied_at) VALUES (?, ?, ?)", version, name, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			_ = transaction.Rollback()
			return fmt.Errorf("record SQLite migration %d: %w", version, err)
		}
		if err := transaction.Commit(); err != nil {
			return fmt.Errorf("commit SQLite migration %d: %w", version, err)
		}
	}
	return nil
}

func parseMigrationName(filename string) (int, string, error) {
	parts := strings.SplitN(filename, "_", 2)
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("invalid SQLite migration filename %q", filename)
	}
	version, err := strconv.Atoi(parts[0])
	if err != nil || version <= 0 {
		return 0, "", fmt.Errorf("invalid SQLite migration filename %q", filename)
	}
	name := strings.TrimSuffix(parts[1], ".sql")
	if name == "" {
		return 0, "", fmt.Errorf("invalid SQLite migration filename %q", filename)
	}
	return version, name, nil
}

type SQLiteSnapshotStore struct {
	database *sql.DB
}

func NewSQLiteSnapshotStore(database *sql.DB) *SQLiteSnapshotStore {
	return &SQLiteSnapshotStore{database: database}
}

func (s *SQLiteSnapshotStore) Save(name string, data []byte) error {
	if s == nil || s.database == nil {
		return fmt.Errorf("SQLite snapshot store is not configured")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("snapshot name is empty")
	}
	_, err := s.database.Exec(`INSERT INTO snapshots(name, data, backup, updated_at)
        VALUES (?, ?, NULL, ?)
        ON CONFLICT(name) DO UPDATE SET
            backup = snapshots.data,
            data = excluded.data,
            updated_at = excluded.updated_at`, name, data, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("save SQLite snapshot %s: %w", name, err)
	}
	return nil
}

func (s *SQLiteSnapshotStore) Load(name string) ([]byte, error) {
	if s == nil || s.database == nil {
		return nil, fmt.Errorf("SQLite snapshot store is not configured")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("snapshot name is empty")
	}
	var data []byte
	var backup []byte
	err := s.database.QueryRow("SELECT data, backup FROM snapshots WHERE name = ?", name).Scan(&data, &backup)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load SQLite snapshot %s: %w", name, err)
	}
	if snapshotDataValid(name, data) {
		return data, nil
	}
	if len(backup) > 0 && snapshotDataValid(name, backup) {
		return backup, nil
	}
	return nil, fmt.Errorf("SQLite snapshot %s and its backup contain invalid JSON", name)
}
