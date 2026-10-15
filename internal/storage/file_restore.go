package storage

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type FileRestoreReport struct {
	Files             int    `json:"files"`
	RollbackDirectory string `json:"rollbackDirectory"`
}
type restoreEntry struct {
	Path        string `json:"path"`
	HadOriginal bool   `json:"hadOriginal"`
}
type restoreJournal struct {
	State   string         `json:"state"`
	Entries []restoreEntry `json:"entries"`
}

func RestoreFilesOffline(ctx context.Context, options PersistenceOptions, archivePath string) (FileRestoreReport, error) {
	return restoreFilesOffline(ctx, options, archivePath, nil)
}

func restoreFilesOffline(ctx context.Context, options PersistenceOptions, archivePath string, afterPublish func(int) error) (FileRestoreReport, error) {
	options = maintenanceOptions(options)
	if options.Backend != "" && options.Backend != "file" {
		return FileRestoreReport{}, fmt.Errorf("ZIP restore requires file backend; use restore-sqlite for SQLite")
	}
	lease, err := acquirePersistenceLease(options)
	if err != nil {
		return FileRestoreReport{}, err
	}
	defer lease.Close()
	if err := recoverPendingFileRestores(options); err != nil {
		return FileRestoreReport{}, err
	}
	manager, err := NewTransferManager(options.DataDir, options.SQLitePath, options.SQLiteBackupDir)
	if err != nil {
		return FileRestoreReport{}, err
	}
	defer manager.Close()
	directory := filepath.Join(manager.root, ".maintenance", newTransferID("restore"))
	if err := rejectSymlinks(directory); err != nil {
		return FileRestoreReport{}, err
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return FileRestoreReport{}, err
	}
	report := FileRestoreReport{RollbackDirectory: directory}
	stagedArchive := filepath.Join(directory, "source.zip")
	if err := copyNewMaintenanceFile(ctx, archivePath, stagedArchive, MaxTransferArchiveBytes); err != nil {
		return report, err
	}
	if err := manager.validateArchive(stagedArchive); err != nil {
		return report, err
	}
	archive, err := zip.OpenReader(stagedArchive)
	if err != nil {
		return report, err
	}
	defer archive.Close()
	journal := restoreJournal{State: "prepared"}
	// Every file is fully staged and every original copied before the first swap.
	for _, entry := range archive.File {
		if entry.Name == "manifest.json" || entry.FileInfo().IsDir() {
			continue
		}
		relative, err := validateTransferPath(entry.Name)
		if err != nil {
			return report, err
		}
		if manager.isProtected(relative) || entry.Mode()&os.ModeSymlink != 0 {
			return report, fmt.Errorf("protected archive entry")
		}
		target := filepath.Join(manager.root, filepath.FromSlash(relative))
		staged := filepath.Join(directory, "staged", filepath.FromSlash(relative))
		if err := rejectSymlinks(target); err != nil {
			return report, err
		}
		if err := os.MkdirAll(filepath.Dir(staged), 0700); err != nil {
			return report, err
		}
		reader, err := entry.Open()
		if err != nil {
			return report, err
		}
		file, err := os.OpenFile(staged, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			reader.Close()
			return report, err
		}
		copied, copyErr := io.Copy(file, io.LimitReader(&contextReader{ctx: ctx, reader: reader}, MaxTransferArchiveBytes+1))
		reader.Close()
		if copyErr == nil && (copied > MaxTransferArchiveBytes || copied != int64(entry.UncompressedSize64)) {
			copyErr = fmt.Errorf("archive entry size changed")
		}
		if copyErr == nil {
			copyErr = file.Sync()
		}
		closeErr := file.Close()
		if copyErr != nil {
			return report, copyErr
		}
		if closeErr != nil {
			return report, closeErr
		}
		if strings.EqualFold(filepath.Ext(relative), ".json") {
			data, err := os.ReadFile(staged)
			if err != nil {
				return report, err
			}
			if !json.Valid(data) {
				return report, fmt.Errorf("invalid JSON in %s", relative)
			}
		}
		item := restoreEntry{Path: relative}
		if stat, err := os.Lstat(target); err == nil {
			if !stat.Mode().IsRegular() {
				return report, fmt.Errorf("restore target is not regular: %s", relative)
			}
			undo := filepath.Join(directory, "undo", filepath.FromSlash(relative))
			if err := os.MkdirAll(filepath.Dir(undo), 0700); err != nil {
				return report, err
			}
			if err := copyNewMaintenanceFile(ctx, target, undo, MaxTransferArchiveBytes); err != nil {
				return report, err
			}
			if err := syncDir(filepath.Dir(undo)); err != nil {
				return report, err
			}
			item.HadOriginal = true
		} else if !os.IsNotExist(err) {
			return report, err
		}
		journal.Entries = append(journal.Entries, item)
	}
	report.Files = len(journal.Entries)
	if err := syncRestoreDirectories(directory); err != nil {
		return report, err
	}
	if err := syncDir(filepath.Dir(directory)); err != nil {
		return report, err
	}
	if err := syncDir(manager.root); err != nil {
		return report, err
	}
	if err := writeRestoreJournal(directory, journal); err != nil {
		return report, err
	}
	applyErr := func() error {
		for i, entry := range journal.Entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			target := filepath.Join(manager.root, filepath.FromSlash(entry.Path))
			if err := rejectSymlinks(target); err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
				return err
			}
			if err := replaceMaintenanceFile(filepath.Join(directory, "staged", filepath.FromSlash(entry.Path)), target); err != nil {
				return err
			}
			if err := syncDir(filepath.Dir(target)); err != nil {
				return err
			}
			if afterPublish != nil {
				if err := afterPublish(i); err != nil {
					return err
				}
			}
		}
		journal.State = "committed"
		return writeRestoreJournal(directory, journal)
	}()
	if applyErr != nil {
		journal.State = "prepared"
		// Persist a rollback marker before any undo, including when publishing the
		// committed marker itself failed to sync. Crash recovery can then retry.
		if err := writeRestoreJournal(directory, journal); err != nil {
			return report, errors.Join(applyErr, err)
		}
		rollbackErr := rollbackFileRestore(manager, directory, journal)
		if rollbackErr != nil {
			return report, errors.Join(applyErr, fmt.Errorf("rollback pending; keep the bot stopped: %w", rollbackErr))
		}
		return report, applyErr
	}
	return report, nil
}

