package evals

import (
	"context"
	"testing"
	"time"
)

type mockAgent struct {
	id string
}

func (m *mockAgent) GetID() string {
	return m.id
}

func TestStandardEvaluator(t *testing.T) {
	config := &EvalConfig{
		Name:        "Test Evaluation",
		Description: "Testing agent evaluation",
		Timeout:     time.Second * 30,
		MaxRetries:  3,
		Metadata: map[string]interface{}{
			"test_type": "unit_test",
		},
	}

	evaluator := NewStandardEvaluator(config)
	mockAgent := &mockAgent{id: "test_agent"}

	ctx := context.Background()
	result, err := evaluator.Evaluate(ctx, mockAgent)

	if err != nil {
		t.Errorf("Evaluate() error = %v", err)
		return
	}

	// Verify basic result properties
	if result.ID != mockAgent.GetID() {
		t.Errorf("Expected ID %s, got %s", mockAgent.GetID(), result.ID)
	}

	if result.Name != config.Name {
		t.Errorf("Expected Name %s, got %s", config.Name, result.Name)
	}

	// Verify metrics exist
	expectedMetrics := []string{
		"response_time_ms",
		"relevancy_score",
		"completeness_score",
		"consistency_score",
	}

	for _, metric := range expectedMetrics {
		if _, exists := result.Metrics[metric]; !exists {
			t.Errorf("Expected metric %s not found in results", metric)
		}
	}

	// Verify score is within valid range
	if result.Score < 0 || result.Score > 1 {
		t.Errorf("Score %f is outside valid range [0,1]", result.Score)
	}

	// Verify duration is reasonable
	if result.Duration <= 0 {
		t.Error("Expected positive duration")
	}
}
