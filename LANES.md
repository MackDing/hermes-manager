# Lane Boundary Map — HermesManager v0.1

Each directory is owned by exactly one lane. Subagents may ONLY create/edit files
in their owned directories. Violations will cause merge conflicts.

| Directory | Owner Lane | Notes |
|-----------|-----------|-------|
| `cmd/hermesmanager/` | F0 (FROZEN) | No subagent touches main.go |
| `internal/storage/store.go` | F0 (FROZEN) | Interface — no modifications |
| `internal/storage/migrations/` | F0 (FROZEN) | Schema — no modifications |
| `internal/storage/postgres/` | L4 | Postgres Store impl |
| `internal/runtime/runtime.go` | F0 (FROZEN) | Interface + registry — no modifications |
| `internal/runtime/local/` | L2 | Local process driver |
| `internal/runtime/docker/` | L2 | Docker daemon driver |
| `internal/runtime/k8s/` | L1 | K8s Job driver + informer |
| `internal/api/` | Shared (F0 stubs, filled by L4/L1) | Careful: only add handlers, don't modify router.go structure |
| `internal/policy/` | L3 | YAML policy engine |
| `internal/gateway/slack/` | L5 | Slack bot |
| `internal/scheduler/` | L1 | Task dispatch + routing logic |
| `web/` | L6 | Entire React SPA |
| `deploy/helm/` | L7 | Helm chart |
| `deploy/examples/` | L1 (hello-skill.yaml), L3 (policy.yaml) | Seed files |
| `.github/workflows/` | L7 | CI/CD |
| `images/demo-agent/` | L1 | Demo agent container |
| `docs/` | L8 (except AGENT_API.md = F0) | Documentation |
| `scripts/` | F0 | Lane enforcement |
| `design-system/` | FROZEN | No implementation changes |
| `go.mod` / `go.sum` | ALL (orchestrator resolves) | `go mod tidy` after each merge |

## Shared files protocol

- `go.mod` / `go.sum`: Every lane may add dependencies. Orchestrator runs `go mod tidy` after each merge.
- `internal/api/router.go`: Lanes may ADD route handlers but must not rename existing routes or change the mux setup.
- `deploy/helm/hermes-manager/values.yaml`: Only L7 writes. Other lanes create `HELM_VALUES_REQUEST.md` in their worktree root.

## Interface change protocol

If any subagent needs a method that doesn't exist on `Store` or `Runtime`:

1. Create `REQUEST_INTERFACE_CHANGE.md` in worktree root
2. Describe: what method, what signature, why it's needed
3. STOP and wait for orchestrator to add the method + update L4 impl
