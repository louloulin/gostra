package errors

import (
	"context"
	"sync"
	"time"

	"github.com/asynkron/protoactor-go/actor"
)

// Supervisor manages error recovery for actors
type Supervisor struct {
	mu sync.RWMutex

	// Map of actor PIDs to their recovery states
	recoveryStates map[*actor.PID]*RecoveryState

	// Default policy applied to all supervised actors
	defaultPolicy *RecoveryPolicy

	// Custom policies for specific actor types
	customPolicies map[string]*RecoveryPolicy

	// Actor system reference
	system *actor.ActorSystem
}

// NewSupervisor creates a new supervisor
func NewSupervisor(system *actor.ActorSystem, defaultPolicy *RecoveryPolicy) *Supervisor {
	if defaultPolicy == nil {
		defaultPolicy = &RecoveryPolicy{
			MaxRetries: 3,
			RetryDelay: time.Second,
			Strategy:   RestartActor,
			ResetAfter: time.Minute * 5,
		}
	}

	return &Supervisor{
		system:         system,
		recoveryStates: make(map[*actor.PID]*RecoveryState),
		defaultPolicy:  defaultPolicy,
		customPolicies: make(map[string]*RecoveryPolicy),
	}
}

// RegisterPolicy registers a custom recovery policy for an actor type
func (s *Supervisor) RegisterPolicy(actorType string, policy *RecoveryPolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.customPolicies[actorType] = policy
}

// getPolicy returns the appropriate policy for an actor
func (s *Supervisor) getPolicy(actorType string) *RecoveryPolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if policy, exists := s.customPolicies[actorType]; exists {
		return policy
	}
	return s.defaultPolicy
}

// getRecoveryState gets or creates a recovery state for an actor
func (s *Supervisor) getRecoveryState(pid *actor.PID) *RecoveryState {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, exists := s.recoveryStates[pid]
	if !exists {
		state = NewRecoveryState()
		s.recoveryStates[pid] = state
	}
	return state
}

// HandleFailure handles actor failures and implements recovery
func (s *Supervisor) HandleFailure(ctx context.Context, pid *actor.PID, actorType string, err error) error {
	// Convert error to AgentError if needed
	agentErr, ok := err.(*AgentError)
	if !ok {
		agentErr = &AgentError{
			Type:        SystemError,
			Severity:    Error,
			Message:     err.Error(),
			Recoverable: true,
		}
	}

	policy := s.getPolicy(actorType)
	state := s.getRecoveryState(pid)

	if !state.ShouldRecover(policy, agentErr) {
		return agentErr
	}

	state.StartRecovery(ctx, policy, agentErr)

	// Apply recovery strategy
	var recoveryErr error
	switch policy.Strategy {
	case RestartActor:
		recoveryErr = s.restartActor(ctx, pid)
	case ResumeActor:
		recoveryErr = s.resumeActor(ctx, pid)
	case StopActor:
		recoveryErr = s.stopActor(ctx, pid)
	case EscalateError:
		recoveryErr = agentErr
	}

	// Wait for retry delay if specified
	if policy.RetryDelay > 0 {
		time.Sleep(policy.RetryDelay)
	}

	state.EndRecovery(ctx, policy, recoveryErr)
	return recoveryErr
}

// restartActor restarts a failed actor
func (s *Supervisor) restartActor(ctx context.Context, pid *actor.PID) error {
	// Stop the actor using actor system
	s.system.Root.Stop(pid)

	// Create new instance using the actor system
	if _, err := s.system.Root.SpawnNamed(actor.PropsFromProducer(nil), pid.Id); err != nil {
		return &AgentError{
			Type:        SystemError,
			Severity:    Error,
			Message:     "Failed to restart actor",
			Cause:       err,
			Recoverable: false,
		}
	}

	return nil
}

// resumeActor resumes a failed actor
func (s *Supervisor) resumeActor(ctx context.Context, pid *actor.PID) error {
	// Use actor system to send resume message
	s.system.Root.Send(pid, &actor.Resume{})
	return nil
}

// stopActor stops a failed actor
func (s *Supervisor) stopActor(ctx context.Context, pid *actor.PID) error {
	s.system.Root.Stop(pid)
	return nil
}

// RemoveActor removes an actor from supervision
func (s *Supervisor) RemoveActor(pid *actor.PID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.recoveryStates, pid)
}
