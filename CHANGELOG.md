# Changelog

All notable changes to HermesManager are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.1.0] — 2026-04-22

Usability polish release. No breaking API changes.

### Added
- **CLI flags** — `hermesmanager --version` and `--help` now work; help prints all env vars with examples
- **`/readyz` endpoint** — readiness probe pings Postgres; returns 503 if database is unreachable. Liveness (`/healthz`) remains process-only. Helm deployment probes now split correctly
- **`/hermes help`** Slack command — lists all commands with examples; bare `/hermes` also shows help
- **Makefile** — `make build`, `make test`, `make lint`, `make dev` (starts Postgres + backend), `make helm-lint`
- **`docker-compose.yml`** — Postgres 16-alpine for local development with healthcheck
- **`docs/QUICKSTART.md`** — 5-step copy-paste guide from `helm install` to first task submission
- **`docs/TROUBLESHOOTING.md`** — 7 common errors with symptom/cause/fix
- **GitHub templates** — bug report, feature request, and PR templates
- **`scripts/install.sh`** — one-liner binary installer (resolves latest release, detects OS/arch)
- **React `EmptyState` component** — shared empty-state UI for all pages (activates when API returns empty data)
- **React `ErrorBoundary`** — catches render crashes with "Try again" recovery

### Improved
- **Startup error messages** — all `log.Fatal` calls include a `hint` field with actionable fix guidance
- **Startup log** — now includes `version` field for quick identification in container logs
- **Slack error messages** — raw errors no longer leak to users; friendly messages with next-step guidance instead
- **`values.yaml`** — every field annotated with purpose, default, constraints, and examples
- **`NOTES.txt`** — now guides users to wait for Postgres readiness before port-forwarding
- **`deploy/examples/`** — policy.yaml and hello-skill.yaml fully commented
- **Chart.yaml** — added keywords, annotations, and Artifact Hub metadata

### Fixed
- **Version consistency** — `values.yaml` image tag, README helm commands, and Chart version all aligned to 1.1.0 (was a mix of 0.1.0, 1.0.0, 1.0.2)
- **Image repository** — corrected from `ghcr.io/hermesmanager/hermesmanager` to `ghcr.io/mackding/hermes-manager` throughout
- **K8s readiness probe** — `readinessProbe` now targets `/readyz` with `initialDelaySeconds: 10` and `failureThreshold: 6` (30s window for Postgres cold-start)

### Known Limitations
- Single-admin auth only — OIDC/SSO planned for v1.2
- SPA uses mock data — real API integration planned for v1.2
- No multi-tenant namespace isolation yet

## [1.0.2] — 2026-04-22

Helm chart distribution path fix. No runtime code changes.

### Fixed
- **Helm chart OCI path collision** — v1.0.1 pushed both the container image and the Helm chart to `ghcr.io/mackding/hermes-manager`, causing the container manifest to overwrite the chart at the same tag. `helm pull` returned `could not load config with mediatype application/vnd.cncf.helm.config.v1+json`.
- Helm chart now publishes to `oci://ghcr.io/mackding/charts/hermesmanager` (separate namespace from the container image).

### Migration
- Container image stays at `ghcr.io/mackding/hermes-manager:1.0.2` (unchanged).
- Helm install command changes:
  - **Before**: `helm install hermesmanager oci://ghcr.io/mackding/hermes-manager --version 1.0.1`
  - **After**:  `helm install hermesmanager oci://ghcr.io/mackding/charts/hermesmanager --version 1.0.2`

## [1.0.1] — 2026-04-22

Release pipeline fixes. No runtime code changes.

### Fixed
- Add root `Dockerfile` (distroless/static-debian12 nonroot) so the release workflow's `docker/build-push-action` can build multi-arch images from pre-built binaries
- Lowercase the GHCR owner in the Helm OCI push step (`${GITHUB_REPOSITORY_OWNER,,}`) — registry rejected the mixed-case `MackDing` reference

### Note
- v1.0.0 tag exists on GitHub but was never fully published (container and Helm push failed). v1.0.1 is the first fully-released artifact set. Use v1.0.1 for all installs.

## [1.0.0] — 2026-04-22

First public release. Production-ready single-admin deployment.

### Added
- **Runtime drivers** — Local process, Docker, and Kubernetes Job drivers (L1 + L2)
- **Scheduler** — Round-robin and lowest-load strategies with concurrent agent pools
- **Policy engine** — YAML deny/allow rules over LLM models, users, teams, and tools (100% test coverage)
- **Postgres Store** — 11-method persistence interface with `LISTEN/NOTIFY` hot-reload (L4)
- **Slack gateway** — `/hermes status` and `/hermes run` slash commands (L5)
- **React 19 SPA** — Dashboard, Skills, Events pages embedded into the Go binary via `embed.FS` (L6)
- **Dracula Gradient theme** — Light + dark modes with intentional palette and motion
- **Helm chart** — CloudNativePG dependency, RBAC, auto-generated admin password, OCI distribution (L7)
- **Production hardening (Week 1)**:
  - zerolog structured logging across all packages
  - DB health check + retry with exponential backoff
  - Pod `SecurityContext` (non-root, read-only FS, dropped caps, seccomp `RuntimeDefault`)
  - `NetworkPolicy` (default-deny + explicit egress to Postgres / DNS / HTTPS)
  - Ingress with TLS, `PodDisruptionBudget`, `HorizontalPodAutoscaler`
  - Dependabot for Go modules, npm, Docker, GitHub Actions
- **Multi-arch release pipeline** — linux/amd64 + linux/arm64 binaries, GHCR container, OCI Helm chart
- **Documentation** — README, ARCHITECTURE, CONTRIBUTING, AGENT_API

### Security
- Codex review hardening: connection pool validation, retry idempotency semantics, seccomp profile alignment

### Known Limitations
- Single-admin auth only — OIDC / SSO planned for v1.1
- No multi-tenant namespace isolation yet

[1.1.0]: https://github.com/MackDing/hermes-manager/releases/tag/v1.1.0
[1.0.2]: https://github.com/MackDing/hermes-manager/releases/tag/v1.0.2
[1.0.1]: https://github.com/MackDing/hermes-manager/releases/tag/v1.0.1
[1.0.0]: https://github.com/MackDing/hermes-manager/releases/tag/v1.0.0
