package errors

import (
	"context"
	"sync"
	"time"
)

// RecoveryStrategy defines how to handle actor failures
type RecoveryStrategy int

const (
	// Different recovery strategies
	RestartActor RecoveryStrategy = iota
	ResumeActor
	StopActor
	EscalateError
)

// RecoveryPolicy defines how to recover from errors
type RecoveryPolicy struct {
	MaxRetries      int
	RetryDelay      time.Duration
	Strategy        RecoveryStrategy
	ResetAfter      time.Duration
	OnlyFor         []ErrorType
	ExceptFor       []ErrorType
	OnRecoveryStart func(context.Context, *AgentError)
	OnRecoveryEnd   func(context.Context, error)
}

// RecoveryState tracks recovery attempts
type RecoveryState struct {
	mu             sync.Mutex
	retryCount     int
	lastError      *AgentError
	lastRetryTime  time.Time
	recoveryActive bool
}

// NewRecoveryState creates a new recovery state
func NewRecoveryState() *RecoveryState {
	return &RecoveryState{}
}

// ShouldRecover determines if recovery should be attempted
func (r *RecoveryState) ShouldRecover(policy *RecoveryPolicy, err *AgentError) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if error type is allowed
	if len(policy.OnlyFor) > 0 {
		allowed := false
		for _, t := range policy.OnlyFor {
			if t == err.Type {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}

	// Check if error type is excluded
	for _, t := range policy.ExceptFor {
		if t == err.Type {
			return false
		}
	}

	// Check if max retries exceeded
	if policy.MaxRetries > 0 && r.retryCount >= policy.MaxRetries {
		return false
	}

	// Check if we should reset retry count
	if policy.ResetAfter > 0 && time.Since(r.lastRetryTime) > policy.ResetAfter {
		r.retryCount = 0
	}

	return err.Recoverable
}

// StartRecovery begins a recovery attempt
func (r *RecoveryState) StartRecovery(ctx context.Context, policy *RecoveryPolicy, err *AgentError) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.retryCount++
	r.lastError = err
	r.lastRetryTime = time.Now()
	r.recoveryActive = true

	if policy.OnRecoveryStart != nil {
		policy.OnRecoveryStart(ctx, err)
	}
}

// EndRecovery ends a recovery attempt
func (r *RecoveryState) EndRecovery(ctx context.Context, policy *RecoveryPolicy, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.recoveryActive = false

	if policy.OnRecoveryEnd != nil {
		policy.OnRecoveryEnd(ctx, err)
	}
}

// IsRecovering checks if recovery is in progress
func (r *RecoveryState) IsRecovering() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.recoveryActive
}

// GetRetryCount returns the current retry count
func (r *RecoveryState) GetRetryCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.retryCount
}

// GetLastError returns the last error encountered
func (r *RecoveryState) GetLastError() *AgentError {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastError
}
