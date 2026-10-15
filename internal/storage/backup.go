package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const MaxSQLiteBackupBytes int64 = 4 << 30

type BackupReport struct {
	SchemaVersion int    `json:"schemaVersion"`
	Size          int64  `json:"size"`
	SHA256        string `json:"sha256"`
	Snapshots     int64  `json:"snapshots"`
	Events        int64  `json:"events"`
	RollbackPath  string `json:"rollbackPath,omitempty"`
}

func sqliteReadOnlyURI(name string) (string, error) {
	absolute, err := filepath.Abs(name)
	if err != nil {
		return "", err
	}
	path := filepath.ToSlash(absolute)
	if runtime.GOOS == "windows" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return (&url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro&immutable=1"}).String(), nil
}

// ValidateSQLiteBackup accepts standalone backups with a known schema. It never
// opens a writable database, applies migrations or loads runtime managers.
func ValidateSQLiteBackup(ctx context.Context, name string) (BackupReport, error) {
	var report BackupReport
	if err := rejectSymlinks(name); err != nil {
		return report, err
	}
	stat, err := os.Stat(name)
	if err != nil {
		return report, err
	}
	if !stat.Mode().IsRegular() || stat.Size() == 0 || stat.Size() > MaxSQLiteBackupBytes {
		return report, fmt.Errorf("SQLite backup must be a regular file between 1 byte and 4 GiB")
	}
	if err := requireNoSQLiteSidecars(name); err != nil {
		return report, err
	}
	uri, err := sqliteReadOnlyURI(name)
	if err != nil {
		return report, err
	}
	db, err := sql.Open("sqlite", uri)
	if err != nil {
		return report, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var integrity string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil {
		return report, fmt.Errorf("SQLite integrity check: %w", err)
	}
	if integrity != "ok" {
		return report, fmt.Errorf("SQLite integrity check failed: %s", integrity)
	}
	var versions, count int
	if err := db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version),0), COUNT(*) FROM schema_migrations").Scan(&versions, &count); err != nil {
		return report, fmt.Errorf("backup schema: %w", err)
	}
	migrations, err := sqliteMigrations.ReadDir("migrations")
	if err != nil {
		return report, err
	}
	if versions < 1 || versions > len(migrations) || count != versions {
		return report, fmt.Errorf("unsupported backup schema %d; supported range 1-%d", versions, len(migrations))
	}
	for _, migration := range migrations {
		version, name, err := parseMigrationName(migration.Name())
		if err != nil {
			return report, err
		}
		if version > versions {
			continue
		}
		var stored string
		if err := db.QueryRowContext(ctx, "SELECT name FROM schema_migrations WHERE version = ?", version).Scan(&stored); err != nil || stored != name {
			return report, fmt.Errorf("backup migration history mismatch at version %d", version)
		}
	}
	// Verify the query contract as well as table names, even for empty tables.
	queries := []string{
		"SELECT name,data,backup,updated_at FROM snapshots LIMIT 0",
		"SELECT id,user_id,updated_at,payload_json FROM sessions LIMIT 0",
		"SELECT id,user_id,session_id,status,updated_at,payload_json FROM intents LIMIT 0",
		"SELECT id,user_id,session_id,status,updated_at,payload_json FROM open_loops LIMIT 0",
		"SELECT id,intent_id,user_id,status,updated_at,payload_json FROM deliveries LIMIT 0",
		"SELECT cursor,id,type,created_at,session_id,turn_id,intent_id,delivery_id,payload_json FROM events LIMIT 0",
	}
	if versions >= 2 {
		queries = append(queries, "SELECT user_id FROM events LIMIT 0")
	}
	if versions >= 3 {
		queries = append(queries, "SELECT summary_json FROM sessions LIMIT 0", "SELECT session_id,turn_id,event_type,event_cursor,terminal FROM deliveries LIMIT 0", "SELECT kind,refreshed_at,revision FROM read_model_refreshes LIMIT 0")
	}
	for _, query := range queries {
		rows, err := db.QueryContext(ctx, query)
		if err != nil {
			return report, fmt.Errorf("backup contract: %w", err)
		}
		rows.Close()
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM snapshots").Scan(&report.Snapshots); err != nil {
		return report, err
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM events").Scan(&report.Events); err != nil {
		return report, err
	}
	rows, err := db.QueryContext(ctx, "SELECT name,data,backup FROM snapshots")
	if err != nil {
		return report, err
	}
	for rows.Next() {
		var key string
		var data, backup []byte
		if err := rows.Scan(&key, &data, &backup); err != nil {
			rows.Close()
			return report, err
		}
		if !snapshotDataValid(key, data) && !snapshotDataValid(key, backup) {
			rows.Close()
			return report, fmt.Errorf("backup has invalid snapshot %s", key)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return report, err
	}
	var invalidEvents int64
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM events WHERE NOT json_valid(payload_json)").Scan(&invalidEvents); err != nil {
		return report, err
	}
	if invalidEvents != 0 {
		return report, fmt.Errorf("backup contains invalid event JSON")
	}
	file, err := os.Open(name)
	if err != nil {
		return report, err
	}
	digest, size, err := hashReader(file)
	file.Close()
	if err != nil {
		return report, err
	}
	report.SchemaVersion, report.Size, report.SHA256 = versions, size, digest
	return report, nil
}

