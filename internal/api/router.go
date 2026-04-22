package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/MackDing/hermes-manager/internal/policy"
	"github.com/MackDing/hermes-manager/internal/scheduler"
	"github.com/MackDing/hermes-manager/internal/storage"
	"github.com/MackDing/hermes-manager/web"
	"github.com/rs/zerolog/log"
)

// Server holds dependencies for all API handlers.
type Server struct {
	store     storage.Store
	scheduler *scheduler.Scheduler
	policy    *policy.Engine
}

// NewServer creates an API server with real dependencies.
func NewServer(store storage.Store, sched *scheduler.Scheduler, pol *policy.Engine) *Server {
	return &Server{store: store, scheduler: sched, policy: pol}
}

// NewRouter returns the HTTP handler with stub API routes + embedded SPA.
func NewRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /readyz", handleReadyzStub)

	// Stubs for when no Server is configured
	mux.HandleFunc("GET /v1/skills", stub("GET /v1/skills"))
	mux.HandleFunc("GET /v1/skills/{name}", stub("GET /v1/skills/{name}"))
	mux.HandleFunc("POST /v1/tasks", stub("POST /v1/tasks"))
	mux.HandleFunc("GET /v1/tasks", stub("GET /v1/tasks"))
	mux.HandleFunc("GET /v1/tasks/{id}", stub("GET /v1/tasks/{id}"))
	mux.HandleFunc("POST /v1/events", stub("POST /v1/events"))
	mux.HandleFunc("GET /v1/events", stub("GET /v1/events"))

	// SPA fallback for all other GET requests
	mountSPA(mux)

	return wrapMiddleware(mux)
}

// Handler returns the HTTP handler with all routes wired to real implementations.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)

	// Skills
	mux.HandleFunc("GET /v1/skills", s.handleListSkills)
	mux.HandleFunc("GET /v1/skills/{name}", s.handleGetSkill)

	// Tasks
	mux.HandleFunc("POST /v1/tasks", s.handleCreateTask)
	mux.HandleFunc("GET /v1/tasks", s.handleListTasks)
	mux.HandleFunc("GET /v1/tasks/{id}", s.handleGetTask)

	// Events
	mux.HandleFunc("POST /v1/events", s.handleCreateEvent)
	mux.HandleFunc("GET /v1/events", s.handleListEvents)

	// SPA — serves the embedded React app for all non-API routes
	mountSPA(mux)

	return authMiddleware(wrapMiddleware(mux))
}

// wrapMiddleware applies body-limit, RequestID, and Logging middleware to any handler.
func wrapMiddleware(h http.Handler) http.Handler {
	return RequestIDMiddleware(LoggingMiddleware(limitBody(h)))
}

const maxBodySize = 1 << 20 // 1 MB

// limitBody caps the request body to maxBodySize to prevent denial-of-service
// from unbounded payloads.
func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
		next.ServeHTTP(w, r)
	})
}

// mountSPA adds the embedded React SPA as a catch-all for non-API GET requests.
func mountSPA(mux *http.ServeMux) {
	spaHandler := web.Handler()
	mux.HandleFunc("GET /{path...}", func(w http.ResponseWriter, r *http.Request) {
		spaHandler.ServeHTTP(w, r)
	})
}

// --- Skills ---

func (s *Server) handleListSkills(w http.ResponseWriter, r *http.Request) {
	skills, err := s.store.ListSkills(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list skills: %v", err)
		return
	}
	if skills == nil {
		skills = []storage.Skill{}
	}
	writeJSON(w, http.StatusOK, skills)
}

func (s *Server) handleGetSkill(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	skill, err := s.store.GetSkill(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get skill: %v", err)
		return
	}
	if skill == nil {
		writeError(w, http.StatusNotFound, "skill %q not found", name)
		return
	}
	writeJSON(w, http.StatusOK, skill)
}

// --- Tasks ---

