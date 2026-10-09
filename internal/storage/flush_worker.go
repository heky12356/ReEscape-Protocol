package storage

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"project-yume/internal/metrics"
	"project-yume/internal/utils"
)

type FlushFunc func() error

type flushTask struct {
	name  string
	flush FlushFunc
	dirty bool
}

type FlushWorker struct {
	interval time.Duration

	mu       sync.Mutex
	tasks    map[string]*flushTask
	signal   chan struct{}
	stopCh   chan struct{}
	doneCh   chan struct{}
	stopOnce sync.Once
	runOnce  sync.Once
	started  bool
	stopped  bool
	stopErr  error
}

func NewFlushWorker(interval time.Duration) *FlushWorker {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	return &FlushWorker{
		interval: interval,
		tasks:    make(map[string]*flushTask),
		signal:   make(chan struct{}, 1),
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

func (w *FlushWorker) Register(name string, flush FlushFunc) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.tasks[name] = &flushTask{
		name:  name,
		flush: flush,
	}
}

func (w *FlushWorker) MarkDirty(name string) {
	w.mu.Lock()
	task := w.tasks[name]
	if task != nil {
		task.dirty = true
	}
	w.mu.Unlock()

	select {
	case w.signal <- struct{}{}:
	default:
	}
}

func (w *FlushWorker) Run(ctx context.Context) {
	started := false
	w.runOnce.Do(func() {
		w.mu.Lock()
		if !w.stopped {
			w.started = true
			started = true
		}
		w.mu.Unlock()
	})
	if !started {
		return
	}
	defer func() {
		w.mu.Lock()
		w.started = false
		w.stopped = true
		w.mu.Unlock()
		close(w.doneCh)
	}()

	var timer *time.Timer
	var timerCh <-chan time.Time

	resetTimer := func() {
		if timer == nil {
			timer = time.NewTimer(w.interval)
		} else {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(w.interval)
		}
		timerCh = timer.C
	}

	stopTimer := func() {
		if timer == nil {
			return
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timerCh = nil
	}

	for {
		select {
		case <-ctx.Done():
			stopTimer()
			w.setStopError(w.flushDirty())
			return
		case <-w.stopCh:
			stopTimer()
			w.setStopError(w.flushDirty())
			return
		case <-w.signal:
			resetTimer()
		case <-timerCh:
			w.flushDirty()
			stopTimer()
		}
	}
}

func (w *FlushWorker) Stop() error {
	w.stopOnce.Do(func() {
		w.mu.Lock()
		started := w.started
		alreadyStopped := w.stopped
		if !started {
			w.stopped = true
		}
		w.mu.Unlock()
		if alreadyStopped {
			return
		}
		if started {
			close(w.stopCh)
			<-w.doneCh
			return
		}
		w.setStopError(w.flushDirty())
		close(w.doneCh)
	})
	<-w.doneCh
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.stopErr
}

func (w *FlushWorker) setStopError(err error) {
	if err == nil {
		return
	}
	w.mu.Lock()
	w.stopErr = errors.Join(w.stopErr, err)
	w.mu.Unlock()
}

func (w *FlushWorker) flushDirty() error {
	w.mu.Lock()
	tasks := make([]*flushTask, 0, len(w.tasks))
	for _, task := range w.tasks {
		if task.dirty {
			task.dirty = false
			tasks = append(tasks, task)
		}
	}
	w.mu.Unlock()

	var flushErrors []error
	for _, task := range tasks {
		if err := task.flush(); err != nil {
			utils.Error("flush %s failed: %v", task.name, err)
			metrics.IncCounter(
				"bot_flush_total",
				"Total persistence flush attempts by task and result.",
				map[string]string{"name": task.name, "result": "error"},
			)
			w.mu.Lock()
			if existing := w.tasks[task.name]; existing != nil {
				existing.dirty = true
			}
			w.mu.Unlock()
			flushErrors = append(flushErrors, fmt.Errorf("flush %s: %w", task.name, err))
			continue
		}
		metrics.IncCounter(
			"bot_flush_total",
			"Total persistence flush attempts by task and result.",
			map[string]string{"name": task.name, "result": "ok"},
		)
	}
	return errors.Join(flushErrors...)
}
