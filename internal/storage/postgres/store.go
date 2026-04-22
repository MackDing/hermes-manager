// Package postgres implements the storage.Store interface using PostgreSQL 16+ via pgx/v5.
//
// Owner: Lane L4
// Depends on: internal/storage/store.go (FROZEN interface)
package postgres

import (
	"context"
	"crypto/subtle"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MackDing/hermes-manager/internal/storage"
	"github.com/MackDing/hermes-manager/internal/storage/migrations"
)

// Store implements storage.Store backed by PostgreSQL.
type Store struct {
	pool   *pgxpool.Pool
	health Health
}

// New creates a new Postgres-backed Store from a connection string.
// Example DSN: "postgres://user:pass@localhost:5432/hermesmanager?sslmode=disable"
func New(ctx context.Context, dsn string) (*Store, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres: parse config: %w", err)
	}
	maxConns := int32(envInt("PG_MAX_CONNS", 10))
	minConns := int32(envInt("PG_MIN_CONNS", 2))
	if maxConns < 1 {
		maxConns = 10
	}
	if minConns < 0 {
		minConns = 2
	}
	if minConns > maxConns {
		minConns = maxConns
	}
	config.MaxConns = maxConns
	config.MinConns = minConns
	config.MaxConnLifetime = time.Duration(envInt("PG_MAX_CONN_LIFETIME_MINS", 60)) * time.Minute
	config.HealthCheckPeriod = time.Duration(envInt("PG_HEALTH_CHECK_SECS", 30)) * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("postgres: connect: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}

	return &Store{pool: pool}, nil
}

// Ping checks database connectivity.
func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// Migrate runs embedded migration SQL.
// For v0.1, only 001_init.up.sql is expected.
func (s *Store) Migrate(ctx context.Context) error {
	upSQL, err := migrations.FS.ReadFile("001_init.up.sql")
	if err != nil {
		return fmt.Errorf("postgres: read migration: %w", err)
	}
	_, err = s.pool.Exec(ctx, string(upSQL))
	if err != nil {
		return fmt.Errorf("postgres: run migration: %w", err)
	}
	return nil
}

// Close shuts down the connection pool.
func (s *Store) Close() error {
	s.pool.Close()
	return nil
}

// --- Skills ---

func (s *Store) UpsertSkill(ctx context.Context, sk storage.Skill) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO skills (name, version, description, entrypoint, parameters, required_tools, required_models, source_file, loaded_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		ON CONFLICT (name) DO UPDATE SET
			version = EXCLUDED.version,
			description = EXCLUDED.description,
			entrypoint = EXCLUDED.entrypoint,
			parameters = EXCLUDED.parameters,
			required_tools = EXCLUDED.required_tools,
			required_models = EXCLUDED.required_models,
			source_file = EXCLUDED.source_file,
			loaded_at = EXCLUDED.loaded_at,
			updated_at = NOW()
	`, sk.Name, sk.Version, sk.Description, sk.Entrypoint, sk.Parameters,
		sk.RequiredTools, sk.RequiredModels, sk.SourceFile, sk.LoadedAt)
	return err
}

func (s *Store) GetSkill(ctx context.Context, name string) (*storage.Skill, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT name, version, description, entrypoint, parameters, required_tools, required_models, source_file, loaded_at
		FROM skills WHERE name = $1
	`, name)

	var sk storage.Skill
	err := row.Scan(&sk.Name, &sk.Version, &sk.Description, &sk.Entrypoint,
		&sk.Parameters, &sk.RequiredTools, &sk.RequiredModels, &sk.SourceFile, &sk.LoadedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sk, nil
}

func (s *Store) ListSkills(ctx context.Context) ([]storage.Skill, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT name, version, description, entrypoint, parameters, required_tools, required_models, source_file, loaded_at
		FROM skills ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var skills []storage.Skill
	for rows.Next() {
		var sk storage.Skill
		if err := rows.Scan(&sk.Name, &sk.Version, &sk.Description, &sk.Entrypoint,
			&sk.Parameters, &sk.RequiredTools, &sk.RequiredModels, &sk.SourceFile, &sk.LoadedAt); err != nil {
			return nil, err
		}
		skills = append(skills, sk)
	}
	return skills, rows.Err()
}

// --- Tasks ---

