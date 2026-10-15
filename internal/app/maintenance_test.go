package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"project-yume/internal/storage"
)

func TestStopRetainsPersistenceLeaseUntilFinalFlush(t *testing.T) {
	options := storage.PersistenceOptions{DataDir: t.TempDir()}
	p, err := storage.OpenPersistence(options)
	if err != nil {
		t.Fatal(err)
	}
	worker := storage.NewFlushWorker(time.Hour)
	entered, release := make(chan struct{}), make(chan struct{})
	worker.Register("state", func() error {
		close(entered)
		<-release
		return p.Snapshots.Save("memory/conversation_history.json", []byte(`{"sessions":{}}`))
	})
	worker.MarkDirty("state")
	components := &Components{Persistence: p, FlushWorker: worker}
	done := make(chan error, 1)
	go func() { done <- components.Stop() }()
	<-entered
	_, maintenanceErr := storage.MigrateJSONOffline(context.Background(), options)
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !errors.Is(maintenanceErr, storage.ErrPersistenceBusy) {
		t.Fatalf("maintenance raced final flush: %v", maintenanceErr)
	}
	if _, err := storage.MigrateJSONOffline(context.Background(), options); err != nil {
		t.Fatalf("maintenance after stop: %v", err)
	}
}
