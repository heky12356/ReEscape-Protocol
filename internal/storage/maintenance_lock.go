package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var ErrPersistenceBusy = errors.New("persistence is in use; stop the bot before offline maintenance")

type persistenceLease struct{ files []*os.File }

func acquirePersistenceLease(options PersistenceOptions) (*persistenceLease, error) {
	root, err := filepath.Abs(options.DataDir)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(options.DataDir) == "" {
		root, err = filepath.Abs("./data")
		if err != nil {
			return nil, err
		}
	}
	paths := []string{filepath.Join(root, ".runtime.lock")}
	if options.Backend == "sqlite" {
		database := options.SQLitePath
		if database == "" {
			database = filepath.Join(root, "runtime.sqlite")
		}
		absolute, err := filepath.Abs(database)
		if err != nil {
			return nil, err
		}
		paths = append(paths, absolute+".runtime.lock")
	}
	sort.Strings(paths)
	lease := &persistenceLease{}
	for _, name := range paths {
		if err := rejectSymlinks(name); err != nil {
			lease.Close()
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
			lease.Close()
			return nil, err
		}
		file, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			lease.Close()
			return nil, err
		}
		if err := lockMaintenanceFile(file); err != nil {
			file.Close()
			lease.Close()
			return nil, fmt.Errorf("%w: %v", ErrPersistenceBusy, err)
		}
		lease.files = append(lease.files, file)
	}
	return lease, nil
}

func (l *persistenceLease) Close() error {
	if l == nil {
		return nil
	}
	var result error
	for i := len(l.files) - 1; i >= 0; i-- {
		result = errors.Join(result, l.files[i].Close())
	}
	l.files = nil
	// Do not unlink lock files: another process may already hold their inode.
	return result
}

func rejectSymlinks(name string) error {
	absolute, err := filepath.Abs(name)
	if err != nil {
		return err
	}
	for current := absolute; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic link is not permitted: %s", current)
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	return nil
}
