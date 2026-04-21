# Agent API Contract — HermesManager v0.1

**Status:** FROZEN. Changes require a design doc amendment.

## Overview

This document defines the protocol between the HermesManager control plane and any running Hermes agent. It is the smallest viable contract; nothing here is optional for v0.1.

## Control Plane → Agent (Task Dispatch)

The control plane creates a workload (K8s Job, Docker container, or local process) with:

### Environment Variables

| Variable | Description |
|----------|-------------|
| `HERMESMANAGER_TASK_ID` | UUID, immutable for this task |
| `HERMESMANAGER_CALLBACK_URL` | Events endpoint, **computed per-runtime** (see below) |
| `HERMESMANAGER_AGENT_TOKEN` | Bearer token for callback auth (per-task, revocable) |

### Callback URL by Runtime

| Runtime | Callback URL |
|---------|-------------|
| `local` | `http://127.0.0.1:{PORT}/v1/events` |
| `docker` | `http://host.docker.internal:{PORT}/v1/events` |
| `k8s` | `http://hermesmanager.{NAMESPACE}.svc.cluster.local:{PORT}/v1/events` |

### Task Definition (mounted at `/etc/hermesmanager/task.json`)

```json
{
  "task_id": "uuid",
  "skill": "hello-skill",
  "parameters": {"name": "world"},
  "policy_context": {
    "user": "admin",
    "team": "default",
    "model_allowed": ["gpt-4o-mini"]
  },
  "deadline_seconds": 300
}
```

## Agent → Control Plane (Event Reporting)

Agent POSTs JSON events to `HERMESMANAGER_CALLBACK_URL` with header:
```
Authorization: Bearer <HERMESMANAGER_AGENT_TOKEN>
Content-Type: application/json
```

### Event Types

| Type | Payload Fields | When |
|------|---------------|------|
| `task.started` | `{}` | Agent has started executing |
| `task.llm_call` | `{model, prompt_tokens, completion_tokens, cost_usd}` | Each LLM API call |
| `task.tool_call` | `{tool_name, args_redacted}` | Each tool invocation |
| `task.policy_blocked` | `{rule_id, reason}` | Agent self-gated against policy |
| `task.completed` | `{result, exit_code}` | Task finished successfully |
| `task.failed` | `{error, exit_code}` | Task failed |

### Event JSON Shape

```json
{
  "id": "evt-uuid",
  "task_id": "task-uuid",
  "type": "task.llm_call",
  "payload": {
    "model": "gpt-4o-mini",
    "prompt_tokens": 150,
    "completion_tokens": 42,
    "cost_usd": 0.004
  }
}
```

## Authentication

- Per-task bearer token stored as bcrypt hash in `agent_tokens` table
- Token mounted into the agent via env var (K8s: Secret with `ownerReferences` on the Job)
- Token is revoked on `task.completed`, `task.failed`, or `task.timeout`
- Replayed token after revocation → HTTP 401

## State Machine

```
pending → running → completed
                  → failed
                  → timeout
```

The **control plane** is the source of truth for task state. The agent has no opinion on state outside its own execution.

## Failure Semantics

If the agent exits without sending `task.completed` or `task.failed`:
- **K8s runtime:** The informer detects Job completion/failure and synthesizes a `task.timeout` or `task.failed` event
- **Docker runtime:** Container exit code is checked; non-zero → `task.failed`, timeout → `task.timeout`
- **Local runtime:** Process exit is monitored; same logic as Docker

**No silent task losses.** Every task will eventually reach a terminal state.

## K8s Informer Scoping

- Namespace-scoped (configurable via `--watch-namespace` flag / `watchNamespace` Helm value)
- Label-selected: `hermesmanager.io/managed=true`
- All Jobs created by the scheduler carry this label
