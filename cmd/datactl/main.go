// datactl is deliberately independent of config/app: no .env loading, model
// clients, workers or background flushes are started by maintenance commands.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"project-yume/internal/storage"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: datactl <migrate-json|backup-sqlite|validate-sqlite|restore-sqlite|restore-files> --data-dir DIR --sqlite-path PATH [--source PATH|--output PATH|--archive PATH]")
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	root := flags.String("data-dir", "./data", "data directory; must match the bot")
	database := flags.String("sqlite-path", "", "database path; must match the bot")
	backupDirectory := flags.String("backup-dir", "", "configured backup directory to protect during ZIP restore")
	source := flags.String("source", "", "standalone SQLite backup")
	output := flags.String("output", "", "new backup path")
	archive := flags.String("archive", "", "validated ZIP archive")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	options := storage.PersistenceOptions{DataDir: *root, SQLitePath: *database, SQLiteBackupDir: *backupDirectory}
	var report any
	var err error
	switch args[0] {
	case "migrate-json":
		report, err = storage.MigrateJSONOffline(ctx, options)
	case "backup-sqlite":
		if *output == "" {
			return fmt.Errorf("--output is required")
		}
		report, err = storage.BackupSQLiteOffline(ctx, options, *output)
	case "validate-sqlite":
		if *source == "" {
			return fmt.Errorf("--source is required")
		}
		report, err = storage.ValidateSQLiteBackup(ctx, *source)
	case "restore-sqlite":
		if *source == "" {
			return fmt.Errorf("--source is required")
		}
		report, err = storage.RestoreSQLiteOffline(ctx, options, *source)
	case "restore-files":
		if *archive == "" {
			return fmt.Errorf("--archive is required")
		}
		report, err = storage.RestoreFilesOffline(ctx, options, *archive)
	default:
		return fmt.Errorf("unknown maintenance command %q", args[0])
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(report)
}
