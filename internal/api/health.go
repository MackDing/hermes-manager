package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// handleHealthz is a liveness probe — returns 200 as long as the process is alive.
// It MUST NOT check dependencies; kubelet uses this to decide when to restart the pod.
func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleReadyz is a readiness probe — returns 200 only when dependencies (DB) are ready.
// 503 during startup or when DB becomes unreachable.
func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	w.Header().Set("Content-Type", "application/json")
	if err := s.store.Ping(ctx); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"ready":  "false",
			"reason": "database not ready",
		})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"ready": "true"})
}

// handleReadyzStub always returns ready for dev mode (no DB configured).
func handleReadyzStub(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"ready": "true", "mode": "dev-stub"})
}
