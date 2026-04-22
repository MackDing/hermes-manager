// Package slack implements a Slack bot gateway for HermesManager.
// It handles incoming Slack slash commands via HTTP POST and translates
// them into task creation via the shared pipeline (policy, token, dispatch).
package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/MackDing/hermes-manager/internal/storage"
	"github.com/rs/zerolog/log"
)

// TaskCreator abstracts the full task-creation pipeline so the Slack bot
// goes through the same path as the HTTP API (policy, token, dispatch).
type TaskCreator interface {
	CreateTask(ctx context.Context, req TaskCreateInput) (*TaskCreateOutput, error)
}

// TaskCreateInput mirrors api.CreateTaskRequest without importing the api
// package, keeping the dependency arrow from gateway/slack → api optional.
// The wiring layer (main.go) bridges the two via an adapter or direct call.
type TaskCreateInput struct {
	SkillName  string
	Parameters map[string]any
	Runtime    string
	User       string
	Team       string
}

// TaskCreateOutput carries the result of a successful pipeline-based task creation.
type TaskCreateOutput struct {
	TaskID string
	Token  string
}

// Bot is the Slack slash-command HTTP handler. It holds a reference to the
// Store for querying skills and task status, and a TaskCreator for creating
// tasks through the full pipeline (policy + token + dispatch).
type Bot struct {
	store         storage.Store
	signingSecret string
	creator       TaskCreator
}

// NewBot returns a Bot wired to the given Store and TaskCreator. The
// signingSecret is the Slack app's signing secret (Basic Information → App
// Credentials). Pass "" only in dev: signature verification is skipped, and
// any caller can invoke slash commands. The creator handles the full task
// pipeline (policy, token, dispatch); pass nil only if task creation via Slack
// is not needed (status-only mode).
func NewBot(store storage.Store, signingSecret string, creator TaskCreator) *Bot {
	return &Bot{store: store, signingSecret: signingSecret, creator: creator}
}

// slackResponse is the JSON envelope Slack expects from slash-command handlers.
type slackResponse struct {
	ResponseType string `json:"response_type"`
	Text         string `json:"text"`
}

// ServeHTTP dispatches incoming Slack slash commands. The command text
// (everything after the slash command itself) arrives in the "text" form field.
// The first word is treated as a sub-command; the rest is the argument.
//
// Supported sub-commands:
//
//	status  — list task counts grouped by state
//	run     — create a new task: "run skill_name {json_params}"
func (b *Bot) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Verify the request came from Slack (HMAC-SHA256 signature check).
	// This consumes r.Body, so we parse form values from the returned bytes.
	body := verifySlackRequest(w, r, b.signingSecret)
	if body == nil {
		return // verifySlackRequest already wrote the HTTP error
	}

	form, err := url.ParseQuery(string(body))
	if err != nil {
		http.Error(w, "malformed form body", http.StatusBadRequest)
		return
	}

	text := strings.TrimSpace(form.Get("text"))
	subCmd, rest := splitFirst(text)

	switch strings.ToLower(subCmd) {
	case "status":
		b.handleStatus(w, r)
	case "run":
		b.handleRun(w, r, rest)
	case "help", "":
		b.handleHelp(w, r)
	default:
		respondJSON(w, slackResponse{
			ResponseType: "ephemeral",
			Text: fmt.Sprintf(
				"Unknown command `%s`. Try `/hermes help` to see all commands.",
				subCmd,
			),
		})
	}
}

// handleHelp returns a user-friendly list of supported sub-commands.
// Also triggered when the user invokes `/hermes` with no arguments.
func (b *Bot) handleHelp(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, slackResponse{
		ResponseType: "ephemeral",
		Text: "*HermesManager Slack Commands*\n\n" +
			"`/hermes status` — Show task counts by state (pending / running / completed / failed / timeout)\n" +
			"`/hermes run <skill> <json-params>` — Submit a task\n" +
			"   Example: `/hermes run hello-skill {\"name\":\"World\"}`\n" +
			"`/hermes help` — Show this message\n\n" +
			"Docs: <https://github.com/MackDing/hermes-manager/blob/main/docs/QUICKSTART.md|Quickstart>",
	})
}

