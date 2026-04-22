package slack

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MackDing/hermes-manager/internal/storage"
)

// ---------------------------------------------------------------------------
// Mock Store — only methods actually used by Bot (ListTasks for status).
// Every other method panics so tests fail loudly if Bot ever touches
// an unexpected dependency.
// ---------------------------------------------------------------------------

type mockStore struct {
	tasks []storage.Task

	// Error injection.
	listTasksErr error
}

func newMockStore() *mockStore {
	return &mockStore{}
}

func (m *mockStore) ListTasks(_ context.Context, _ storage.TaskFilter) ([]storage.Task, error) {
	if m.listTasksErr != nil {
		return nil, m.listTasksErr
	}
	return m.tasks, nil
}

// --- Unimplemented methods (panic on call) ---

func (m *mockStore) UpsertSkill(context.Context, storage.Skill) error {
	panic("UpsertSkill not expected")
}
func (m *mockStore) GetSkill(context.Context, string) (*storage.Skill, error) {
	panic("GetSkill not expected")
}
func (m *mockStore) ListSkills(context.Context) ([]storage.Skill, error) {
	panic("ListSkills not expected")
}
func (m *mockStore) CreateTask(context.Context, storage.Task) error {
	panic("CreateTask not expected")
}
func (m *mockStore) GetTask(context.Context, string) (*storage.Task, error) {
	panic("GetTask not expected")
}
func (m *mockStore) UpdateTaskState(context.Context, string, storage.TaskState) error {
	panic("UpdateTaskState not expected")
}
func (m *mockStore) AppendEvent(context.Context, storage.Event) error {
	panic("AppendEvent not expected")
}
func (m *mockStore) ListEvents(context.Context, storage.EventFilter) ([]storage.Event, error) {
	panic("ListEvents not expected")
}
func (m *mockStore) StoreAgentToken(context.Context, string, []byte) error {
	panic("StoreAgentToken not expected")
}
func (m *mockStore) VerifyAgentToken(context.Context, string, []byte) (bool, error) {
	panic("VerifyAgentToken not expected")
}
func (m *mockStore) RevokeAgentToken(context.Context, string) error {
	panic("RevokeAgentToken not expected")
}
func (m *mockStore) Migrate(context.Context) error { panic("Migrate not expected") }
func (m *mockStore) Ping(context.Context) error    { return nil }
func (m *mockStore) Close() error                  { panic("Close not expected") }

// ---------------------------------------------------------------------------
// Mock TaskCreator — satisfies the TaskCreator interface for run tests.
// ---------------------------------------------------------------------------

type mockTaskCreator struct {
	// result to return on success.
	taskID string
	token  string

	// captured input for assertions.
	lastInput *TaskCreateInput

	// Error injection.
	createErr error
}

