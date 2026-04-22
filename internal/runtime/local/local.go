// Package local implements the local-process runtime driver for HermesManager.
//
// The local driver spawns agent processes on the same host as the control plane.
// It is intended for development and single-machine deployments.
package local

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/MackDing/hermes-manager/internal/runtime"
	"github.com/MackDing/hermes-manager/internal/storage"
)

const (
	defaultPort    = "8080"
	envPort        = "HERMESMANAGER_PORT"
	maxConcurrency = 10
)

func init() {
	runtime.Register("local", func() (runtime.Runtime, error) {
		return New(), nil
	})
}

// Runtime is the local-process execution backend.
type Runtime struct {
	port    string
	active  atomic.Int64
	mu      sync.Mutex
	handles map[string]*os.Process // taskID → process
}

// New creates a local Runtime. Port is read from HERMESMANAGER_PORT env or defaults to 8080.
func New() *Runtime {
	port := os.Getenv(envPort)
	if port == "" {
		port = defaultPort
	}
	return &Runtime{
		port:    port,
		handles: make(map[string]*os.Process),
	}
}

// Name returns the runtime identifier.
func (r *Runtime) Name() string { return "local" }

// CallbackURL returns the control-plane events endpoint reachable from
// a locally spawned agent process.
func (r *Runtime) CallbackURL() string {
	return "http://127.0.0.1:" + r.port + "/v1/events"
}

// taskJSON is the on-disk representation of a task passed to the agent process.
type taskJSON struct {
	TaskID          string            `json:"task_id"`
	Skill           string            `json:"skill"`
	Parameters      map[string]any    `json:"parameters"`
	PolicyContext   storage.PolicyContext `json:"policy_context"`
	DeadlineSeconds int               `json:"deadline_seconds"`
}

// Dispatch spawns a subprocess to execute the given task.
//
// The agent process receives:
//   - env HERMESMANAGER_TASK_ID
//   - env HERMESMANAGER_CALLBACK_URL
//   - env HERMESMANAGER_AGENT_TOKEN (random per-task token)
//   - task.json written to a temp directory
//
// The subprocess runs the skill's Entrypoint with the temp dir as an argument.
func (r *Runtime) Dispatch(ctx context.Context, task storage.Task, skill storage.Skill) (runtime.Handle, error) {
	if int(r.active.Load()) >= maxConcurrency {
		return runtime.Handle{}, fmt.Errorf("%w: local driver at capacity (%d/%d)", runtime.ErrRuntimeUnavailable, r.active.Load(), maxConcurrency)
	}

	// Create temp directory for task data.
	tmpDir, err := os.MkdirTemp("", "hermesmanager-task-"+task.ID+"-")
	if err != nil {
		return runtime.Handle{}, fmt.Errorf("local: create temp dir: %w", err)
	}

	// Write task.json.
	tj := taskJSON{
		TaskID:          task.ID,
		Skill:           task.SkillName,
		Parameters:      task.Parameters,
		PolicyContext:    task.PolicyContext,
		DeadlineSeconds: task.DeadlineSecs,
	}
	taskData, err := json.Marshal(tj)
	if err != nil {
		return runtime.Handle{}, fmt.Errorf("local: marshal task json: %w", err)
	}
	taskPath := filepath.Join(tmpDir, "task.json")
	if err := os.WriteFile(taskPath, taskData, 0o600); err != nil {
		return runtime.Handle{}, fmt.Errorf("local: write task.json: %w", err)
	}

	// Generate a per-task agent token.
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return runtime.Handle{}, fmt.Errorf("local: generate agent token: %w", err)
	}
	agentToken := hex.EncodeToString(tokenBytes)

	// Build the subprocess command.
	cmd := exec.CommandContext(ctx, skill.Entrypoint, tmpDir)
	cmd.Env = append(os.Environ(),
		"HERMESMANAGER_TASK_ID="+task.ID,
		"HERMESMANAGER_CALLBACK_URL="+r.CallbackURL(),
		"HERMESMANAGER_AGENT_TOKEN="+agentToken,
	)
	cmd.Dir = tmpDir

	if err := cmd.Start(); err != nil {
		return runtime.Handle{}, fmt.Errorf("local: start process: %w", err)
	}

	pid := cmd.Process.Pid
	r.active.Add(1)

	r.mu.Lock()
	r.handles[task.ID] = cmd.Process
	r.mu.Unlock()

	// Monitor the process in the background to update active count.
	go func() {
		_ = cmd.Wait()
		r.active.Add(-1)
		r.mu.Lock()
		delete(r.handles, task.ID)
		r.mu.Unlock()
		// Best-effort cleanup of temp dir.
		_ = os.RemoveAll(tmpDir)
	}()

	return runtime.Handle{
		RuntimeName: "local",
		ExternalID:  strconv.Itoa(pid),
	}, nil
}

// Status returns the current load metric for scheduling decisions.
// Returns (active_count, max_capacity, error).
func (r *Runtime) Status(_ context.Context) (int, int, error) {
	return int(r.active.Load()), maxConcurrency, nil
}