func (s *Store) CreateTask(ctx context.Context, t storage.Task) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO tasks (id, skill_name, parameters, policy_context, runtime, state, deadline_seconds, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
	`, t.ID, t.SkillName, t.Parameters, t.PolicyContext, t.Runtime, t.State, t.DeadlineSecs, t.CreatedAt)
	return err
}

func (s *Store) GetTask(ctx context.Context, id string) (*storage.Task, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, skill_name, parameters, policy_context, runtime, state, deadline_seconds, created_at, updated_at
		FROM tasks WHERE id = $1
	`, id)

	var t storage.Task
	err := row.Scan(&t.ID, &t.SkillName, &t.Parameters, &t.PolicyContext,
		&t.Runtime, &t.State, &t.DeadlineSecs, &t.CreatedAt, &t.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) ListTasks(ctx context.Context, filter storage.TaskFilter) ([]storage.Task, error) {
	var where []string
	var args []any
	argN := 1

	if filter.State != nil {
		where = append(where, fmt.Sprintf("state = $%d", argN))
		args = append(args, *filter.State)
		argN++
	}

	query := "SELECT id, skill_name, parameters, policy_context, runtime, state, deadline_seconds, created_at, updated_at FROM tasks"
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY created_at DESC"

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argN, argN+1)
	args = append(args, limit, filter.Offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []storage.Task
	for rows.Next() {
		var t storage.Task
		if err := rows.Scan(&t.ID, &t.SkillName, &t.Parameters, &t.PolicyContext,
			&t.Runtime, &t.State, &t.DeadlineSecs, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (s *Store) UpdateTaskState(ctx context.Context, id string, state storage.TaskState) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE tasks SET state = $1, updated_at = NOW() WHERE id = $2
	`, state, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: task %q not found", id)
	}
	return nil
}

// --- Events ---

func (s *Store) AppendEvent(ctx context.Context, e storage.Event) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO events (id, task_id, type, payload, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, e.ID, e.TaskID, e.Type, e.Payload, e.CreatedAt)
	return err
}

func (s *Store) ListEvents(ctx context.Context, filter storage.EventFilter) ([]storage.Event, error) {
	var where []string
	var args []any
	argN := 1

	if filter.TaskID != nil {
		where = append(where, fmt.Sprintf("task_id = $%d", argN))
		args = append(args, *filter.TaskID)
		argN++
	}
	if filter.EventType != nil {
		where = append(where, fmt.Sprintf("type = $%d", argN))
		args = append(args, *filter.EventType)
		argN++
	}
	if filter.Since != nil {
		where = append(where, fmt.Sprintf("created_at >= $%d", argN))
		args = append(args, *filter.Since)
		argN++
	}

	query := "SELECT id, task_id, type, payload, created_at FROM events"
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY created_at DESC"

	limit := filter.Limit
	if limit <= 0 {
		limit = 500
	}
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argN, argN+1)
	args = append(args, limit, filter.Offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []storage.Event
	for rows.Next() {
		var e storage.Event
		if err := rows.Scan(&e.ID, &e.TaskID, &e.Type, &e.Payload, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// --- Agent Tokens ---

func (s *Store) StoreAgentToken(ctx context.Context, taskID string, tokenHash []byte) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO agent_tokens (task_id, token_hash)
		VALUES ($1, $2)
		ON CONFLICT (task_id) DO UPDATE SET token_hash = EXCLUDED.token_hash, revoked = FALSE
	`, taskID, tokenHash)
	return err
}

func (s *Store) VerifyAgentToken(ctx context.Context, taskID string, presented []byte) (bool, error) {
	var stored []byte
	var revoked bool
	err := s.pool.QueryRow(ctx, `
		SELECT token_hash, revoked FROM agent_tokens WHERE task_id = $1
	`, taskID).Scan(&stored, &revoked)

	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if revoked {
		return false, nil
	}
	return subtle.ConstantTimeCompare(stored, presented) == 1, nil
}

func (s *Store) RevokeAgentToken(ctx context.Context, taskID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE agent_tokens SET revoked = TRUE WHERE task_id = $1
	`, taskID)
	return err
}

// --- LISTEN/NOTIFY helpers ---

// NotifySkillsChanged sends a notification on the skills_changed channel.
// Call this after reloading skills from YAML files.
func (s *Store) NotifySkillsChanged(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, "SELECT pg_notify('skills_changed', '')")
	return err
}

// NotifyPoliciesChanged sends a notification on the policies_changed channel.
func (s *Store) NotifyPoliciesChanged(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, "SELECT pg_notify('policies_changed', '')")
	return err
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
