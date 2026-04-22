# Contributing to HermesManager

## Local Development Setup

### Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.21+ | Backend |
| Node.js | 22+ | Frontend build |
| Docker | 24+ | Container runtime, Docker driver testing |
| kind | 0.20+ | Local K8s cluster for integration tests |
| Helm | 3.12+ | Chart testing |
| PostgreSQL | 16+ | Local database (or use Docker) |

### Clone and Build

```bash
git clone https://github.com/MackDing/hermes-manager.git
cd hermesmanager
go mod download
go build ./cmd/hermesmanager/
```

### Start PostgreSQL

```bash
docker run -d --name hermesmanager-pg \
  -e POSTGRES_USER=hermesmanager \
  -e POSTGRES_PASSWORD=dev \
  -e POSTGRES_DB=hermesmanager \
  -p 5432:5432 \
  postgres:16-alpine
```

### Run the Backend

```bash
export DATABASE_URL="postgres://hermesmanager:dev@localhost:5432/hermesmanager?sslmode=disable"
go run cmd/hermesmanager/main.go
```

The API server starts on `:8080`. Verify with:

```bash
curl http://localhost:8080/healthz
```

### Run the Frontend

```bash
cd web
npm install
npm run dev
```

The dev server proxies API requests to `localhost:8080`.

## Running Tests

### All Tests

```bash
go test ./... -cover
```

### Specific Package

```bash
go test ./internal/policy/ -v -cover
go test ./internal/scheduler/ -v -cover
go test ./internal/runtime/local/ -v -cover
```

### K8s Integration Tests

These require a running kind cluster:

```bash
kind create cluster --name hermesmanager-test
go test ./internal/runtime/k8s/ -v -tags=integration
kind delete cluster --name hermesmanager-test
```

### Frontend Tests

```bash
cd web
npm test
```

## Building

### Go Binary

```bash
go build -o hermesmanager ./cmd/hermesmanager/
```

### Container Image

```bash
docker build -t hermesmanager:dev .
```

### Helm Chart (local testing)

```bash
kind create cluster --name hermesmanager-dev
kind load docker-image hermesmanager:dev --name hermesmanager-dev
helm install hermesmanager deploy/helm/hermes-manager/ --set image.tag=dev
```

## How to Add a New Runtime Driver

Runtime drivers live in sub-packages under `internal/runtime/`. Each driver implements the `Runtime` interface defined in `internal/runtime/runtime.go` (FROZEN -- do not modify).

### Steps

1. Create a new package: `internal/runtime/yourdriver/`

2. Implement the `Runtime` interface:

```go
package yourdriver

import (
    "context"
    "github.com/MackDing/hermes-manager/internal/runtime"
    "github.com/MackDing/hermes-manager/internal/storage"
)

type Driver struct {
    // driver-specific fields
}

func (d *Driver) Name() string { return "yourdriver" }

func (d *Driver) CallbackURL() string {
    // Return the URL agents on this runtime use to reach the control plane
    return "http://..."
}

func (d *Driver) Dispatch(ctx context.Context, task storage.Task, skill storage.Skill) (runtime.Handle, error) {
    // Create and start the agent workload
    // Return a Handle with the runtime name and an opaque external ID
    return runtime.Handle{RuntimeName: "yourdriver", ExternalID: "..."}, nil
}

func (d *Driver) Status(ctx context.Context) (int, int, error) {
    // Return (active_count, max_capacity, error)
    return 0, 10, nil
}
```

3. Register via `init()`:

```go
func init() {
    runtime.Register("yourdriver", func() (runtime.Runtime, error) {
        return &Driver{}, nil
    })
}
```

4. Import the package in `cmd/hermesmanager/main.go` (blank import):

```go
import _ "github.com/MackDing/hermes-manager/internal/runtime/yourdriver"
```

5. Write tests. At minimum, unit tests with a mock workload. See `internal/runtime/local/local_test.go` for the pattern.

### Interface Change Protocol

If your driver needs a method that does not exist on `Runtime` or `Store`:

1. Create `REQUEST_INTERFACE_CHANGE.md` in the repository root
2. Describe: what method, what signature, why it is needed
3. Wait for the change to be approved and merged before depending on it

## How to Add a New Skill

Skills are YAML files loaded from `deploy/examples/` (or any directory mounted into the control plane at `/etc/hermesmanager/skills/`).

### Steps

1. Create a YAML file in `deploy/examples/`:

```yaml
apiVersion: hermesmanager.io/v1alpha1
kind: Skill
metadata:
  name: your-skill
  version: "0.1.0"
spec:
  description: "What this skill does."
  entrypoint: "skills/your-skill/main.py"
  parameters:
    - name: input
      type: string
      required: true
  required_tools: ["llm"]
  required_models: ["gpt-4o-mini"]
```

2. Mount via Helm values (or place in the skills directory if running locally):

```yaml
skills:
  extraFiles:
    your-skill.yaml: |
      apiVersion: hermesmanager.io/v1alpha1
      kind: Skill
      ...
```

3. The control plane loads the skill on startup and caches it in PostgreSQL. Hot reload is triggered via Postgres LISTEN/NOTIFY when the file changes.

## Code Style

- Run `go fmt` before committing
- Run `go vet ./...` for static analysis
- Follow existing patterns in the codebase
- Keep functions under 50 lines
- Keep files under 800 lines
- Handle errors explicitly; do not silently discard them
- Depend on interfaces (`Store`, `Runtime`), not concrete implementations
- Use immutable patterns where possible: create new structs instead of mutating

### Naming

- Packages: lowercase, single word where possible
- Exported types and functions: PascalCase
- Unexported: camelCase
- Constants: PascalCase (Go convention) or UPPER_SNAKE_CASE for sentinel values
- Test files: `*_test.go` in the same package

## PR Process

1. **One feature per PR.** Keep changes focused and reviewable.
2. **Tests required.** New functionality must include tests. Target 80%+ coverage for new code.
3. **Description of what and why.** The PR body should explain the change and the motivation, not just list files touched.
4. **Do not modify FROZEN interfaces.** `internal/storage/store.go` and `internal/runtime/runtime.go` are frozen. If you need a new method, follow the interface change protocol above.
5. **Run checks before submitting:**

```bash
go fmt ./...
go vet ./...
go test ./... -cover
```

6. **Conventional commit messages:**

```
feat: add SSH runtime driver
fix: handle nil skill in scheduler dispatch
refactor: extract token generation into shared helper
docs: update ARCHITECTURE.md with new runtime
test: add integration tests for Docker driver
chore: bump pgx to v5.6.0
```

## Lane Ownership

HermesManager uses lane-based ownership to prevent merge conflicts during parallel development. See `LANES.md` for the full map. Key rules:

- Only edit files in directories you own
- `go.mod` / `go.sum` are shared; the orchestrator resolves conflicts
- `internal/api/router.go` is shared: add handlers, but do not restructure the router
- Documentation (`docs/`) is owned by Lane L8, except `docs/AGENT_API.md` which is frozen