func requireNoSQLiteSidecars(name string) error {
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if _, err := os.Lstat(name + suffix); err == nil {
			return fmt.Errorf("SQLite sidecar %s is present; close/checkpoint the database before replacing or validating it", suffix)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// BackupSQLite reads committed WAL content via SQLite, rather than copying the
// main database file. Publishing is exclusive and happens only after validation.
func (p *Persistence) BackupSQLite(ctx context.Context, destination string) (BackupReport, error) {
	if p == nil || !p.Status().Ready || p.Status().Backend != BackendSQLite {
		return BackupReport{}, fmt.Errorf("active SQLite persistence is required")
	}
	return backupSQLite(ctx, p.database, destination)
}

func backupSQLite(ctx context.Context, db *sql.DB, destination string) (BackupReport, error) {
	var report BackupReport
	destination, err := filepath.Abs(destination)
	if err != nil {
		return report, err
	}
	if err := rejectSymlinks(destination); err != nil {
		return report, err
	}
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		return report, fmt.Errorf("backup destination must not exist")
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		return report, err
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".sqlite-backup-*")
	if err != nil {
		return report, err
	}
	staged := temporary.Name()
	temporary.Close()
	defer os.Remove(staged)
	if _, err := db.ExecContext(ctx, "VACUUM main INTO ?", staged); err != nil {
		return report, fmt.Errorf("create SQLite backup: %w", err)
	}
	if err := syncFile(staged); err != nil {
		return report, err
	}
	report, err = ValidateSQLiteBackup(ctx, staged)
	if err != nil {
		return report, err
	}
	if err := ctx.Err(); err != nil {
		return report, err
	}
	if err := os.Link(staged, destination); err != nil {
		return report, fmt.Errorf("publish new backup: %w", err)
	}
	if err := syncDir(filepath.Dir(destination)); err != nil {
		return report, err
	}
	return report, nil
}

// BackupSQLiteOffline holds the same locks as the bot and closes the database
// (including its WAL) before releasing them.
func BackupSQLiteOffline(ctx context.Context, options PersistenceOptions, destination string) (BackupReport, error) {
	options = maintenanceOptions(options)
	options.Backend = "sqlite"
	lease, err := acquirePersistenceLease(options)
	if err != nil {
		return BackupReport{}, err
	}
	defer lease.Close()
	if err := recoverPendingFileRestores(options); err != nil {
		return BackupReport{}, err
	}
	if _, err := os.Stat(options.SQLitePath); err != nil {
		return BackupReport{}, err
	}
	db, err := openSQLiteDatabase(options.SQLitePath, options.BusyTimeout)
	if err != nil {
		return BackupReport{}, err
	}
	report, backupErr := backupSQLite(ctx, db, destination)
	closeErr := db.Close()
	if backupErr != nil {
		return report, backupErr
	}
	return report, closeErr
}

func RestoreSQLiteOffline(ctx context.Context, options PersistenceOptions, source string) (BackupReport, error) {
	options = maintenanceOptions(options)
	options.Backend = "sqlite"
	lease, err := acquirePersistenceLease(options)
	if err != nil {
		return BackupReport{}, err
	}
	defer lease.Close()
	if err := recoverPendingFileRestores(options); err != nil {
		return BackupReport{}, err
	}
	if err := requireNoSQLiteSidecars(source); err != nil {
		return BackupReport{}, err
	}
	if err := rejectSymlinks(options.SQLitePath); err != nil {
		return BackupReport{}, err
	}
	if err := requireNoSQLiteSidecars(options.SQLitePath); err != nil {
		return BackupReport{}, err
	}
	if err := os.MkdirAll(filepath.Dir(options.SQLitePath), 0700); err != nil {
		return BackupReport{}, err
	}
	temp, err := os.CreateTemp(filepath.Dir(options.SQLitePath), ".sqlite-restore-*")
	if err != nil {
		return BackupReport{}, err
	}
	staged := temp.Name()
	temp.Close()
	defer os.Remove(staged)
	if err := copyMaintenanceFile(ctx, source, staged, MaxSQLiteBackupBytes); err != nil {
		return BackupReport{}, err
	}
	report, err := ValidateSQLiteBackup(ctx, staged)
	if err != nil {
		return report, err
	}
	// Migrations run only on this private candidate. The supplied backup and
	// current target stay unchanged if validation or upgrading fails.
	db, err := openSQLiteDatabase(staged, options.BusyTimeout)
	if err != nil {
		return report, err
	}
	upgradeErr := migrateSQLite(db)
	closeErr := db.Close()
	if upgradeErr != nil {
		return report, upgradeErr
	}
	if closeErr != nil {
		return report, closeErr
	}
	report, err = ValidateSQLiteBackup(ctx, staged)
	if err != nil {
		return report, err
	}
	rollback := ""
	if stat, err := os.Lstat(options.SQLitePath); err == nil {
		if !stat.Mode().IsRegular() {
			return report, fmt.Errorf("restore target is not a regular file")
		}
		rollback = options.SQLitePath + ".pre-restore-" + newTransferID("db")
		if err := copyNewMaintenanceFile(ctx, options.SQLitePath, rollback, MaxSQLiteBackupBytes); err != nil {
			return report, err
		}
		if err := syncDir(filepath.Dir(rollback)); err != nil {
			return report, err
		}
	} else if !os.IsNotExist(err) {
		return report, err
	}
	if err := ctx.Err(); err != nil {
		return report, err
	}
	if err := replaceMaintenanceFile(staged, options.SQLitePath); err != nil {
		return report, err
	}
	report.RollbackPath = rollback
	if err := syncDir(filepath.Dir(options.SQLitePath)); err != nil {
		var rollbackErr error
		if rollback != "" {
			file, createErr := os.CreateTemp(filepath.Dir(options.SQLitePath), ".sqlite-rollback-*")
			rollbackErr = createErr
			if createErr == nil {
				name := file.Name()
				file.Close()
				rollbackErr = copyMaintenanceFile(context.Background(), rollback, name, MaxSQLiteBackupBytes)
				if rollbackErr == nil {
					rollbackErr = replaceMaintenanceFile(name, options.SQLitePath)
				}
				os.Remove(name)
			}
		} else {
			rollbackErr = os.Remove(options.SQLitePath)
		}
		return report, errors.Join(err, rollbackErr)
	}
	return report, nil
}

func maintenanceOptions(options PersistenceOptions) PersistenceOptions {
	options.Backend = strings.ToLower(strings.TrimSpace(options.Backend))
	options.DataDir = strings.TrimSpace(options.DataDir)
	options.SQLitePath = strings.TrimSpace(options.SQLitePath)
	options.SQLiteBackupDir = strings.TrimSpace(options.SQLiteBackupDir)
	if options.DataDir == "" {
		options.DataDir = "./data"
	}
	if options.SQLitePath == "" {
		options.SQLitePath = filepath.Join(options.DataDir, "runtime.sqlite")
	}
	if options.SQLiteBackupDir == "" {
		options.SQLiteBackupDir = filepath.Join(options.DataDir, "backups")
	}
	return options
}

func syncFile(name string) error {
	file, err := os.OpenFile(name, os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}

func copyNewMaintenanceFile(ctx context.Context, source, target string, limit int64) error {
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	file.Close()
	if err := copyMaintenanceFile(ctx, source, target, limit); err != nil {
		os.Remove(target)
		return err
	}
	return nil
}

func copyMaintenanceFile(ctx context.Context, source, target string, limit int64) error {
	if err := rejectSymlinks(source); err != nil {
		return err
	}
	if err := rejectSymlinks(target); err != nil {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return fmt.Errorf("invalid maintenance source size/type")
	}
	output, err := os.OpenFile(target, os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	copied, copyErr := io.Copy(output, io.LimitReader(&contextReader{ctx: ctx, reader: input}, limit+1))
	if copyErr == nil && copied > limit {
		copyErr = fmt.Errorf("maintenance source exceeds size limit")
	}
	if copyErr == nil {
		copyErr = output.Sync()
	}
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
