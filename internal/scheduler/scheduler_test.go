package scheduler

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/hermesmanager/hermesmanager/internal/runtime"
	"github.com/hermesmanager/hermesmanager/internal/storage"
)

// --- Mock runtime ---

type mockRuntime struct {
	name       string
	dispatched []storage.Task
	dispatchFn func(ctx context.Context, task storage.Task, skill storage.Skill) (runtime.Handle, error)
	active     int
	max        int
	statusErr  error
}

func (m *mockRuntime) Name() string { return m.name }

func (m *mockRuntime) CallbackURL() string {
	return "http://mock:" + m.name + "/v1/events"
}

func (m *mockRuntime) Dispatch(ctx context.Context, task storage.Task, skill storage.Skill) (runtime.Handle, error) {
	m.dispatched = append(m.dispatched, task)
	if m.dispatchFn != nil {
		return m.dispatchFn(ctx, task, skill)
	}
	return runtime.Handle{
		RuntimeName: m.name,
		ExternalID:  "mock-" + task.ID,
	}, nil
}

func (m *mockRuntime) Status(_ context.Context) (int, int, error) {
	return m.active, m.max, m.statusErr
}

// --- Mock store ---

type mockStore struct {
	updatedStates map[string]storage.TaskState
	updateErr     error
}

func newMockStore() *mockStore {
	return &mockStore{
		updatedStates: make(map[string]storage.TaskState),
	}
}

func (s *mockStore) UpsertSkill(_ context.Context, _ storage.Skill) error             { return nil }
func (s *mockStore) GetSkill(_ context.Context, _ string) (*storage.Skill, error)      { return nil, nil }
func (s *mockStore) ListSkills(_ context.Context) ([]storage.Skill, error)             { return nil, nil }
func (s *mockStore) CreateTask(_ context.Context, _ storage.Task) error                { return nil }
func (s *mockStore) GetTask(_ context.Context, _ string) (*storage.Task, error)        { return nil, nil }
func (s *mockStore) ListTasks(_ context.Context, _ storage.TaskFilter) ([]storage.Task, error) {
	return nil, nil
}
func (s *mockStore) UpdateTaskState(_ context.Context, id string, state storage.TaskState) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.updatedStates[id] = state
	return nil
}
func (s *mockStore) AppendEvent(_ context.Context, _ storage.Event) error { return nil }
func (s *mockStore) ListEvents(_ context.Context, _ storage.EventFilter) ([]storage.Event, error) {
	return nil, nil
}
func (s *mockStore) StoreAgentToken(_ context.Context, _ string, _ []byte) error      { return nil }
func (s *mockStore) VerifyAgentToken(_ context.Context, _ string, _ []byte) (bool, error) {
	return false, nil
}
func (s *mockStore) RevokeAgentToken(_ context.Context, _ string) error { return nil }
func (s *mockStore) Migrate(_ context.Context) error                    { return nil }
func (s *mockStore) Ping(_ context.Context) error                       { return nil }
func (s *mockStore) Close() error                                       { return nil }

func newTestTask(rt string) storage.Task {
	return storage.Task{
		ID:        "task-001",
		SkillName: "hello-skill",
		Parameters: map[string]any{
			"name": "world",
		},
		Runtime: rt,
		State:   storage.TaskStatePending,
	}
}

func newTestSkill() storage.Skill {
	return storage.Skill{
		Name:    "hello-skill",
		Version: "0.1.0",
	}
}

func TestDispatchCallsRuntime(t *testing.T) {
	mr := &mockRuntime{name: "test-rt", max: 10}
	ms := newMockStore()

	sched := NewScheduler(map[string]runtime.Runtime{"test-rt": mr}, ms)
	ctx := context.Background()

	task := newTestTask("test-rt")
	skill := newTestSkill()

	handle, err := sched.Dispatch(ctx, task, skill)
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}

	if len(mr.dispatched) != 1 {
		t.Fatalf("expected 1 dispatch call, got %d", len(mr.dispatched))
	}
	if mr.dispatched[0].ID != task.ID {
		t.Errorf("dispatched task ID = %q, want %q", mr.dispatched[0].ID, task.ID)
	}
	if handle.RuntimeName != "test-rt" {
		t.Errorf("handle.RuntimeName = %q, want %q", handle.RuntimeName, "test-rt")
	}
}

func TestDispatchUpdatesTaskState(t *testing.T) {
	mr := &mockRuntime{name: "test-rt", max: 10}
	ms := newMockStore()

	sched := NewScheduler(map[string]runtime.Runtime{"test-rt": mr}, ms)
	ctx := context.Background()

	task := newTestTask("test-rt")
	skill := newTestSkill()

	_, err := sched.Dispatch(ctx, task, skill)
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}

	state, ok := ms.updatedStates[task.ID]
	if !ok {
		t.Fatal("expected task state to be updated")
	}
	if state != storage.TaskStateRunning {
		t.Errorf("task state = %q, want %q", state, storage.TaskStateRunning)
	}
}

