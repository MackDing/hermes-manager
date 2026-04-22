// Package scheduler implements task dispatch and runtime selection for HermesManager.
//
// The scheduler routes tasks to the appropriate runtime backend based on
// the task's runtime field or a selection policy (round-robin, lowest-load).
package scheduler

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/MackDing/hermes-manager/internal/runtime"
	"github.com/MackDing/hermes-manager/internal/storage"
)

// Scheduler dispatches tasks to runtime backends and updates task state via Store.
type Scheduler struct {
	runtimes map[string]runtime.Runtime
	store    storage.Store

	// round-robin state
	mu       sync.Mutex
	rrNames  []string
	rrIndex  atomic.Uint64
}

// NewScheduler creates a Scheduler with the given runtimes and store.
func NewScheduler(runtimes map[string]runtime.Runtime, store storage.Store) *Scheduler {
	names := make([]string, 0, len(runtimes))
	for name := range runtimes {
		names = append(names, name)
	}

	return &Scheduler{
		runtimes: runtimes,
		store:    store,
		rrNames:  names,
	}
}

// Dispatch sends a task to the runtime specified in task.Runtime, executes it,
// and updates the task state to "running" in the store.
func (s *Scheduler) Dispatch(ctx context.Context, task storage.Task, skill storage.Skill) (runtime.Handle, error) {
	rt, ok := s.runtimes[task.Runtime]
	if !ok {
		return runtime.Handle{}, fmt.Errorf("%w: %q", runtime.ErrRuntimeNotFound, task.Runtime)
	}

	handle, err := rt.Dispatch(ctx, task, skill)
	if err != nil {
		return runtime.Handle{}, fmt.Errorf("scheduler: dispatch on %q: %w", task.Runtime, err)
	}

	if err := s.store.UpdateTaskState(ctx, task.ID, storage.TaskStateRunning); err != nil {
		return handle, fmt.Errorf("scheduler: update task state: %w", err)
	}

	return handle, nil
}

// SelectRuntime chooses a runtime according to the given policy.
//
// Supported policies:
//   - "round-robin": cycles through all available runtimes
//   - "lowest-load": picks the runtime with the lowest active/max ratio
//   - any other string: treated as a runtime name (direct lookup)
func (s *Scheduler) SelectRuntime(ctx context.Context, policy string) (runtime.Runtime, error) {
	switch policy {
	case "round-robin":
		return s.selectRoundRobin()
	case "lowest-load":
		return s.selectLowestLoad(ctx)
	default:
		rt, ok := s.runtimes[policy]
		if !ok {
			return nil, fmt.Errorf("%w: %q", runtime.ErrRuntimeNotFound, policy)
		}
		return rt, nil
	}
}

func (s *Scheduler) selectRoundRobin() (runtime.Runtime, error) {
	if len(s.rrNames) == 0 {
		return nil, fmt.Errorf("%w: no runtimes registered", runtime.ErrRuntimeNotFound)
	}

	idx := s.rrIndex.Add(1) - 1
	name := s.rrNames[idx%uint64(len(s.rrNames))]
	return s.runtimes[name], nil
}

func (s *Scheduler) selectLowestLoad(ctx context.Context) (runtime.Runtime, error) {
	if len(s.runtimes) == 0 {
		return nil, fmt.Errorf("%w: no runtimes registered", runtime.ErrRuntimeNotFound)
	}

	var bestRT runtime.Runtime
	bestRatio := 2.0 // start above any possible ratio

	for _, rt := range s.runtimes {
		active, max, err := rt.Status(ctx)
		if err != nil {
			continue // skip runtimes that fail to report status
		}
		if max <= 0 {
			continue
		}

		ratio := float64(active) / float64(max)
		if ratio < bestRatio {
			bestRatio = ratio
			bestRT = rt
		}
	}

	if bestRT == nil {
		return nil, fmt.Errorf("%w: no runtimes available", runtime.ErrRuntimeUnavailable)
	}

	return bestRT, nil
}