type createTaskRequest struct {
	SkillName    string         `json:"skill_name"`
	Parameters   map[string]any `json:"parameters"`
	Runtime      string         `json:"runtime"`
	User         string         `json:"user"`
	Team         string         `json:"team"`
	DeadlineSecs int            `json:"deadline_seconds"`
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var req createTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: %v", err)
		return
	}

	if req.SkillName == "" {
		writeError(w, http.StatusBadRequest, "skill_name is required")
		return
	}

	result, err := s.CreateTask(r.Context(), CreateTaskRequest{
		SkillName:    req.SkillName,
		Parameters:   req.Parameters,
		Runtime:      req.Runtime,
		User:         req.User,
		Team:         req.Team,
		DeadlineSecs: req.DeadlineSecs,
	})
	if err != nil {
		if pde, ok := IsPolicyDenied(err); ok {
			writeError(w, http.StatusForbidden, "blocked by policy rule: %s", pde.RuleID)
			return
		}
		// Distinguish "skill not found" (client error) from server errors.
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusBadRequest, "%v", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}

	resp := map[string]any{
		"task_id": result.TaskID,
		"token":   result.Token,
	}
	if result.ExternalID != "" {
		resp["runtime"] = result.Runtime
		resp["external_id"] = result.ExternalID
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	filter := storage.TaskFilter{
		Limit:  50,
		Offset: 0,
	}
	if v := r.URL.Query().Get("state"); v != "" {
		state := storage.TaskState(v)
		filter.State = &state
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			filter.Limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			filter.Offset = n
		}
	}

	const maxPageSize = 100
	if filter.Limit > maxPageSize {
		filter.Limit = maxPageSize
	}

	tasks, err := s.store.ListTasks(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tasks: %v", err)
		return
	}
	if tasks == nil {
		tasks = []storage.Task{}
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	task, err := s.store.GetTask(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get task: %v", err)
		return
	}
	if task == nil {
		writeError(w, http.StatusNotFound, "task %q not found", id)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

// --- Events ---

type createEventRequest struct {
	ID      string         `json:"id"`
	TaskID  string         `json:"task_id"`
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
}

func (s *Server) handleCreateEvent(w http.ResponseWriter, r *http.Request) {
	// Verify agent token from Authorization header
	token := r.Header.Get("Authorization")
	if len(token) < 8 || token[:7] != "Bearer " {
		writeError(w, http.StatusUnauthorized, "missing or invalid Authorization header")
		return
	}
	bearerToken := token[7:]

	var req createEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: %v", err)
		return
	}

	if req.TaskID == "" {
		writeError(w, http.StatusBadRequest, "task_id is required")
		return
	}

	// Verify token
	tokenHash := sha256.Sum256([]byte(bearerToken))
	valid, err := s.store.VerifyAgentToken(r.Context(), req.TaskID, tokenHash[:])
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token verification failed: %v", err)
		return
	}
	if !valid {
		writeError(w, http.StatusUnauthorized, "invalid or revoked agent token")
		return
	}

	// Store event
	eventID := req.ID
	if eventID == "" {
		eventID = generateID("evt")
	}
	event := storage.Event{
		ID:        eventID,
		TaskID:    req.TaskID,
		Type:      storage.EventType(req.Type),
		Payload:   req.Payload,
		CreatedAt: time.Now(),
	}

	if err := s.store.AppendEvent(r.Context(), event); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store event: %v", err)
		return
	}

	// If terminal event, update task state and revoke token
	switch storage.EventType(req.Type) {
	case storage.EventTaskCompleted:
		if err := s.store.UpdateTaskState(r.Context(), req.TaskID, storage.TaskStateCompleted); err != nil {
			log.Error().Err(err).Str("task_id", req.TaskID).Msg("failed to update task state on terminal event")
			writeError(w, http.StatusInternalServerError, "failed to finalize task state")
			return
		}
		if err := s.store.RevokeAgentToken(r.Context(), req.TaskID); err != nil {
			log.Error().Err(err).Str("task_id", req.TaskID).Msg("failed to revoke agent token")
		}
	case storage.EventTaskFailed:
		if err := s.store.UpdateTaskState(r.Context(), req.TaskID, storage.TaskStateFailed); err != nil {
			log.Error().Err(err).Str("task_id", req.TaskID).Msg("failed to update task state on terminal event")
			writeError(w, http.StatusInternalServerError, "failed to finalize task state")
			return
		}
		if err := s.store.RevokeAgentToken(r.Context(), req.TaskID); err != nil {
			log.Error().Err(err).Str("task_id", req.TaskID).Msg("failed to revoke agent token")
		}
	case storage.EventTaskTimeout:
		if err := s.store.UpdateTaskState(r.Context(), req.TaskID, storage.TaskStateTimeout); err != nil {
			log.Error().Err(err).Str("task_id", req.TaskID).Msg("failed to update task state on terminal event")
			writeError(w, http.StatusInternalServerError, "failed to finalize task state")
			return
		}
		if err := s.store.RevokeAgentToken(r.Context(), req.TaskID); err != nil {
			log.Error().Err(err).Str("task_id", req.TaskID).Msg("failed to revoke agent token")
		}
	case storage.EventTaskStarted:
		if err := s.store.UpdateTaskState(r.Context(), req.TaskID, storage.TaskStateRunning); err != nil {
			log.Error().Err(err).Str("task_id", req.TaskID).Msg("failed to update task state on started event")
			writeError(w, http.StatusInternalServerError, "failed to update task state")
			return
		}
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": eventID})
}

func (s *Server) handleListEvents(w http.ResponseWriter, r *http.Request) {
	filter := storage.EventFilter{
		Limit:  50,
		Offset: 0,
	}
	if v := r.URL.Query().Get("task_id"); v != "" {
		filter.TaskID = &v
	}
	if v := r.URL.Query().Get("type"); v != "" {
		et := storage.EventType(v)
		filter.EventType = &et
	}
	if v := r.URL.Query().Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.Since = &t
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			filter.Limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			filter.Offset = n
		}
	}

	const maxPageSize = 100
	if filter.Limit > maxPageSize {
		filter.Limit = maxPageSize
	}

	events, err := s.store.ListEvents(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list events: %v", err)
		return
	}
	if events == nil {
		events = []storage.Event{}
	}
	writeJSON(w, http.StatusOK, events)
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, format string, args ...any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": fmt.Sprintf(format, args...),
	})
}

func generateID(prefix string) string {
	b := make([]byte, 12)
	rand.Read(b)
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(b))
}

func stub(route string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotImplemented, "not implemented: %s", route)
	}
}