func writeRestoreJournal(directory string, journal restoreJournal) error {
	data, err := json.Marshal(journal)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, ".journal-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	_, err = file.Write(data)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := replaceMaintenanceFile(temporary, filepath.Join(directory, "journal.json")); err != nil {
		return err
	}
	return syncDir(directory)
}

func rollbackFileRestore(manager *TransferManager, directory string, journal restoreJournal) error {
	for _, entry := range journal.Entries {
		relative, err := validateTransferPath(entry.Path)
		if err != nil || relative != entry.Path || manager.isProtected(relative) {
			return fmt.Errorf("unsafe restore journal path")
		}
		if err := rejectSymlinks(filepath.Join(manager.root, filepath.FromSlash(relative))); err != nil {
			return err
		}
	}
	for i := len(journal.Entries) - 1; i >= 0; i-- {
		entry := journal.Entries[i]
		relative, err := validateTransferPath(entry.Path)
		if err != nil || relative != entry.Path || manager.isProtected(relative) {
			return fmt.Errorf("unsafe restore journal path")
		}
		target := filepath.Join(manager.root, filepath.FromSlash(relative))
		if err := rejectSymlinks(target); err != nil {
			return err
		}
		if entry.HadOriginal {
			temporary, err := os.CreateTemp(filepath.Dir(target), ".rollback-*")
			if err != nil {
				return err
			}
			name := temporary.Name()
			temporary.Close()
			err = copyMaintenanceFile(context.Background(), filepath.Join(directory, "undo", filepath.FromSlash(relative)), name, MaxTransferArchiveBytes)
			if err == nil {
				err = replaceMaintenanceFile(name, target)
			}
			os.Remove(name)
			if err != nil {
				return err
			}
		} else if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			return err
		}
		if stat, err := os.Stat(filepath.Dir(target)); err == nil && stat.IsDir() {
			if err := syncDir(filepath.Dir(target)); err != nil {
				return err
			}
		}
	}
	journal.State = "rolled_back"
	return writeRestoreJournal(directory, journal)
}

func syncRestoreDirectories(directory string) error {
	return filepath.WalkDir(directory, func(name string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return syncDir(name)
		}
		return nil
	})
}

// Called under the process lease before any Manager can load data or flush it.
func recoverPendingFileRestores(options PersistenceOptions) error {
	options = maintenanceOptions(options)
	root, err := filepath.Abs(options.DataDir)
	if err != nil {
		return err
	}
	operations := filepath.Join(root, ".maintenance")
	if err := rejectSymlinks(operations); err != nil {
		return err
	}
	entries, err := os.ReadDir(operations)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	manager := &TransferManager{root: root, protectedPaths: []string{options.SQLitePath, options.SQLiteBackupDir}}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		directory := filepath.Join(operations, entry.Name())
		name := filepath.Join(directory, "journal.json")
		if err := rejectSymlinks(name); err != nil {
			return err
		}
		file, err := os.Open(name)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		var journal restoreJournal
		err = json.NewDecoder(io.LimitReader(file, 1<<20)).Decode(&journal)
		file.Close()
		if err != nil {
			return fmt.Errorf("invalid restore journal; keep the bot stopped: %w", err)
		}
		switch journal.State {
		case "committed", "rolled_back":
			continue
		case "prepared":
			if err := rollbackFileRestore(manager, directory, journal); err != nil {
				return fmt.Errorf("pending restore recovery: %w", err)
			}
		default:
			return fmt.Errorf("unknown restore journal state")
		}
	}
	return nil
}
