package policy

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// helper writes YAML content to a temp file and returns the path.
func writeTempPolicy(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp policy: %v", err)
	}
	return path
}

func TestDenyRuleBlocks(t *testing.T) {
	yaml := `
rules:
  - id: block-gpt4-free
    action: deny
    conditions:
      model: gpt-4
      team: free-tier
`
	path := writeTempPolicy(t, yaml)
	eng, err := NewEngine(path)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	allowed, ruleID, err := eng.Evaluate(context.Background(), PolicyRequest{
		Model: "gpt-4",
		Team:  "free-tier",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if allowed {
		t.Fatal("expected request to be denied")
	}
	if ruleID != "block-gpt4-free" {
		t.Fatalf("expected ruleID block-gpt4-free, got %s", ruleID)
	}
}

func TestAllowWhenNoMatch(t *testing.T) {
	yaml := `
rules:
  - id: block-gpt4-free
    action: deny
    conditions:
      model: gpt-4
      team: free-tier
`
	path := writeTempPolicy(t, yaml)
	eng, err := NewEngine(path)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	// Different model — should not match the deny rule.
	allowed, ruleID, err := eng.Evaluate(context.Background(), PolicyRequest{
		Model: "gpt-3.5-turbo",
		Team:  "free-tier",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !allowed {
		t.Fatalf("expected request to be allowed, denied by %s", ruleID)
	}
	if ruleID != "" {
		t.Fatalf("expected empty ruleID, got %s", ruleID)
	}
}

func TestMultipleDenyFirstWins(t *testing.T) {
	yaml := `
rules:
  - id: rule-a
    action: deny
    conditions:
      model: gpt-4
  - id: rule-b
    action: deny
    conditions:
      team: free-tier
`
	path := writeTempPolicy(t, yaml)
	eng, err := NewEngine(path)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	// Both rules match; first deny wins.
	allowed, ruleID, err := eng.Evaluate(context.Background(), PolicyRequest{
		Model: "gpt-4",
		Team:  "free-tier",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if allowed {
		t.Fatal("expected request to be denied")
	}
	if ruleID != "rule-a" {
		t.Fatalf("expected first matching rule rule-a, got %s", ruleID)
	}
}

func TestLoadFromFile(t *testing.T) {
	yaml := `
rules:
  - id: deny-tool
    action: deny
    conditions:
      tool: shell-exec
`
	path := writeTempPolicy(t, yaml)
	eng, err := NewEngine("")
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	// Before loading — everything allowed.
	allowed, _, err := eng.Evaluate(context.Background(), PolicyRequest{Tool: "shell-exec"})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !allowed {
		t.Fatal("expected allowed before loading rules")
	}

	// Load file.
	if err := eng.LoadFromFile(path); err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}

	allowed, ruleID, err := eng.Evaluate(context.Background(), PolicyRequest{Tool: "shell-exec"})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if allowed {
		t.Fatal("expected denied after loading rules")
	}
	if ruleID != "deny-tool" {
		t.Fatalf("expected deny-tool, got %s", ruleID)
	}
}

func TestReloadUpdatesRules(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")

	// Start with one rule.
	v1 := `
rules:
  - id: v1-rule
    action: deny
    conditions:
      model: gpt-4
`
	if err := os.WriteFile(path, []byte(v1), 0644); err != nil {
		t.Fatal(err)
	}

	eng, err := NewEngine(path)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	allowed, ruleID, _ := eng.Evaluate(context.Background(), PolicyRequest{Model: "gpt-4"})
	if allowed {
		t.Fatal("expected denied by v1-rule")
	}
	if ruleID != "v1-rule" {
		t.Fatalf("expected v1-rule, got %s", ruleID)
	}

	// Overwrite file with different rule.
	v2 := `
rules:
  - id: v2-rule
    action: deny
    conditions:
      user: admin
`
	if err := os.WriteFile(path, []byte(v2), 0644); err != nil {
		t.Fatal(err)
	}

	if err := eng.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	// v1 rule should no longer match.
	allowed, _, _ = eng.Evaluate(context.Background(), PolicyRequest{Model: "gpt-4"})
	if !allowed {
		t.Fatal("expected gpt-4 allowed after reload")
	}

	// v2 rule should match.
	allowed, ruleID, _ = eng.Evaluate(context.Background(), PolicyRequest{User: "admin"})
	if allowed {
		t.Fatal("expected denied by v2-rule")
	}
	if ruleID != "v2-rule" {
		t.Fatalf("expected v2-rule, got %s", ruleID)
	}
}

func TestEmptyPolicyAllowsAll(t *testing.T) {
	yaml := `rules: []`
	path := writeTempPolicy(t, yaml)
	eng, err := NewEngine(path)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	allowed, ruleID, err := eng.Evaluate(context.Background(), PolicyRequest{
		Model:   "gpt-4",
		User:    "root",
		Team:    "admin",
		Tool:    "anything",
		CostUSD: 999.99,
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !allowed {
		t.Fatalf("expected allowed with empty rules, denied by %s", ruleID)
	}
}

func TestMalformedYAMLError(t *testing.T) {
	path := writeTempPolicy(t, `{{{not valid yaml!!!`)
	_, err := NewEngine(path)
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
}

func TestFileNotFoundError(t *testing.T) {
	_, err := NewEngine("/nonexistent/path/policy.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReloadNoPathError(t *testing.T) {
	eng, err := NewEngine("")
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := eng.Reload(); err == nil {
		t.Fatal("expected error when reloading with no file path")
	}
}

func TestAllowRuleDoesNotBlock(t *testing.T) {
	yaml := `
rules:
  - id: allow-gpt4
    action: allow
    conditions:
      model: gpt-4
`
	path := writeTempPolicy(t, yaml)
	eng, err := NewEngine(path)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	// Allow rules do not block; only deny rules can deny.
	allowed, ruleID, err := eng.Evaluate(context.Background(), PolicyRequest{Model: "gpt-4"})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !allowed {
		t.Fatalf("allow rule should not block, denied by %s", ruleID)
	}
}

func TestAllowBeforeDenyShortCircuits(t *testing.T) {
	yaml := `
rules:
  - id: allow-admin
    action: allow
    conditions:
      user: admin
  - id: deny-gpt4
    action: deny
    conditions:
      model: gpt-4
`
	path := writeTempPolicy(t, yaml)
	eng, err := NewEngine(path)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	// Admin user requesting gpt-4: allow rule matches first, deny rule never reached.
	allowed, ruleID, err := eng.Evaluate(context.Background(), PolicyRequest{
		Model: "gpt-4",
		User:  "admin",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !allowed {
		t.Fatalf("expected allow (admin exemption), denied by %s", ruleID)
	}

	// Non-admin user requesting gpt-4: allow rule does not match, deny rule matches.
	allowed, ruleID, err = eng.Evaluate(context.Background(), PolicyRequest{
		Model: "gpt-4",
		User:  "bob",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if allowed {
		t.Fatal("expected deny for non-admin gpt-4 request")
	}
	if ruleID != "deny-gpt4" {
		t.Fatalf("expected deny-gpt4, got %s", ruleID)
	}
}

func TestDenyBeforeAllowWins(t *testing.T) {
	yaml := `
rules:
  - id: deny-free
    action: deny
    conditions:
      team: free-tier
  - id: allow-gpt4
    action: allow
    conditions:
      model: gpt-4
`
	path := writeTempPolicy(t, yaml)
	eng, err := NewEngine(path)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	// free-tier requesting gpt-4: deny rule matches first (first-match-wins).
	allowed, ruleID, err := eng.Evaluate(context.Background(), PolicyRequest{
		Model: "gpt-4",
		Team:  "free-tier",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if allowed {
		t.Fatal("expected deny (deny rule ordered before allow)")
	}
	if ruleID != "deny-free" {
		t.Fatalf("expected deny-free, got %s", ruleID)
	}
}

func TestUnknownActionReturnsError(t *testing.T) {
	yaml := `
rules:
  - id: bad-action
    action: quarantine
    conditions:
      model: gpt-4
`
	path := writeTempPolicy(t, yaml)
	eng, err := NewEngine(path)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	_, _, err = eng.Evaluate(context.Background(), PolicyRequest{Model: "gpt-4"})
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestUnknownConditionKeyDoesNotMatch(t *testing.T) {
	yaml := `
rules:
  - id: bad-key
    action: deny
    conditions:
      nonexistent_field: value
`
	path := writeTempPolicy(t, yaml)
	eng, err := NewEngine(path)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	allowed, _, err := eng.Evaluate(context.Background(), PolicyRequest{Model: "gpt-4"})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !allowed {
		t.Fatal("unknown condition key should not match any request")
	}
}

func TestCostConditionMatch(t *testing.T) {
	yaml := `
rules:
  - id: cost-limit
    action: deny
    conditions:
      cost_usd: "10.5"
`
	path := writeTempPolicy(t, yaml)
	eng, err := NewEngine(path)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	// Exact match.
	allowed, ruleID, err := eng.Evaluate(context.Background(), PolicyRequest{CostUSD: 10.5})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if allowed {
		t.Fatal("expected denied for exact cost match")
	}
	if ruleID != "cost-limit" {
		t.Fatalf("expected cost-limit, got %s", ruleID)
	}

	// Different cost — should be allowed.
	allowed, _, err = eng.Evaluate(context.Background(), PolicyRequest{CostUSD: 5.0})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !allowed {
		t.Fatal("expected allowed for different cost")
	}
}

func TestEmptyConditionsMatchesAll(t *testing.T) {
	yaml := `
rules:
  - id: deny-all
    action: deny
    conditions: {}
`
	path := writeTempPolicy(t, yaml)
	eng, err := NewEngine(path)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	allowed, ruleID, err := eng.Evaluate(context.Background(), PolicyRequest{Model: "anything"})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if allowed {
		t.Fatal("empty conditions should match everything")
	}
	if ruleID != "deny-all" {
		t.Fatalf("expected deny-all, got %s", ruleID)
	}
}

func TestNoRulesFileAllowsAll(t *testing.T) {
	eng, err := NewEngine("")
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	allowed, _, err := eng.Evaluate(context.Background(), PolicyRequest{Model: "gpt-4"})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !allowed {
		t.Fatal("expected allowed with no file loaded")
	}
}

func TestPartialConditionMismatch(t *testing.T) {
	yaml := `
rules:
  - id: partial
    action: deny
    conditions:
      model: gpt-4
      team: paid
      user: alice
`
	path := writeTempPolicy(t, yaml)
	eng, err := NewEngine(path)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	// Only two of three conditions match — should be allowed.
	allowed, _, err := eng.Evaluate(context.Background(), PolicyRequest{
		Model: "gpt-4",
		Team:  "paid",
		User:  "bob",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !allowed {
		t.Fatal("expected allowed when not all conditions match")
	}

	// All three match — should be denied.
	allowed, ruleID, err := eng.Evaluate(context.Background(), PolicyRequest{
		Model: "gpt-4",
		Team:  "paid",
		User:  "alice",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if allowed {
		t.Fatal("expected denied when all conditions match")
	}
	if ruleID != "partial" {
		t.Fatalf("expected partial, got %s", ruleID)
	}
}
