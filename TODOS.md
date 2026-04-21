# TODOS — HermesManager

Captured TODOs from `/plan-eng-review` on 2026-04-21. Each entry has enough context to be picked up in 3 months without losing the reasoning.

---

## v0.2 — Replace single admin password with OIDC

**What:** Add OIDC (OpenID Connect) authentication to the Web UI, replacing the v0.1 single-admin-password (`HERMESMANAGER_ADMIN_PASSWORD`).

**Why:** Codex's /office-hours cold-read flagged P8 (admin auth) as "deferred risk disguised as closure." A generated password in `NOTES.txt` plus a Slack-triggerable control plane is a footgun in shared environments. v0.1 README marks it "demo / dev-cluster only," so OIDC is the credibility unlock for the first production deployment.

**Pros:**
- Real multi-user support (each user has their own identity, sessions, audit trail)
- Compatible with corporate IdPs (Okta, Auth0, Google Workspace, Azure AD, Keycloak)
- Required precondition for multi-user RBAC (which is itself a v0.2+ candidate)

**Cons:**
- ≈1 week of focused work (handler rewrite + session middleware + Helm value plumbing + docs)
- Adds ~2 dependencies (`coreos/go-oidc` + a session store driver)

**Context:**
- P8 in design doc commits to OIDC as the v0.2 upgrade path.
- The bcrypt-password code in v0.1 lives in `internal/api/auth.go` (not yet written) — keep it behind an `Authenticator` interface so OIDC is a parallel implementation, not a rewrite.
- Recommended IdP for self-hosters: bundle Dex as an optional Helm subchart so single-tenant deployments work without an external IdP.

**Depends on / blocked by:** v0.1 must ship first.

---

## v0.2+ — Enable pgvector for semantic skill search

**What:** Add `CREATE EXTENSION pgvector` to the Postgres bootstrap, generate embeddings for `skill.description`, expose `GET /skills/search?q=...`.

**Why:** P2 cited pgvector as a strategic reason to choose Postgres over MySQL. Cashing in on that reason needs an explicit step: enable extension, embed descriptions on skill load, add a search endpoint. Once a team has 50+ skills, browsing the alphabetical list breaks down — search becomes the primary discovery interface.

**Pros:**
- Hermes Agent already has FTS5 + LLM summarization for its own skill discovery; HermesManager exposing the same UX at fleet scale is a continuation of the Hermes thesis
- Extremely cheap with Postgres ("nearly free" once the chart includes pgvector)
- A genuine differentiator vs. ClawManager's hash-keyed skill registry

**Cons:**
- Requires picking an embedding model (local `nomic-embed-text` via Ollama? OpenAI text-embedding-3-small? Configurable?)
- Embedding generation cost on skill upsert (one-time + on description change)
- Re-embedding on model change is a migration job

**Context:**
- P2 in design doc: "pgvector is a single `CREATE EXTENSION` away when v0.2+ adds semantic skill search"
- Schema sketch: `ALTER TABLE skills ADD COLUMN description_embedding vector(768)`; ivfflat or hnsw index for ANN search
- Default to local embedding (Ollama) to honor "no API key required" v0.1 promise

**Depends on / blocked by:** ≥50 skills existing in real deployments before this is worth UX-justifying. Currently 0 skills exist.

---

## Pre-Week-1 — Pin Hermes Agent version for v0.1 integration

**What:** Choose a specific Hermes Agent release tag (e.g., `v0.4.x`) and document the integration assumptions: what entrypoint to invoke, what env vars Hermes reads, how it emits events, what filesystem layout it expects.

**Why:** Open Question #1 from the design doc. BOTH the human reviewer (#9) and Codex flagged Week 1 Day 5-7 (the K8s driver + Hermes integration) as the highest schedule slip risk. Resolving the pin BEFORE Week 1 starts converts a 5-day spike with unknown unknowns into a 3-4 day implementation with known unknowns.

**Pros:**
- De-risks the highest-risk week of v0.1
- Forces an early read of Hermes upstream — likely surfaces 1-2 things the design doc got wrong about Hermes
- Makes the Agent API Contract concrete instead of theoretical

**Cons:**
- 0.5-1 day of upfront research before Day 1 of "real" work
- Locking to a version means later Hermes upgrades require migration (acceptable cost)

**Context:**
- Design doc Open Questions #1 and Dependencies section both flag this as a v0.1 blocker
- Reading targets: `https://github.com/nousresearch/hermes-agent/releases`, `agent/` directory in that repo, the README's "running headless" section
- Outcome should be a `docs/HERMES_INTEGRATION.md` that locks: version, env vars consumed, filesystem expectations, exit codes, log format
- Codex's 48-hour prototype suggested using a `fake-hermes` worker as a workaround if Hermes integration takes too long; pinning the version cheaply might eliminate the need for fake-hermes entirely

**Depends on / blocked by:** Nothing. This is pre-Week-1 work that should happen before any code is written.
