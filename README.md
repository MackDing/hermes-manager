<p align="center">
  <img src="docs/screenshots/dashboard-dark.png" alt="HermesManager — Dracula Gradient Dashboard" width="100%" />
</p>

<h1 align="center">HermesManager</h1>

<p align="center">
  <strong>K8s-native control plane for Hermes Agent fleets</strong><br/>
  5-minute helm install · Multi-driver runtime · Policy + audit built-in
</p>

<p align="center">
  <a href="https://github.com/MackDing/HermesManager/releases/tag/v1.1.0"><img src="https://img.shields.io/badge/release-v1.1.0-bd93f9?style=flat-square" alt="v1.1.0"/></a>
  <a href="https://github.com/MackDing/HermesManager/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/MackDing/HermesManager/ci.yml?branch=main&style=flat-square&color=50fa7b" alt="CI"/></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-ff79c6?style=flat-square" alt="MIT"/></a>
  <img src="https://img.shields.io/badge/go-1.24-8be9fd?style=flat-square" alt="Go 1.24"/>
</p>

---

```bash
helm install hermesmanager oci://ghcr.io/mackding/charts/hermesmanager --version 1.1.0
```

> **Note:** v1.0 ships with single-admin auth. Intended for dev clusters and internal teams, not public-facing production without a reverse proxy. OIDC/SSO is planned for v1.1.

## What It Does

- **Fleet management** -- submit tasks to a pool of Hermes agents running as K8s Jobs, Docker containers, or local processes. Round-robin and lowest-load scheduling out of the box.
- **Policy and audit** -- every agent action (LLM calls, tool invocations, completions) is written to a JSONB audit log. A YAML policy file gates what models, users, and teams are allowed.
- **Skill sharing** -- declare skills in YAML, mount them via Helm, and they appear in the registry. Hot-reload across replicas via Postgres LISTEN/NOTIFY.

## Architecture

```
                          +-----------------------------------------+
                          |     hermesmanager  (single Go binary)   |
                          |                                         |
      Slack               |  REST API  ----+---- Policy Engine      |
      /hermes run  ------>|                |                        |
      /hermes status ---->|  Scheduler ----+---- Skill Registry     |
                          |       |                                 |
      React SPA           |       |        Slack Gateway            |
      (embedded) -------->|       |                                 |
                          +-------|-----+-----------+---------------+
                                  |     |           |
                 +----------------+     |           |
                 |                      |           |
         +-------v--------+    +-------v--+   +---v-----------+
         | K8s Jobs        |    | Docker   |   | Local process |
         | (agent pods)    |    | containers|   |               |
         | Label-selected  |    |          |   |               |
         | informer        |    |          |   |               |
         +--------+--------+    +----+-----+   +-------+-------+
                  |                   |                 |
                  +------- POST /v1/events ------------+
                  |        (per-runtime callback URL)   |
                  |                                     |
         +--------v-------------------------------------v-------+
         |  PostgreSQL 16 (via CloudNativePG)                   |
         |  - skills, tasks, events (JSONB + GIN), agent_tokens |
         |  - LISTEN/NOTIFY for hot reload                      |
         +------------------------------------------------------+
```

Each runtime driver computes its own callback URL so agents always know how to reach the control plane:
- **local:** `http://127.0.0.1:{PORT}/v1/events`
- **docker:** `http://host.docker.internal:{PORT}/v1/events`
- **k8s:** `http://hermesmanager.{NAMESPACE}.svc.cluster.local:8080/v1/events`

## Quick Start

### Prerequisites

- Kubernetes cluster (k3s, kind, or minikube)
- Helm 3.12+

### Install

```bash
helm install hermesmanager oci://ghcr.io/mackding/charts/hermesmanager --version 1.1.0
```

On a warm-cache cluster (images pre-pulled), the control plane is ready in under 5 minutes. Cold-start (first image pull) takes 8-10 minutes; use `kind load docker-image` to pre-populate.

### Access the Web UI

```bash
kubectl port-forward svc/hermesmanager 8080:8080
open http://localhost:8080
```

The admin password is printed in Helm's `NOTES.txt` output. If you set it explicitly:

```bash
helm install hermesmanager oci://ghcr.io/mackding/charts/hermesmanager \
  --set adminPassword=changeme --version 1.1.0
```

### Submit a Task

