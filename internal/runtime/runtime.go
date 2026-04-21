// Package runtime defines the Runtime interface for agent execution backends.
//
// FROZEN after F0 — no subagent may add, remove, or modify methods.
// If a new method is needed, create REQUEST_INTERFACE_CHANGE.md in the worktree root.
package runtime

import (
	"context"
	"fmt"
	"sync"

	"github.com/hermesmanager/hermesmanager/internal/storage"
)

// Runtime is the interface each execution backend must implement.
// v0.1 ships: local, docker, k8s.
type Runtime interface {
	// Name returns the runtime identifier (e.g. "local", "docker", "k8s").
	Name() string

	// CallbackURL returns the control-plane events endpoint reachable from
	// agents running on THIS runtime. Per-driver because network topology differs:
	//   local  → http://127.0.0.1:{port}/v1/events
	//   docker → http://host.docker.internal:{port}/v1/events
	//   k8s    → http://hermesmanager.{ns}.svc.cluster.local:{port}/v1/events
	CallbackURL() string

	// Dispatch creates and starts an agent for the given task.
	// Returns an opaque runtime-specific handle for status queries.
	Dispatch(ctx context.Context, task storage.Task, skill storage.Skill) (Handle, error)

	// Status returns the current load metric for scheduling decisions.
	// Returns (active_count, max_capacity, error).
	Status(ctx context.Context) (int, int, error)
}

// Handle is an opaque reference to a running agent, returned by Dispatch.
type Handle struct {
	RuntimeName string `json:"runtime_name"`
	ExternalID  string `json:"external_id"` // PID, container ID, Job name, etc.
}

// --- Sentinel errors ---

var (
	ErrRuntimeUnavailable = fmt.Errorf("runtime: backend unavailable")
	ErrTaskTimeout        = fmt.Errorf("runtime: task exceeded deadline")
	ErrPolicyBlocked      = fmt.Errorf("runtime: task blocked by policy")
	ErrRuntimeNotFound    = fmt.Errorf("runtime: unknown runtime name")
)

// --- Plugin registry ---
// Subagents register their drivers here; main.go never needs to be modified.

var (
	mu       sync.RWMutex
	registry = make(map[string]Factory)
)

// Factory creates a Runtime instance. Called once at startup.
type Factory func() (Runtime, error)

// Register adds a runtime factory to the global registry.
// Call this in an init() function in each driver package.
func Register(name string, f Factory) {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("runtime: duplicate registration for %q", name))
	}
	registry[name] = f
}

// Build instantiates all registered runtimes.
// Returns a map of name → Runtime.
func Build() (map[string]Runtime, error) {
	mu.RLock()
	defer mu.RUnlock()

	runtimes := make(map[string]Runtime, len(registry))
	for name, factory := range registry {
		rt, err := factory()
		if err != nil {
			return nil, fmt.Errorf("runtime: failed to build %q: %w", name, err)
		}
		runtimes[name] = rt
	}
	return runtimes, nil
}

// Get returns a single registered runtime by name.
func Get(name string) (Factory, error) {
	mu.RLock()
	defer mu.RUnlock()
	f, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrRuntimeNotFound, name)
	}
	return f, nil
}
