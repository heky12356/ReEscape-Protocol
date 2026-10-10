package eventlog

import (
	"context"
	"fmt"
	"sync"
	"time"

	"project-yume/internal/utils"
)

// BlockingHook runs before events are persisted. Returning an error rejects
// the append, which lets policy hooks stop a write before it becomes a fact.
type BlockingHook func(context.Context, Event) error

// ObserveHook runs after events are persisted. It must not affect the result
// of the append operation.
type ObserveHook func(context.Context, Event) error

type HookOptions struct {
	Timeout      time.Duration
	ErrorHandler func(error)
}

// Dispatcher owns the two hook layers and isolates hook failures and panics.
type Dispatcher struct {
	mu           sync.RWMutex
	blocking     []BlockingHook
	observe      []ObserveHook
	timeout      time.Duration
	errorHandler func(error)
}

func NewDispatcher(options HookOptions) *Dispatcher {
	if options.Timeout <= 0 {
		options.Timeout = 2 * time.Second
	}
	if options.ErrorHandler == nil {
		options.ErrorHandler = func(err error) { utils.Warn("event hook failed: %v", err) }
	}
	return &Dispatcher{timeout: options.Timeout, errorHandler: options.ErrorHandler}
}

func (d *Dispatcher) AddBlocking(hook BlockingHook) {
	if d == nil || hook == nil {
		return
	}
	d.mu.Lock()
	d.blocking = append(d.blocking, hook)
	d.mu.Unlock()
}

func (d *Dispatcher) AddObserve(hook ObserveHook) {
	if d == nil || hook == nil {
		return
	}
	d.mu.Lock()
	d.observe = append(d.observe, hook)
	d.mu.Unlock()
}

func (d *Dispatcher) Before(ctx context.Context, events []Event) error {
	if d == nil {
		return nil
	}
	d.mu.RLock()
	hooks := append([]BlockingHook(nil), d.blocking...)
	timeout := d.timeout
	d.mu.RUnlock()
	for _, event := range events {
		for _, hook := range hooks {
			hookCtx, cancel := context.WithTimeout(ctx, timeout)
			err := callBlockingHook(hookCtx, hook, event)
			cancel()
			if err != nil {
				return fmt.Errorf("event blocking hook: %w", err)
			}
		}
	}
	return nil
}

func (d *Dispatcher) After(events []Event) {
	if d == nil || len(events) == 0 {
		return
	}
	d.mu.RLock()
	hooks := append([]ObserveHook(nil), d.observe...)
	timeout := d.timeout
	d.mu.RUnlock()
	if len(hooks) == 0 {
		return
	}
	go func() {
		for _, event := range events {
			for _, hook := range hooks {
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				err := callObserveHook(ctx, hook, event)
				cancel()
				if err != nil {
					d.report(err)
				}
			}
		}
	}()
}

func (d *Dispatcher) report(err error) {
	if err == nil {
		return
	}
	d.mu.RLock()
	handler := d.errorHandler
	d.mu.RUnlock()
	if handler != nil {
		defer func() { _ = recover() }()
		handler(err)
	}
}

func callBlockingHook(ctx context.Context, hook BlockingHook, event Event) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic: %v", recovered)
		}
	}()
	return hook(ctx, event)
}

func callObserveHook(ctx context.Context, hook ObserveHook, event Event) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("event observe hook panic: %v", recovered)
		} else if err != nil {
			err = fmt.Errorf("event observe hook: %w", err)
		}
	}()
	return hook(ctx, event)
}

// HookedStore keeps Store consumers independent from the hook implementation.
type HookedStore struct {
	Store
	dispatcher *Dispatcher
}

func NewHookedStore(store Store, dispatcher *Dispatcher) *HookedStore {
	return &HookedStore{Store: store, dispatcher: dispatcher}
}

func (s *HookedStore) Append(ctx context.Context, events ...Event) error {
	prepared := normalizeEvents(events)
	if len(prepared) == 0 {
		return nil
	}
	if s.dispatcher != nil {
		if err := s.dispatcher.Before(ctx, prepared); err != nil {
			return err
		}
	}
	if err := s.Store.Append(ctx, prepared...); err != nil {
		return err
	}
	if s.dispatcher != nil {
		s.dispatcher.After(prepared)
	}
	return nil
}