func (m *mockTaskCreator) CreateTask(_ context.Context, req TaskCreateInput) (*TaskCreateOutput, error) {
	m.lastInput = &req
	if m.createErr != nil {
		return nil, m.createErr
	}
	return &TaskCreateOutput{
		TaskID: m.taskID,
		Token:  m.token,
	}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// postSlashCommand builds an httptest request that looks like a Slack slash
// command POST.
func postSlashCommand(t *testing.T, bot *Bot, text string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{}
	form.Set("text", text)

	req := httptest.NewRequest(http.MethodPost, "/slack", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	bot.ServeHTTP(rec, req)
	return rec
}

// decodeSlackResponse unmarshals the recorder body into a slackResponse.
func decodeSlackResponse(t *testing.T, rec *httptest.ResponseRecorder) slackResponse {
	t.Helper()
	var resp slackResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}
	return resp
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestStatusReturnsTaskCounts(t *testing.T) {
	ms := newMockStore()
	ms.tasks = []storage.Task{
		{ID: "t1", State: storage.TaskStatePending},
		{ID: "t2", State: storage.TaskStatePending},
		{ID: "t3", State: storage.TaskStateRunning},
		{ID: "t4", State: storage.TaskStateCompleted},
		{ID: "t5", State: storage.TaskStateFailed},
		{ID: "t6", State: storage.TaskStateFailed},
		{ID: "t7", State: storage.TaskStateFailed},
	}

	bot := NewBot(ms, "", nil)
	rec := postSlashCommand(t, bot, "status")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	resp := decodeSlackResponse(t, rec)
	if resp.ResponseType != "in_channel" {
		t.Errorf("expected response_type in_channel, got %q", resp.ResponseType)
	}

	// Verify counts appear in the text.
	expectations := map[string]string{
		"pending":   "pending: 2",
		"running":   "running: 1",
		"completed": "completed: 1",
		"failed":    "failed: 3",
		"timeout":   "timeout: 0",
		"total":     "Total: 7",
	}
	for label, want := range expectations {
		if !strings.Contains(resp.Text, want) {
			t.Errorf("status text missing %s count: want substring %q in %q", label, want, resp.Text)
		}
	}
}

func TestRunCreatesTask(t *testing.T) {
	ms := newMockStore()
	mc := &mockTaskCreator{taskID: "task-42", token: "tok-abc"}

	bot := NewBot(ms, "", mc)
	rec := postSlashCommand(t, bot, `run deploy {"env":"staging"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	resp := decodeSlackResponse(t, rec)
	if resp.ResponseType != "in_channel" {
		t.Errorf("expected response_type in_channel, got %q", resp.ResponseType)
	}

	if !strings.Contains(resp.Text, "deploy") {
		t.Errorf("response should mention skill name, got %q", resp.Text)
	}
	if !strings.Contains(resp.Text, "task-42") {
		t.Errorf("response should mention task ID, got %q", resp.Text)
	}

	// Verify TaskCreator received the correct input.
	if mc.lastInput == nil {
		t.Fatal("expected TaskCreator to be called")
	}
	if mc.lastInput.SkillName != "deploy" {
		t.Errorf("expected skill_name deploy, got %q", mc.lastInput.SkillName)
	}
	if mc.lastInput.Parameters["env"] != "staging" {
		t.Errorf("expected env=staging, got %v", mc.lastInput.Parameters)
	}
	if mc.lastInput.Runtime != "local" {
		t.Errorf("expected runtime local, got %q", mc.lastInput.Runtime)
	}
	if mc.lastInput.User != "slack-user" {
		t.Errorf("expected user slack-user, got %q", mc.lastInput.User)
	}
}

func TestRunInvalidSkillReturnsError(t *testing.T) {
	ms := newMockStore()
	mc := &mockTaskCreator{createErr: fmt.Errorf("skill %q not found", "nonexistent")}

	bot := NewBot(ms, "", mc)
	rec := postSlashCommand(t, bot, "run nonexistent {}")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	resp := decodeSlackResponse(t, rec)
	if resp.ResponseType != "ephemeral" {
		t.Errorf("expected ephemeral response for error, got %q", resp.ResponseType)
	}
	// Internal error details must NOT leak to Slack.
	if strings.Contains(resp.Text, "nonexistent") {
		t.Errorf("raw error must not leak skill name, got %q", resp.Text)
	}
	if !strings.Contains(resp.Text, "Failed to submit task") {
		t.Errorf("expected friendly error message, got %q", resp.Text)
	}
}

func TestRunMalformedParamsReturnsError(t *testing.T) {
	ms := newMockStore()
	mc := &mockTaskCreator{taskID: "task-99"}

	bot := NewBot(ms, "", mc)
	rec := postSlashCommand(t, bot, "run deploy {not-valid-json}")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	resp := decodeSlackResponse(t, rec)
	if resp.ResponseType != "ephemeral" {
		t.Errorf("expected ephemeral response for error, got %q", resp.ResponseType)
	}
	if !strings.Contains(resp.Text, "Invalid JSON") {
		t.Errorf("expected invalid JSON message, got %q", resp.Text)
	}
	if !strings.Contains(resp.Text, "hello-skill") {
		t.Errorf("expected example command in message, got %q", resp.Text)
	}
}

func TestBotHelp(t *testing.T) {
	ms := newMockStore()
	bot := NewBot(ms, "", nil)
	rec := postSlashCommand(t, bot, "help")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	resp := decodeSlackResponse(t, rec)
	if resp.ResponseType != "ephemeral" {
		t.Errorf("expected ephemeral response for help, got %q", resp.ResponseType)
	}
	if !strings.Contains(resp.Text, "HermesManager Slack Commands") {
		t.Errorf("expected help header in text, got %q", resp.Text)
	}
	for _, want := range []string{"/hermes status", "/hermes run", "/hermes help"} {
		if !strings.Contains(resp.Text, want) {
			t.Errorf("expected help text to mention %q, got %q", want, resp.Text)
		}
	}
}

func TestBotHelpBareCommand(t *testing.T) {
	// An empty "text" (user types `/hermes` alone) should route to help.
	ms := newMockStore()
	bot := NewBot(ms, "", nil)
	rec := postSlashCommand(t, bot, "")

	resp := decodeSlackResponse(t, rec)
	if resp.ResponseType != "ephemeral" {
		t.Errorf("expected ephemeral, got %q", resp.ResponseType)
	}
	if !strings.Contains(resp.Text, "HermesManager Slack Commands") {
		t.Errorf("expected help on bare invocation, got %q", resp.Text)
	}
}

func TestUnknownSubCommand(t *testing.T) {
	ms := newMockStore()
	bot := NewBot(ms, "", nil)
	rec := postSlashCommand(t, bot, "foobar")

	resp := decodeSlackResponse(t, rec)
	if resp.ResponseType != "ephemeral" {
		t.Errorf("expected ephemeral, got %q", resp.ResponseType)
	}
	if !strings.Contains(resp.Text, "Unknown command") {
		t.Errorf("expected unknown command message, got %q", resp.Text)
	}
	if !strings.Contains(resp.Text, "/hermes help") {
		t.Errorf("expected pointer to /hermes help, got %q", resp.Text)
	}
	if !strings.Contains(resp.Text, "foobar") {
		t.Errorf("expected echoed subcommand, got %q", resp.Text)
	}
}

func TestRunMissingSkillName(t *testing.T) {
	ms := newMockStore()
	bot := NewBot(ms, "", nil)
	rec := postSlashCommand(t, bot, "run")

	resp := decodeSlackResponse(t, rec)
	if resp.ResponseType != "ephemeral" {
		t.Errorf("expected ephemeral, got %q", resp.ResponseType)
	}
	if !strings.Contains(resp.Text, "Usage") {
		t.Errorf("expected usage message, got %q", resp.Text)
	}
	if !strings.Contains(resp.Text, "/hermes help") {
		t.Errorf("expected pointer to /hermes help, got %q", resp.Text)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	ms := newMockStore()
	bot := NewBot(ms, "", nil)

	req := httptest.NewRequest(http.MethodGet, "/slack", nil)
	rec := httptest.NewRecorder()
	bot.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestRunNoParams(t *testing.T) {
	ms := newMockStore()
	mc := &mockTaskCreator{taskID: "task-77", token: "tok-xyz"}

	bot := NewBot(ms, "", mc)
	rec := postSlashCommand(t, bot, "run ping")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	resp := decodeSlackResponse(t, rec)
	if resp.ResponseType != "in_channel" {
		t.Errorf("expected in_channel, got %q", resp.ResponseType)
	}

	if mc.lastInput == nil {
		t.Fatal("expected TaskCreator to be called")
	}
	if mc.lastInput.SkillName != "ping" {
		t.Errorf("expected skill ping, got %q", mc.lastInput.SkillName)
	}
	if len(mc.lastInput.Parameters) != 0 {
		t.Errorf("expected empty params, got %v", mc.lastInput.Parameters)
	}
}

func TestStatusStoreError(t *testing.T) {
	ms := newMockStore()
	ms.listTasksErr = fmt.Errorf("db connection lost")

	bot := NewBot(ms, "", nil)
	rec := postSlashCommand(t, bot, "status")

	resp := decodeSlackResponse(t, rec)
	if resp.ResponseType != "ephemeral" {
		t.Errorf("expected ephemeral, got %q", resp.ResponseType)
	}
	// Original error must NOT leak to Slack; only a friendly message should.
	if strings.Contains(resp.Text, "db connection lost") {
		t.Errorf("raw error must not leak to user, got %q", resp.Text)
	}
	if !strings.Contains(resp.Text, "Failed to query task status") {
		t.Errorf("expected friendly status error, got %q", resp.Text)
	}
}

func TestRunCreateTaskError(t *testing.T) {
	ms := newMockStore()
	mc := &mockTaskCreator{createErr: fmt.Errorf("dispatch failed (task rolled back): insert failed")}

	bot := NewBot(ms, "", mc)
	rec := postSlashCommand(t, bot, `run deploy {}`)

	resp := decodeSlackResponse(t, rec)
	if resp.ResponseType != "ephemeral" {
		t.Errorf("expected ephemeral, got %q", resp.ResponseType)
	}
	if strings.Contains(resp.Text, "insert failed") {
		t.Errorf("raw error must not leak to user, got %q", resp.Text)
	}
	if !strings.Contains(resp.Text, "Failed to submit task") {
		t.Errorf("expected friendly submit error, got %q", resp.Text)
	}
}

func TestRunGetSkillError(t *testing.T) {
	ms := newMockStore()
	mc := &mockTaskCreator{createErr: fmt.Errorf("failed to lookup skill: timeout")}

	bot := NewBot(ms, "", mc)
	rec := postSlashCommand(t, bot, `run deploy {}`)

	resp := decodeSlackResponse(t, rec)
	if resp.ResponseType != "ephemeral" {
		t.Errorf("expected ephemeral, got %q", resp.ResponseType)
	}
	if strings.Contains(resp.Text, "timeout") {
		t.Errorf("raw error must not leak to user, got %q", resp.Text)
	}
	if !strings.Contains(resp.Text, "Failed to submit task") {
		t.Errorf("expected friendly submit error, got %q", resp.Text)
	}
}

func TestRunNilCreatorReturnsError(t *testing.T) {
	ms := newMockStore()
	// creator is nil — task creation should gracefully refuse.
	bot := NewBot(ms, "", nil)
	rec := postSlashCommand(t, bot, "run deploy {}")

	resp := decodeSlackResponse(t, rec)
	if resp.ResponseType != "ephemeral" {
		t.Errorf("expected ephemeral, got %q", resp.ResponseType)
	}
	if !strings.Contains(resp.Text, "not available") {
		t.Errorf("expected not-available message, got %q", resp.Text)
	}
}

func TestGenerateID(t *testing.T) {
	ts := time.Date(2026, 4, 21, 12, 0, 0, 123456789, time.UTC)
	id := GenerateIDForTest(ts)
	if !strings.HasPrefix(id, "task-") {
		t.Errorf("expected task- prefix, got %q", id)
	}
}

// ---------------------------------------------------------------------------
// Slack signature verification tests
// ---------------------------------------------------------------------------

const testSigningSecret = "test-signing-secret-abc123"

// signedSlashRequest builds an httptest request signed with the given secret.
func signedSlashRequest(t *testing.T, secret, text string, timestamp time.Time) *http.Request {
	t.Helper()
	form := url.Values{}
	form.Set("text", text)
	body := form.Encode()

	ts := strconv.FormatInt(timestamp.Unix(), 10)
	baseString := fmt.Sprintf("%s:%s:%s", slackSignatureVersion, ts, body)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(baseString))
	sig := slackSignatureVersion + "=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/slack", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Slack-Request-Timestamp", ts)
	req.Header.Set("X-Slack-Signature", sig)
	return req
}

func TestVerifySlackSignature_Valid(t *testing.T) {
	ms := newMockStore()
	bot := NewBot(ms, testSigningSecret, nil)

	req := signedSlashRequest(t, testSigningSecret, "help", time.Now())
	rec := httptest.NewRecorder()
	bot.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid signature, got %d: %s", rec.Code, rec.Body.String())
	}
	resp := decodeSlackResponse(t, rec)
	if !strings.Contains(resp.Text, "HermesManager Slack Commands") {
		t.Errorf("expected help output, got %q", resp.Text)
	}
}

func TestVerifySlackSignature_Invalid(t *testing.T) {
	ms := newMockStore()
	bot := NewBot(ms, testSigningSecret, nil)

	// Sign with wrong secret.
	req := signedSlashRequest(t, "wrong-secret", "help", time.Now())
	rec := httptest.NewRecorder()
	bot.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with invalid signature, got %d", rec.Code)
	}
}

func TestVerifySlackSignature_MissingHeaders(t *testing.T) {
	ms := newMockStore()
	bot := NewBot(ms, testSigningSecret, nil)

	// POST with no signature headers at all.
	form := url.Values{}
	form.Set("text", "help")
	req := httptest.NewRequest(http.MethodPost, "/slack", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	bot.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with missing headers, got %d", rec.Code)
	}
}

func TestVerifySlackSignature_Replay(t *testing.T) {
	ms := newMockStore()
	bot := NewBot(ms, testSigningSecret, nil)

	// Timestamp 10 minutes in the past — beyond 5 minute window.
	staleTime := time.Now().Add(-10 * time.Minute)
	req := signedSlashRequest(t, testSigningSecret, "help", staleTime)
	rec := httptest.NewRecorder()
	bot.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for stale timestamp, got %d", rec.Code)
	}
}

func TestVerifySlackSignature_DevModeSkipsVerification(t *testing.T) {
	ms := newMockStore()
	// Empty signing secret = dev mode.
	bot := NewBot(ms, "", nil)

	// No signature headers — should still work in dev mode.
	form := url.Values{}
	form.Set("text", "help")
	req := httptest.NewRequest(http.MethodPost, "/slack", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	bot.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 in dev mode, got %d: %s", rec.Code, rec.Body.String())
	}
}
