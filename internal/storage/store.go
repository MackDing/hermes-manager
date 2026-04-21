// Package storage defines the Store interface for HermesManager's data layer.
//
// FROZEN after F0 — no subagent may add, remove, or modify methods.
// If a new method is needed, create REQUEST_INTERFACE_CHANGE.md in the worktree root.
package storage

import (
	"context"
	"time"
)

// Store is the DB-agnostic interface for all HermesManager state.
// v0.1 ships PostgreSQL (internal/storage/postgres/).
// v0.2+ may add MySQL or external-managed Postgres — same interface.
type Store interface {
	// Skills — read-only in v0.1; YAML files are source of truth, DB is cache.
	UpsertSkill(ctx context.Context, s Skill) error
	GetSkill(ctx context.Context, name string) (*Skill, error)
	ListSkills(ctx context.Context) ([]Skill, error)

	// Tasks
	CreateTask(ctx context.Context, t Task) error
	GetTask(ctx context.Context, id string) (*Task, error)
	ListTasks(ctx context.Context, filter TaskFilter) ([]Task, error)
	UpdateTaskState(ctx context.Context, id string, state TaskState) error

	// Events — agent callbacks write here; audit UI reads.
	AppendEvent(ctx context.Context, e Event) error
	ListEvents(ctx context.Context, filter EventFilter) ([]Event, error)

	// Agent tokens — per-task bearer tokens for callback auth.
	StoreAgentToken(ctx context.Context, taskID string, tokenHash []byte) error
	VerifyAgentToken(ctx context.Context, taskID string, presented []byte) (bool, error)
	RevokeAgentToken(ctx context.Context, taskID string) error

	// Lifecycle
	Migrate(ctx context.Context) error
	Close() error
}

// --- Domain types ---

type Skill struct {
	Name           string   `json:"name"`
	Version        string   `json:"version"`
	Description    string   `json:"description"`
	Entrypoint     string   `json:"entrypoint"`
	Parameters     []Param  `json:"parameters"`
	RequiredTools  []string `json:"required_tools"`
	RequiredModels []string `json:"required_models"`
	SourceFile     string   `json:"source_file"` // path to the YAML file that defined this skill
	LoadedAt       time.Time `json:"loaded_at"`
}

type Param struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description,omitempty"`
}

type TaskState string

const (
	TaskStatePending   TaskState = "pending"
	TaskStateRunning   TaskState = "running"
	TaskStateCompleted TaskState = "completed"
	TaskStateFailed    TaskState = "failed"
	TaskStateTimeout   TaskState = "timeout"
)

type Task struct {
	ID            string            `json:"id"`
	SkillName     string            `json:"skill_name"`
	Parameters    map[string]any    `json:"parameters"`
	PolicyContext PolicyContext      `json:"policy_context"`
	Runtime       string            `json:"runtime"` // "local", "docker", "k8s"
	State         TaskState         `json:"state"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	DeadlineSecs  int               `json:"deadline_seconds"`
}

type PolicyContext struct {
	User         string   `json:"user"`
	Team         string   `json:"team"`
	ModelAllowed []string `json:"model_allowed"`
}

type TaskFilter struct {
	State  *TaskState `json:"state,omitempty"`
	Limit  int        `json:"limit"`
	Offset int        `json:"offset"`
}

type EventType string

const (
	EventTaskStarted       EventType = "task.started"
	EventTaskLLMCall       EventType = "task.llm_call"
	EventTaskToolCall      EventType = "task.tool_call"
	EventTaskPolicyBlocked EventType = "task.policy_blocked"
	EventTaskCompleted     EventType = "task.completed"
	EventTaskFailed        EventType = "task.failed"
	EventTaskTimeout       EventType = "task.timeout"
)

type Event struct {
	ID        string         `json:"id"`
	TaskID    string         `json:"task_id"`
	Type      EventType      `json:"type"`
	Payload   map[string]any `json:"payload"` // JSONB in Postgres
	CreatedAt time.Time      `json:"created_at"`
}

type EventFilter struct {
	TaskID    *string    `json:"task_id,omitempty"`
	EventType *EventType `json:"event_type,omitempty"`
	Since     *time.Time `json:"since,omitempty"`
	Limit     int        `json:"limit"`
	Offset    int        `json:"offset"`
}
