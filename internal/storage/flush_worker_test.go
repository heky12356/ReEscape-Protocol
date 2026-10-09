package storage

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestFlushWorkerStopBeforeRunFlushesDirtyTask(t *testing.T) {
	worker := NewFlushWorker(time.Hour)
	var calls atomic.Int32
	worker.Register("state", func() error {
		calls.Add(1)
		return nil
	})
	worker.MarkDirty("state")
	if err := worker.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("flush calls = %d, want 1", got)
	}
	if err := worker.Stop(); err != nil {
		t.Fatalf("second stop: %v", err)
	}
}

func TestFlushWorkerStopReturnsFlushError(t *testing.T) {
	worker := NewFlushWorker(time.Hour)
	expected := errors.New("disk full")
	worker.Register("state", func() error { return expected })
	worker.MarkDirty("state")
	if err := worker.Stop(); !errors.Is(err, expected) {
		t.Fatalf("stop error = %v, want %v", err, expected)
	}
}

func TestFlushWorkerContextCancellationFlushesDirtyTask(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	worker := NewFlushWorker(time.Hour)
	flushed := make(chan struct{}, 1)
	worker.Register("state", func() error {
		flushed <- struct{}{}
		return nil
	})
	worker.MarkDirty("state")
	go worker.Run(ctx)
	cancel()
	select {
	case <-flushed:
	case <-time.After(time.Second):
		t.Fatal("worker did not perform final flush")
	}
	if err := worker.Stop(); err != nil {
		t.Fatalf("stop after context cancellation: %v", err)
	}
}
