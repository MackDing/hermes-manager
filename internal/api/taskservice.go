package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/MackDing/hermes-manager/internal/policy"
	"github.com/MackDing/hermes-manager/internal/storage"
	"github.com/rs/zerolog/log"
)

// CreateTaskRequest is the input for creating a task through the shared pipeline.
// Both the HTTP API and the Slack bot use this structure.
type CreateTaskRequest struct {
	SkillName    string
	Parameters   map[string]any
	Runtime      string
	User         string
	Team         string
	DeadlineSecs int
}

// CreateTaskResult is the output of a successful task creation.
type CreateTaskResult struct {
	TaskID     string
	Runtime    string
	ExternalID string
	Token      string
}

// CreateTask runs the full task creation pipeline: skill lookup, policy
// evaluation, store persistence, agent token generation, and scheduler dispatch.
// If dispatch fails, it rolls back the task (marks it failed) and revokes the
// agent token so no zombie tasks remain.
func (s *Server) CreateTask(ctx context.Context, req CreateTaskRequest) (*CreateTaskResult, error) {
	// 1. Verify skill exists.
	skill, err := s.store.GetSkill(ctx, req.SkillName)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup skill: %w", err)
	}
	if skill == nil {
		return nil, fmt.Errorf("skill %q not found", req.SkillName)
	}

	// 2. Policy evaluation.
	if s.policy != nil {
		pReq := policy.PolicyRequest{
			User: req.User,
			Team: req.Team,
		}
		if len(skill.RequiredModels) > 0 {
			pReq.Model = skill.RequiredModels[0]
		}
		allowed, ruleID, err := s.policy.Evaluate(ctx, pReq)
		if err != nil {
			return nil, fmt.Errorf("policy evaluation failed: %w", err)
		}
		if !allowed {
			return nil, &PolicyDeniedError{RuleID: ruleID}
		}
	}

	// 3. Build and persist task.
	rt := req.Runtime
	if rt == "" {
		rt = "local"
	}
	deadline := req.DeadlineSecs
	if deadline <= 0 {
		deadline = 300
	}

	taskID := generateID("task")
	task := storage.Task{
		ID:         taskID,
		SkillName:  req.SkillName,
		Parameters: req.Parameters,
		PolicyContext: storage.PolicyContext{
			User:         req.User,
			Team:         req.Team,
			ModelAllowed: skill.RequiredModels,
		},
		Runtime:      rt,
		State:        storage.TaskStatePending,
		CreatedAt:    time.Now(),
		DeadlineSecs: deadline,
	}

	if err := s.store.CreateTask(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	// 4. Generate and store agent token.
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}
	tokenStr := hex.EncodeToString(tokenBytes)
	tokenHash := sha256.Sum256([]byte(tokenStr))

	if err := s.store.StoreAgentToken(ctx, taskID, tokenHash[:]); err != nil {
		return nil, fmt.Errorf("failed to store agent token: %w", err)
	}

	result := &CreateTaskResult{
		TaskID:  taskID,
		Runtime: rt,
		Token:   tokenStr,
	}

	// 5. Dispatch via scheduler (with rollback on failure).
	if s.scheduler != nil {
		handle, err := s.scheduler.Dispatch(ctx, task, *skill)
		if err != nil {
			// Rollback: mark task as failed and revoke token.
			if rbErr := s.store.UpdateTaskState(ctx, taskID, storage.TaskStateFailed); rbErr != nil {
				log.Error().Err(rbErr).Str("task_id", taskID).Msg("rollback: failed to mark task as failed")
			}
			if rbErr := s.store.RevokeAgentToken(ctx, taskID); rbErr != nil {
				log.Error().Err(rbErr).Str("task_id", taskID).Msg("rollback: failed to revoke agent token")
			}
			return nil, fmt.Errorf("dispatch failed (task rolled back): %w", err)
		}
		result.ExternalID = handle.ExternalID
		result.Runtime = handle.RuntimeName
	}

	return result, nil
}

// PolicyDeniedError is returned when a policy rule blocks task creation.
type PolicyDeniedError struct {
	RuleID string
}

func (e *PolicyDeniedError) Error() string {
	return fmt.Sprintf("blocked by policy rule: %s", e.RuleID)
}

// IsPolicyDenied checks whether an error is a PolicyDeniedError.
func IsPolicyDenied(err error) (*PolicyDeniedError, bool) {
	var pde *PolicyDeniedError
	if errors.As(err, &pde) {
		return pde, true
	}
	return nil, false
}
