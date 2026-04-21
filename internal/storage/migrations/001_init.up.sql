-- 001_init.up.sql
-- HermesManager v0.1 schema

CREATE TABLE IF NOT EXISTS skills (
    name           TEXT PRIMARY KEY,
    version        TEXT NOT NULL DEFAULT '0.0.0',
    description    TEXT NOT NULL DEFAULT '',
    entrypoint     TEXT NOT NULL,
    parameters     JSONB NOT NULL DEFAULT '[]',
    required_tools TEXT[] NOT NULL DEFAULT '{}',
    required_models TEXT[] NOT NULL DEFAULT '{}',
    source_file    TEXT NOT NULL DEFAULT '',
    loaded_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tasks (
    id               TEXT PRIMARY KEY,
    skill_name       TEXT NOT NULL REFERENCES skills(name),
    parameters       JSONB NOT NULL DEFAULT '{}',
    policy_context   JSONB NOT NULL DEFAULT '{}',
    runtime          TEXT NOT NULL DEFAULT 'local',
    state            TEXT NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'running', 'completed', 'failed', 'timeout')),
    deadline_seconds INTEGER NOT NULL DEFAULT 300,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tasks_state ON tasks (state);
CREATE INDEX idx_tasks_created_at ON tasks (created_at DESC);
CREATE INDEX idx_tasks_skill_name ON tasks (skill_name);

CREATE TABLE IF NOT EXISTS events (
    id         TEXT PRIMARY KEY,
    task_id    TEXT NOT NULL REFERENCES tasks(id),
    type       TEXT NOT NULL CHECK (type IN (
        'task.started', 'task.llm_call', 'task.tool_call',
        'task.policy_blocked', 'task.completed', 'task.failed', 'task.timeout'
    )),
    payload    JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_events_task_id ON events (task_id);
CREATE INDEX idx_events_type ON events (type);
CREATE INDEX idx_events_created_at ON events (created_at DESC);

-- GIN index on JSONB payload for efficient queries like:
--   WHERE payload @> '{"model": "gpt-4o-mini"}'
CREATE INDEX idx_events_payload_gin ON events USING GIN (payload jsonb_path_ops);

CREATE TABLE IF NOT EXISTS agent_tokens (
    task_id     TEXT PRIMARY KEY REFERENCES tasks(id) ON DELETE CASCADE,
    token_hash  BYTEA NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked     BOOLEAN NOT NULL DEFAULT FALSE
);
