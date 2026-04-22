package api

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
)

// authMiddleware validates Bearer token on protected /v1/* routes.
// If HERMESMANAGER_API_TOKEN is empty, auth is skipped (dev mode).
func authMiddleware(next http.Handler) http.Handler {
	token := os.Getenv("HERMESMANAGER_API_TOKEN")
	if token == "" {
		log.Warn().Msg("HERMESMANAGER_API_TOKEN not set, API auth disabled (dev mode)")
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health probes
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			next.ServeHTTP(w, r)
			return
		}

		// Skip auth for SPA static assets (non-API paths)
		if !strings.HasPrefix(r.URL.Path, "/v1/") {
			next.ServeHTTP(w, r)
			return
		}

		// Skip auth for agent event ingestion — it uses per-task agent tokens
		// validated inside the handler (handleCreateEvent).
		if r.Method == http.MethodPost && r.URL.Path == "/v1/events" {
			next.ServeHTTP(w, r)
			return
		}

		auth := r.Header.Get("Authorization")
		if auth == "" {
			http.Error(w, `{"error":"missing Authorization header"}`, http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(auth, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			http.Error(w, `{"error":"invalid Authorization header, expected: Bearer <token>"}`, http.StatusUnauthorized)
			return
		}

		if subtle.ConstantTimeCompare([]byte(parts[1]), []byte(token)) != 1 {
			http.Error(w, `{"error":"invalid token"}`, http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
