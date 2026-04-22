// Package docker implements the Docker runtime driver for HermesManager.
//
// v0.1 is a stub: Dispatch returns ErrRuntimeUnavailable unless DOCKER_HOST is set.
// Real Docker SDK integration is deferred to avoid adding heavy dependencies in this lane.
package docker

import (
	"context"
	"fmt"
	"os"

	"github.com/MackDing/hermes-manager/internal/runtime"
	"github.com/MackDing/hermes-manager/internal/storage"
)

const (
	defaultPort = "8080"
	envPort     = "HERMESMANAGER_PORT"
	envDocker   = "DOCKER_HOST"
)

func init() {
	runtime.Register("docker", func() (runtime.Runtime, error) {
		return New(), nil
	})
}

// Runtime is the Docker execution backend.
type Runtime struct {
	port string
}

// New creates a Docker Runtime. Port is read from HERMESMANAGER_PORT env or defaults to 8080.
func New() *Runtime {
	port := os.Getenv(envPort)
	if port == "" {
		port = defaultPort
	}
	return &Runtime{port: port}
}

// Name returns the runtime identifier.
func (r *Runtime) Name() string { return "docker" }

// CallbackURL returns the control-plane events endpoint reachable from
// inside a Docker container. Uses host.docker.internal which is the standard
// Docker Desktop alias; on Linux dockerd this requires --add-host=host.docker.internal:host-gateway.
func (r *Runtime) CallbackURL() string {
	return "http://host.docker.internal:" + r.port + "/v1/events"
}

// Dispatch is a stub in v0.1. It returns ErrRuntimeUnavailable if the
// DOCKER_HOST env var is not set, indicating no Docker daemon is configured.
// When DOCKER_HOST is set, it still returns ErrRuntimeUnavailable because
// real Docker SDK integration is deferred.
func (r *Runtime) Dispatch(_ context.Context, _ storage.Task, _ storage.Skill) (runtime.Handle, error) {
	if os.Getenv(envDocker) == "" {
		return runtime.Handle{}, fmt.Errorf("%w: DOCKER_HOST not set", runtime.ErrRuntimeUnavailable)
	}
	return runtime.Handle{}, fmt.Errorf("%w: docker dispatch not yet implemented", runtime.ErrRuntimeUnavailable)
}

// Status returns the current load metric for scheduling decisions.
// Stub: always returns (0, 5, nil) since no real containers are tracked.
func (r *Runtime) Status(_ context.Context) (int, int, error) {
	return 0, 5, nil
}
