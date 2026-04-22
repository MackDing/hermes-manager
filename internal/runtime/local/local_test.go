package local

import (
	"context"
	"os"
	"testing"

	"github.com/MackDing/hermes-manager/internal/runtime"
	"github.com/MackDing/hermes-manager/internal/storage"
)

func TestName(t *testing.T) {
	r := New()
	if got := r.Name(); got != "local" {
		t.Errorf("Name() = %q, want %q", got, "local")
	}
}

func TestCallbackURL_DefaultPort(t *testing.T) {
	// Ensure env is clean for this test.
	orig := os.Getenv(envPort)
	os.Unsetenv(envPort)
	defer func() {
		if orig != "" {
			os.Setenv(envPort, orig)
		}
	}()

	r := New()
	want := "http://127.0.0.1:8080/v1/events"
	if got := r.CallbackURL(); got != want {
		t.Errorf("CallbackURL() = %q, want %q", got, want)
	}
}

func TestCallbackURL_CustomPort(t *testing.T) {
	orig := os.Getenv(envPort)
	os.Setenv(envPort, "9999")
	defer func() {
		if orig != "" {
			os.Setenv(envPort, orig)
		} else {
			os.Unsetenv(envPort)
		}
	}()

	r := New()
	want := "http://127.0.0.1:9999/v1/events"
	if got := r.CallbackURL(); got != want {
		t.Errorf("CallbackURL() = %q, want %q", got, want)
	}
}

func TestStatus_InitialState(t *testing.T) {
	r := New()
	active, max, err := r.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() returned error: %v", err)
	}
	if active != 0 {
		t.Errorf("Status() active = %d, want 0", active)
	}
	if max != maxConcurrency {
		t.Errorf("Status() max = %d, want %d", max, maxConcurrency)
	}
}

func TestStatus_TracksActiveProcesses(t *testing.T) {
	r := New()

	// Verify interface compliance.
	var _ runtime.Runtime = r

	// Dispatch a real process (sleep) to verify active count increments.
	task := storage.Task{
		ID:        "test-task-1",
		SkillName: "test-skill",
		Parameters: map[string]any{
			"name": "world",
		},
		DeadlineSecs: 60,
	}
	skill := storage.Skill{
		Name:       "test-skill",
		Entrypoint: "sleep",
	}

	handle, err := r.Dispatch(context.Background(), task, skill)
	if err != nil {
		t.Fatalf("Dispatch() returned error: %v", err)
	}
	if handle.RuntimeName != "local" {
		t.Errorf("Handle.RuntimeName = %q, want %q", handle.RuntimeName, "local")
	}
	if handle.ExternalID == "" {
		t.Error("Handle.ExternalID is empty, want a PID string")
	}

	// Check that active count increased.
	active, max, err := r.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() returned error: %v", err)
	}
	if active != 1 {
		t.Errorf("Status() active = %d after Dispatch, want 1", active)
	}
	if max != maxConcurrency {
		t.Errorf("Status() max = %d, want %d", max, maxConcurrency)
	}
}

func TestRegister(t *testing.T) {
	// Verify the local driver registered itself via init().
	factory, err := runtime.Get("local")
	if err != nil {
		t.Fatalf("runtime.Get(\"local\") returned error: %v", err)
	}
	rt, err := factory()
	if err != nil {
		t.Fatalf("factory() returned error: %v", err)
	}
	if rt.Name() != "local" {
		t.Errorf("factory().Name() = %q, want %q", rt.Name(), "local")
	}
}