// handleStatus queries all tasks and returns per-state counts.
func (b *Bot) handleStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tasks, err := b.store.ListTasks(ctx, storage.TaskFilter{Limit: 1000})
	if err != nil {
		log.Error().Err(err).Msg("slack: failed to list tasks for status command")
		respondJSON(w, slackResponse{
			ResponseType: "ephemeral",
			Text:         "Failed to query task status. Check the hermesmanager logs or contact an admin.",
		})
		return
	}

	counts := map[storage.TaskState]int{}
	for _, t := range tasks {
		counts[t.State]++
	}

	var sb strings.Builder
	sb.WriteString("*Task Status*\n")
	for _, state := range []storage.TaskState{
		storage.TaskStatePending,
		storage.TaskStateRunning,
		storage.TaskStateCompleted,
		storage.TaskStateFailed,
		storage.TaskStateTimeout,
	} {
		sb.WriteString(fmt.Sprintf("• %s: %d\n", state, counts[state]))
	}
	sb.WriteString(fmt.Sprintf("Total: %d", len(tasks)))

	respondJSON(w, slackResponse{
		ResponseType: "in_channel",
		Text:         sb.String(),
	})
}

// handleRun parses "skill_name {json_params}" from the remaining command text
// and creates a task through the full pipeline (policy, token, dispatch).
func (b *Bot) handleRun(w http.ResponseWriter, r *http.Request, args string) {
	ctx := r.Context()

	skillName, paramsRaw := splitFirst(args)
	if skillName == "" {
		respondJSON(w, slackResponse{
			ResponseType: "ephemeral",
			Text:         "Usage: `/hermes run <skill> <json-params>`. Try `/hermes help` for examples.",
		})
		return
	}

	if b.creator == nil {
		respondJSON(w, slackResponse{
			ResponseType: "ephemeral",
			Text:         "Task creation is not available. The server may be running in status-only mode.",
		})
		return
	}

	// Parse parameters JSON.
	params := map[string]any{}
	if paramsRaw != "" {
		if err := json.Unmarshal([]byte(paramsRaw), &params); err != nil {
			log.Debug().Err(err).Str("skill", skillName).Msg("slack: invalid JSON params")
			respondJSON(w, slackResponse{
				ResponseType: "ephemeral",
				Text:         "Invalid JSON parameters. Example: `/hermes run hello-skill {\"name\":\"World\"}`",
			})
			return
		}
	}

	result, err := b.creator.CreateTask(ctx, TaskCreateInput{
		SkillName:  skillName,
		Parameters: params,
		Runtime:    "local",
		User:       "slack-user",
	})
	if err != nil {
		log.Error().Err(err).Str("skill", skillName).Msg("slack: task creation failed")
		// Don't leak internal error details to Slack users.
		respondJSON(w, slackResponse{
			ResponseType: "ephemeral",
			Text:         "Failed to submit task (runtime error). Check the hermesmanager logs or contact an admin.",
		})
		return
	}

	respondJSON(w, slackResponse{
		ResponseType: "in_channel",
		Text:         fmt.Sprintf("Task %s created for skill %q", result.TaskID, skillName),
	})
}

// --- helpers ---

// splitFirst splits s into the first whitespace-delimited word and the rest.
func splitFirst(s string) (string, string) {
	s = strings.TrimSpace(s)
	idx := strings.IndexAny(s, " \t")
	if idx < 0 {
		return s, ""
	}
	return s[:idx], strings.TrimSpace(s[idx+1:])
}

// respondJSON writes a slackResponse as JSON with the appropriate content type.
func respondJSON(w http.ResponseWriter, resp slackResponse) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// generateID creates a simple time-based ID. Not crypto-random — good enough
// for v0.1 where the scheduler will assign the canonical ID if needed.
func generateID(t time.Time) string {
	return fmt.Sprintf("task-%d", t.UnixNano())
}

// GenerateIDForTest exposes generateID for deterministic test assertions.
// Kept in this file to avoid a _test.go export hack.
func GenerateIDForTest(t time.Time) string {
	return generateID(t)
}

// StoreSubset documents which Store methods Bot actually uses. It is not
// referenced at runtime (Bot takes the full Store interface) but guides
// test-mock construction. Task creation goes through the TaskCreator
// interface, not through the Store directly.
type StoreSubset interface {
	ListTasks(ctx context.Context, filter storage.TaskFilter) ([]storage.Task, error)
}
