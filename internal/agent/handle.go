package agent

import "sync"

// RuntimeHandle lets long-lived workers switch runtimes between turns safely.
type RuntimeHandle struct {
	mu      sync.RWMutex
	runtime *Runtime
}

func NewRuntimeHandle(runtime *Runtime) *RuntimeHandle {
	return &RuntimeHandle{runtime: runtime}
}

func (h *RuntimeHandle) Get() *Runtime {
	if h == nil {
		return nil
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.runtime
}

func (h *RuntimeHandle) Replace(runtime *Runtime) {
	if h == nil {
		return
	}
	h.mu.Lock()
	h.runtime = runtime
	h.mu.Unlock()
}
