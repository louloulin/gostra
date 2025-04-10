package observability

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/louloulin/gostra/pkg/actor"
)

type mockActor struct {
	id    string
	state actor.State
}

func (m *mockActor) ID() string {
	return m.id
}

func (m *mockActor) State() actor.State {
	return m.state
}

func TestMonitorMiddleware(t *testing.T) {
	monitor := NewMonitor()
	middleware := NewMonitorMiddleware(monitor)

	mockActor := &mockActor{
		id:    "test_actor",
		state: actor.StateActive,
	}

	envelope := &actor.MessageEnvelope{
		Recipient: mockActor,
		Message:   "test_message",
	}

	ctx := context.Background()

	// Test successful message handling
	successCalled := false
	successHandler := func(ctx context.Context, env *actor.MessageEnvelope) error {
		successCalled = true
		time.Sleep(10 * time.Millisecond) // Simulate processing time
		return nil
	}

	err := middleware.Receive(ctx, envelope, successHandler)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !successCalled {
		t.Error("Success handler was not called")
	}

	// Verify metrics were recorded
	actorMetrics := monitor.GetActorMetrics()
	if am := actorMetrics[mockActor.ID()]; am == nil {
		t.Error("Expected actor metrics to exist")
	} else {
		if am.MessageCount != 1 {
			t.Errorf("Expected message count 1, got %d", am.MessageCount)
		}
		if am.ErrorCount != 0 {
			t.Errorf("Expected error count 0, got %d", am.ErrorCount)
		}
		if am.ResponseTime <= 0 {
			t.Error("Expected positive response time")
		}
		if am.State != string(mockActor.State()) {
			t.Errorf("Expected state %s, got %s", mockActor.State(), am.State)
		}
	}

	// Test error handling
	monitor.Reset()
	testError := errors.New("test error")
	errorHandler := func(ctx context.Context, env *actor.MessageEnvelope) error {
		return testError
	}

	err = middleware.Receive(ctx, envelope, errorHandler)
	if err != testError {
		t.Errorf("Expected error %v, got %v", testError, err)
	}

	// Verify error metrics were recorded
	actorMetrics = monitor.GetActorMetrics()
	if am := actorMetrics[mockActor.ID()]; am == nil {
		t.Error("Expected actor metrics to exist")
	} else {
		if am.MessageCount != 1 {
			t.Errorf("Expected message count 1, got %d", am.MessageCount)
		}
		if am.ErrorCount != 1 {
			t.Errorf("Expected error count 1, got %d", am.ErrorCount)
		}
	}

	// Test middleware factory function
	middlewareFunc := WithMonitoring(monitor)
	if middlewareFunc == nil {
		t.Error("Expected middleware function to be created")
	}
}
