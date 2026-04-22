package policy

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"

	"gopkg.in/yaml.v3"
)

// Engine evaluates policy rules loaded from a YAML file.
// It is safe for concurrent use after creation.
type Engine struct {
	mu       sync.RWMutex
	rules    []Rule
	filePath string
}

// NewEngine creates a policy engine and loads rules from the given YAML file.
// If filePath is empty, the engine starts with zero rules (allow-all).
func NewEngine(filePath string) (*Engine, error) {
	e := &Engine{filePath: filePath}
	if filePath != "" {
		if err := e.LoadFromFile(filePath); err != nil {
			return nil, err
		}
	}
	return e, nil
}

// LoadFromFile parses the YAML policy file at path and replaces the current rules.
func (e *Engine) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("policy: read file %s: %w", path, err)
	}

	var pf PolicyFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return fmt.Errorf("policy: parse YAML %s: %w", path, err)
	}

	e.mu.Lock()
	e.rules = pf.Rules
	e.filePath = path
	e.mu.Unlock()

	return nil
}

// Reload re-reads the same file path that was last loaded.
func (e *Engine) Reload() error {
	e.mu.RLock()
	path := e.filePath
	e.mu.RUnlock()

	if path == "" {
		return fmt.Errorf("policy: no file path configured for reload")
	}
	return e.LoadFromFile(path)
}

// Evaluate checks the request against all loaded rules using first-match-wins semantics.
// Rules are evaluated in order; the first rule whose conditions match determines the outcome.
// If the matching rule's action is "deny", the request is denied and the rule's ID is returned.
// If the matching rule's action is "allow", the request is explicitly allowed.
// If no rule matches, the request is allowed (default-allow).
func (e *Engine) Evaluate(_ context.Context, req PolicyRequest) (allowed bool, deniedByRuleID string, err error) {
	e.mu.RLock()
	rules := e.rules
	e.mu.RUnlock()

	for i := range rules {
		if e.matchRule(rules[i], req) {
			switch rules[i].Action {
			case "deny":
				return false, rules[i].ID, nil
			case "allow":
				return true, "", nil
			default:
				return false, rules[i].ID, fmt.Errorf("policy: unknown action %q in rule %s", rules[i].Action, rules[i].ID)
			}
		}
	}

	return true, "", nil
}

// matchRule returns true if every condition in the rule matches the corresponding
// field in the request. An empty conditions map matches everything.
func (e *Engine) matchRule(rule Rule, req PolicyRequest) bool {
	for key, expected := range rule.Conditions {
		var actual string
		switch key {
		case "model":
			actual = req.Model
		case "user":
			actual = req.User
		case "team":
			actual = req.Team
		case "tool":
			actual = req.Tool
		case "cost_usd":
			actual = strconv.FormatFloat(req.CostUSD, 'f', -1, 64)
		default:
			// Unknown condition key never matches.
			return false
		}
		if actual != expected {
			return false
		}
	}
	return true
}
