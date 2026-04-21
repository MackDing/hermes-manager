package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rs/zerolog/log"
)

// Health tracks the database connection health status.
type Health struct {
	healthy atomic.Bool
}

// StartHealthCheck runs a background goroutine that pings the database every interval.
func (s *Store) StartHealthCheck(ctx context.Context, interval time.Duration) {
	s.health.healthy.Store(true) // optimistic start

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				err := s.pool.Ping(ctx)
				wasHealthy := s.health.healthy.Load()
				s.health.healthy.Store(err == nil)

				if err != nil && wasHealthy {
					log.Warn().Err(err).Msg("postgres health check failed")
				} else if err == nil && !wasHealthy {
					log.Info().Msg("postgres health check recovered")
				}
			}
		}
	}()
}

// IsHealthy returns the current database health status.
func (s *Store) IsHealthy() bool {
	return s.health.healthy.Load()
}

// WaitForReady blocks until the database is reachable or the timeout expires.
func (s *Store) WaitForReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.After(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("postgres: not ready after %v", timeout)
		case <-ticker.C:
			if err := s.pool.Ping(ctx); err == nil {
				return nil
			}
		}
	}
}

// WithRetry executes fn with exponential backoff on transient errors.
// Only retries connection-level and serialization errors, not constraint violations.
func WithRetry(ctx context.Context, maxAttempts int, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if !isTransient(lastErr) {
			return lastErr
		}
		if attempt < maxAttempts-1 {
			backoff := time.Duration(math.Pow(2, float64(attempt))) * 100 * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}
	}
	return fmt.Errorf("postgres: max retries (%d) exceeded: %w", maxAttempts, lastErr)
}

// isTransient returns true for errors that are worth retrying.
func isTransient(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "40001": // serialization_failure
			return true
		case "40P01": // deadlock_detected
			return true
		case "08000", "08003", "08006": // connection errors
			return true
		case "57P01": // admin_shutdown
			return true
		}
		return false
	}
	// Connection-level errors (not pg protocol errors) — only deadline, NOT canceled
	return errors.Is(err, context.DeadlineExceeded)
}
