# Changelog

All notable changes to HermesManager are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[1.0.1]: https://github.com/MackDing/HermesManager/releases/tag/v1.0.1
[1.0.0]: https://github.com/MackDing/HermesManager/releases/tag/v1.0.0
