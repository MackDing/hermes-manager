package api

import (
	"encoding/json"
	"net/http"
)

// NewRouter returns the HTTP handler with all v0.1 routes.
// Route stubs return 501 until subagents fill in implementations.
func NewRouter() http.Handler {
	mux := http.NewServeMux()

	// Health check — implemented immediately (needed for Helm readiness probe)
	mux.HandleFunc("GET /healthz", handleHealthz)

	// Skills (read-only)
	mux.HandleFunc("GET /v1/skills", stub("GET /v1/skills"))
	mux.HandleFunc("GET /v1/skills/{name}", stub("GET /v1/skills/{name}"))

	// Tasks
	mux.HandleFunc("POST /v1/tasks", stub("POST /v1/tasks"))
	mux.HandleFunc("GET /v1/tasks", stub("GET /v1/tasks"))
	mux.HandleFunc("GET /v1/tasks/{id}", stub("GET /v1/tasks/{id}"))

	// Events (agent callback endpoint + read endpoint)
	mux.HandleFunc("POST /v1/events", stub("POST /v1/events"))
	mux.HandleFunc("GET /v1/events", stub("GET /v1/events"))

	return mux
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func stub(route string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "not implemented",
			"route": route,
		})
	}
}
