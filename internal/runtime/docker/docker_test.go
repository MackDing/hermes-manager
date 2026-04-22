package docker

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/MackDing/hermes-manager/internal/runtime"
	"github.com/MackDing/hermes-manager/internal/storage"
)

func TestName(t *testing.T) {
	r := New()
	if got := r.Name(); got != "docker" {
		t.Errorf("Name() = %q, want %q", got, "docker")
	}
}

func TestCallbackURL_DefaultPort(t *testing.T) {
	orig := os.Getenv(envPort)
	os.Unsetenv(envPort)
	defer func() {
		if orig != "" {
			os.Setenv(envPort, orig)
		}
	}()

	r := New()
	want := "http://host.docker.internal:8080/v1/events"
	if got := r.CallbackURL(); got != want {
		t.Errorf("CallbackURL() = %q, want %q", got, want)
	}
}

func TestCallbackURL_CustomPort(t *testing.T) {
	orig := os.Getenv(envPort)
	os.Setenv(envPort, "3000")
	defer func() {
		if orig != "" {
			os.Setenv(envPort, orig)
		} else {
			os.Unsetenv(envPort)
		}
	}()

	r := New()
	want := "http://host.docker.internal:3000/v1/events"
	if got := r.CallbackURL(); got != want {
		t.Errorf("CallbackURL() = %q, want %q", got, want)
	}
}

func TestDispatch_NoDockerHost(t *testing.T) {
	orig := os.Getenv(envDocker)
	os.Unsetenv(envDocker)
	defer func() {
		if orig != "" {
			os.Setenv(envDocker, orig)
		}
	}()

	r := New()
	task := storage.Task{
		ID:        "test-task-1",
		SkillName: "test-skill",
	}
	skill := storage.Skill{
		Name:       "test-skill",
		Entrypoint: "echo",
	}

	_, err := r.Dispatch(context.Background(), task, skill)
	if err == nil {
		t.Fatal("Dispatch() should return error when DOCKER_HOST is not set")
	}
	if !errors.Is(err, runtime.ErrRuntimeUnavailable) {
		t.Errorf("Dispatch() error = %v, want ErrRuntimeUnavailable", err)
	}
}

func TestDispatch_WithDockerHost_StillStub(t *testing.T) {
	orig := os.Getenv(envDocker)
	os.Setenv(envDocker, "unix:///var/run/docker.sock")
	defer func() {
		if orig != "" {
			os.Setenv(envDocker, orig)
		} else {
			os.Unsetenv(envDocker)
		}
	}()

	r := New()
	task := storage.Task{
		ID:        "test-task-2",
		SkillName: "test-skill",
	}
	skill := storage.Skill{
		Name:       "test-skill",
		Entrypoint: "echo",
	}

	_, err := r.Dispatch(context.Background(), task, skill)
	if err == nil {
		t.Fatal("Dispatch() should return error (stub not yet implemented)")
	}
	if !errors.Is(err, runtime.ErrRuntimeUnavailable) {
		t.Errorf("Dispatch() error = %v, want ErrRuntimeUnavailable", err)
	}
}

func TestStatus_Stub(t *testing.T) {
	r := New()
	active, max, err := r.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() returned error: %v", err)
	}
	if active != 0 {
		t.Errorf("Status() active = %d, want 0", active)
	}
	if max != 5 {
		t.Errorf("Status() max = %d, want 5", max)
	}
}

func TestRegister(t *testing.T) {
	factory, err := runtime.Get("docker")
	if err != nil {
		t.Fatalf("runtime.Get(\"docker\") returned error: %v", err)
	}
	rt, err := factory()
	if err != nil {
		t.Fatalf("factory() returned error: %v", err)
	}
	if rt.Name() != "docker" {
		t.Errorf("factory().Name() = %q, want %q", rt.Name(), "docker")
	}
}

func TestInterfaceCompliance(t *testing.T) {
	var _ runtime.Runtime = &Runtime{}
}