```bash
curl -X POST http://localhost:8080/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "skill_name": "hello-skill",
    "parameters": {"name": "world"},
    "runtime": "k8s"
  }'
```

### View Events

```bash
curl http://localhost:8080/v1/events | jq .
```

## Web UI

![Dashboard — Dark Mode (Dracula Gradient)](docs/screenshots/dashboard-dark.png)

![Dashboard — Light Mode](docs/screenshots/dashboard-light.png)

## Configuration

Key Helm values:

| Value | Default | Description |
|-------|---------|-------------|
| `image.repository` | `ghcr.io/mackding/hermesmanager` | Control plane container image |
| `image.tag` | `v1.1.0` | Image tag |
| `postgres.enabled` | `true` | Deploy PostgreSQL via CloudNativePG |
| `postgres.dsn` | (generated) | Connection string (set to use external Postgres) |
| `slack.enabled` | `false` | Enable the Slack bot gateway |
| `slack.token` | `""` | Slack bot token |
| `slack.signingSecret` | `""` | Slack request signing secret |
| `adminPassword` | (auto-generated) | Single admin password (bcrypt-hashed) |
| `watchNamespace` | Release namespace | Namespace for K8s Job informer (label-selected) |

## Development

### Run Locally

```bash
# Backend (requires a running Postgres instance)
export DATABASE_URL="postgres://user:pass@localhost:5432/hermesmanager?sslmode=disable"
go run cmd/hermesmanager/main.go

# Frontend (separate terminal)
cd web && npm install && npm run dev
```

### Tests

```bash
go test ./... -cover
```

### Build

```bash
go build -o hermesmanager ./cmd/hermesmanager/
```

## Project Structure

```
hermesmanager/
├── cmd/
│   └── hermesmanager/main.go         # Single binary entrypoint
├── internal/
│   ├── api/                          # REST API handlers + router
│   ├── scheduler/                    # Task dispatch, round-robin / lowest-load
│   ├── policy/                       # YAML policy engine (deny/allow rules)
│   ├── storage/
│   │   ├── store.go                  # DB-agnostic Store interface
│   │   ├── postgres/                 # pgx/v5 implementation
│   │   └── migrations/               # SQL schema (001_init.up.sql)
│   ├── runtime/
│   │   ├── runtime.go                # Runtime interface + plugin registry
│   │   ├── local/                    # Local process driver
│   │   ├── docker/                   # Docker daemon driver
│   │   └── k8s/                      # K8s Job driver + label-selected informer
│   └── gateway/
│       └── slack/                    # Slack bot (/hermes status, /hermes run)
├── web/                              # React 19 SPA (embedded in Go binary)
├── deploy/
│   ├── helm/hermesmanager/           # Helm chart
│   └── examples/                     # Seed policy.yaml + hello-skill.yaml
├── images/
│   └── demo-agent/                   # Demo Hermes agent container
├── docs/
│   ├── AGENT_API.md                  # Agent <-> control plane protocol
│   ├── ARCHITECTURE.md               # System design and data flow
│   └── CONTRIBUTING.md               # How to contribute
└── .github/workflows/                # CI + release pipelines
```

## v0.1 Scope

### Included

- Single Go binary: API server + scheduler + policy engine + Slack gateway
- PostgreSQL 16 via CloudNativePG (JSONB events, LISTEN/NOTIFY hot reload)
- Three runtime backends: local, docker, k8s-job
- YAML policy engine with deny rules (model, user, team, tool, cost fields)
- Read-only skill registry (YAML source of truth, DB cache)
- Slack bot: `/hermes status`, `/hermes run <task>`
- React SPA: dashboard, skills list, events viewer
- Single admin password auth (bcrypt, Helm-configurable)
- Helm chart with 5-minute deploy target (warm cache)

### Planned for v0.2+

- OIDC/SSO/SAML authentication
- Multi-user RBAC
- External-managed Postgres (RDS, Cloud SQL, Aiven)
- pgvector semantic skill search
- GUI skill editor with version diffing
- Multi-cluster federation
- Agent-to-agent RPC
- OPA/Rego policy engine option
- Billing integration and SOC2 compliance export
- Daytona/Modal/Singularity/SSH runtime backends
- Agent replay UI
- Skill marketplace

## License

[MIT](LICENSE)

## Contributing

See [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) for development setup, coding standards, and PR process.
