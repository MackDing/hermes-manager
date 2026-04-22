package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/MackDing/hermes-manager/internal/api"
	"github.com/MackDing/hermes-manager/internal/policy"
	"github.com/MackDing/hermes-manager/internal/runtime"
	"github.com/MackDing/hermes-manager/internal/scheduler"
	"github.com/MackDing/hermes-manager/internal/skills"
	"github.com/MackDing/hermes-manager/internal/storage/postgres"

	// Register runtime drivers via init()
	_ "github.com/MackDing/hermes-manager/internal/runtime/docker"
	_ "github.com/MackDing/hermes-manager/internal/runtime/k8s"
	_ "github.com/MackDing/hermes-manager/internal/runtime/local"
)

var version = "dev"

func main() {
	var (
		showVersion = flag.Bool("version", false, "print version and exit")
		showHelp    = flag.Bool("help", false, "print help and exit")
	)
	flag.Usage = printHelp
	flag.Parse()
	if *showVersion {
		fmt.Println("hermesmanager", version)
		return
	}
	if *showHelp {
		printHelp()
		return
	}

	// Configure zerolog
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	level, err := zerolog.ParseLevel(envOr("LOG_LEVEL", "info"))
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	port := envOr("HERMESMANAGER_PORT", "8080")
	dbURL := envOr("DATABASE_URL", "")
	policyFile := envOr("HERMESMANAGER_POLICY_FILE", "")

	// --- Store ---
	var handler http.Handler
	if dbURL != "" {
		store, err := postgres.New(ctx, dbURL)
		if err != nil {
			log.Fatal().Err(err).Str("hint", "check DATABASE_URL is reachable, e.g. postgres://user:pass@host:5432/hermesmanager?sslmode=disable").Msg("postgres connection failed")
		}
		defer store.Close()

		if err := store.Migrate(ctx); err != nil {
			log.Fatal().Err(err).Str("hint", "check DB user has CREATE TABLE permissions; see docs/TROUBLESHOOTING.md").Msg("migration failed")
		}
		log.Info().Msg("postgres connected, migrations applied")

		// --- Seed skills from filesystem ---
		skillsDir := envOr("HERMESMANAGER_SKILLS_DIR", "/etc/hermesmanager/skills")
		if _, err := os.Stat(skillsDir); err == nil {
			n, err := skills.LoadFromDir(ctx, store, skillsDir)
			if err != nil {
				log.Warn().Err(err).Str("dir", skillsDir).Msg("skill loading had errors")
			}
			log.Info().Int("count", n).Str("dir", skillsDir).Msg("skills loaded from filesystem")
		}

		// --- Policy engine ---
		var pol *policy.Engine
		if policyFile != "" {
			pol, err = policy.NewEngine(policyFile)
			if err != nil {
				log.Fatal().Err(err).Str("file", policyFile).Str("hint", "see deploy/examples/policy.yaml for reference syntax").Msg("policy load failed")
			}
			log.Info().Str("file", policyFile).Msg("policy loaded")
		}

		// --- Runtimes ---
		runtimes, err := runtime.Build()
		if err != nil {
			log.Fatal().Err(err).Str("hint", "check runtime driver env vars (DOCKER_HOST, KUBECONFIG); see docs/TROUBLESHOOTING.md").Msg("runtime build failed")
		}
		log.Info().Int("count", len(runtimes)).Msg("runtimes registered")

		// --- Scheduler ---
		sched := scheduler.NewScheduler(runtimes, store)

		// --- API server with real handlers ---
		srv := api.NewServer(store, sched, pol)
		handler = srv.Handler()
	} else {
		log.Warn().Msg("DATABASE_URL not set, running with stub handlers (dev mode)")
		log.Info().Msg("To run in production, set DATABASE_URL. See: https://github.com/MackDing/hermes-manager/blob/main/docs/QUICKSTART.md")
		handler = api.NewRouter()
	}

	httpSrv := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info().Str("version", version).Str("port", port).Msg("hermesmanager listening")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	<-ctx.Done()
	log.Info().Msg("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Fatal().Err(err).Msg("shutdown error")
	}
	log.Info().Msg("stopped")
}

func printHelp() {
	fmt.Fprintf(os.Stderr, `hermesmanager %s — K8s-native control plane for Hermes Agent fleets

Usage:
  hermesmanager [flags]

Flags:
  --version   Print version and exit
  --help      Print this help and exit

Environment variables:
  DATABASE_URL              Postgres connection string (required for production)
                            Example: postgres://user:pass@host:5432/hermesmanager?sslmode=disable
                            If empty, starts in dev mode with stub handlers.

  HERMESMANAGER_PORT        HTTP listen port (default: 8080)

  HERMESMANAGER_POLICY_FILE Path to a YAML policy file with deny/allow rules
                            See deploy/examples/policy.yaml for reference.

  HERMESMANAGER_SKILLS_DIR  Directory containing skill YAML files to seed on startup
                            (default: /etc/hermesmanager/skills)
                            Helm chart mounts the skills ConfigMap here automatically.

  LOG_LEVEL                 trace|debug|info|warn|error (default: info)

  HERMESMANAGER_API_TOKEN   Bearer token for API auth (optional in dev, required in prod)
                            Helm chart auto-injects from the admin-password Secret.

  SLACK_SIGNING_SECRET      Slack app signing secret for verifying inbound slash commands
                            (Basic Information → App Credentials in the Slack dashboard).
                            If empty, signature verification is skipped (dev mode only).

Docs:     https://github.com/MackDing/hermes-manager
Quickstart: https://github.com/MackDing/hermes-manager/blob/main/docs/QUICKSTART.md
`, version)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