func TestDispatchRuntimeError(t *testing.T) {
	mr := &mockRuntime{
		name: "fail-rt",
		max:  10,
		dispatchFn: func(_ context.Context, _ storage.Task, _ storage.Skill) (runtime.Handle, error) {
			return runtime.Handle{}, fmt.Errorf("kaboom")
		},
	}
	ms := newMockStore()

	sched := NewScheduler(map[string]runtime.Runtime{"fail-rt": mr}, ms)
	ctx := context.Background()

	task := newTestTask("fail-rt")
	skill := newTestSkill()

	_, err := sched.Dispatch(ctx, task, skill)
	if err == nil {
		t.Fatal("expected error from Dispatch")
	}

	// Should not update state on dispatch failure.
	if _, ok := ms.updatedStates[task.ID]; ok {
		t.Error("task state should not be updated on dispatch failure")
	}
}

func TestUnknownRuntimeReturnsError(t *testing.T) {
	ms := newMockStore()
	sched := NewScheduler(map[string]runtime.Runtime{}, ms)
	ctx := context.Background()

	task := newTestTask("nonexistent")
	skill := newTestSkill()

	_, err := sched.Dispatch(ctx, task, skill)
	if err == nil {
		t.Fatal("expected error for unknown runtime")
	}
}

func TestSelectRuntimeRoundRobin(t *testing.T) {
	rt1 := &mockRuntime{name: "rt-a", max: 10}
	rt2 := &mockRuntime{name: "rt-b", max: 10}
	ms := newMockStore()

	sched := NewScheduler(map[string]runtime.Runtime{
		"rt-a": rt1,
		"rt-b": rt2,
	}, ms)
	ctx := context.Background()

	// Reset the round-robin index to get deterministic behavior.
	sched.rrIndex = atomic.Uint64{}

	seen := make(map[string]int)
	for i := 0; i < 10; i++ {
		rt, err := sched.SelectRuntime(ctx, "round-robin")
		if err != nil {
			t.Fatalf("SelectRuntime() error = %v", err)
		}
		seen[rt.Name()]++
	}

	// With 2 runtimes and 10 calls, each should get exactly 5.
	if seen["rt-a"] != 5 || seen["rt-b"] != 5 {
		t.Errorf("round-robin distribution: %v, want each=5", seen)
	}
}

func TestSelectRuntimeLowestLoad(t *testing.T) {
	rtHigh := &mockRuntime{name: "high-load", active: 8, max: 10}
	rtLow := &mockRuntime{name: "low-load", active: 1, max: 10}
	ms := newMockStore()

	sched := NewScheduler(map[string]runtime.Runtime{
		"high-load": rtHigh,
		"low-load":  rtLow,
	}, ms)
	ctx := context.Background()

	rt, err := sched.SelectRuntime(ctx, "lowest-load")
	if err != nil {
		t.Fatalf("SelectRuntime() error = %v", err)
	}
	if rt.Name() != "low-load" {
		t.Errorf("SelectRuntime() = %q, want %q", rt.Name(), "low-load")
	}
}

func TestSelectRuntimeLowestLoadAllEqual(t *testing.T) {
	rt1 := &mockRuntime{name: "rt-a", active: 5, max: 10}
	rt2 := &mockRuntime{name: "rt-b", active: 5, max: 10}
	ms := newMockStore()

	sched := NewScheduler(map[string]runtime.Runtime{
		"rt-a": rt1,
		"rt-b": rt2,
	}, ms)
	ctx := context.Background()

	// When loads are equal, any runtime is acceptable.
	rt, err := sched.SelectRuntime(ctx, "lowest-load")
	if err != nil {
		t.Fatalf("SelectRuntime() error = %v", err)
	}
	if rt.Name() != "rt-a" && rt.Name() != "rt-b" {
		t.Errorf("SelectRuntime() = %q, want rt-a or rt-b", rt.Name())
	}
}

func TestSelectRuntimeDirectLookup(t *testing.T) {
	rt := &mockRuntime{name: "specific", max: 10}
	ms := newMockStore()

	sched := NewScheduler(map[string]runtime.Runtime{"specific": rt}, ms)
	ctx := context.Background()

	got, err := sched.SelectRuntime(ctx, "specific")
	if err != nil {
		t.Fatalf("SelectRuntime() error = %v", err)
	}
	if got.Name() != "specific" {
		t.Errorf("SelectRuntime() = %q, want %q", got.Name(), "specific")
	}
}

func TestSelectRuntimeUnknownPolicy(t *testing.T) {
	ms := newMockStore()
	sched := NewScheduler(map[string]runtime.Runtime{}, ms)
	ctx := context.Background()

	_, err := sched.SelectRuntime(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown runtime/policy")
	}
}

func TestSelectRuntimeNoRuntimes(t *testing.T) {
	ms := newMockStore()
	sched := NewScheduler(map[string]runtime.Runtime{}, ms)
	ctx := context.Background()

	_, err := sched.SelectRuntime(ctx, "round-robin")
	if err == nil {
		t.Fatal("expected error when no runtimes available")
	}
}
