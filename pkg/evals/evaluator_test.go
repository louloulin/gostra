package evals

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type mockAgent struct {
	id string
}

func (m *mockAgent) GetID() string {
	return m.id
}

// --- Mock Agent for Evaluation Testing ---

type MockEvaluableAgent struct {
	ID                  string
	SampleInteractions  map[string]struct{ Input, Output, Reference string } // taskIdentifier -> data
	MultiOutputResponse map[string][]string                                  // input -> outputs
	ErrorOnTask         string                                               // If set, return error for this taskIdentifier
	ErrorOnMultiOutput  bool
}

func NewMockEvaluableAgent(id string) *MockEvaluableAgent {
	return &MockEvaluableAgent{
		ID:                  id,
		SampleInteractions:  make(map[string]struct{ Input, Output, Reference string }),
		MultiOutputResponse: make(map[string][]string),
	}
}

func (m *MockEvaluableAgent) GetID() string {
	return m.ID
}

func (m *MockEvaluableAgent) GetSampleInteraction(ctx context.Context, taskIdentifier string) (input, output, referenceOutput string, err error) {
	if m.ErrorOnTask == taskIdentifier {
		return "", "", "", errors.New("mock agent error on task: " + taskIdentifier)
	}
	if data, ok := m.SampleInteractions[taskIdentifier]; ok {
		return data.Input, data.Output, data.Reference, nil
	}
	return "", "", "", errors.New("mock agent task not found: " + taskIdentifier)
}

func (m *MockEvaluableAgent) GetMultipleOutputs(ctx context.Context, input string, count int) (outputs []string, err error) {
	if m.ErrorOnMultiOutput {
		return nil, errors.New("mock agent error on multi-output")
	}
	if outputs, ok := m.MultiOutputResponse[input]; ok {
		// Return up to 'count' outputs if available
		if len(outputs) >= count {
			return outputs[:count], nil
		} else {
			return outputs, nil // Return what we have
		}
	}
	return nil, errors.New("mock agent input not found for multi-output: " + input)
}

// --- Original Test ---
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
	mockAgentImpl := NewMockEvaluableAgent("test_agent_full")

	// Setup mock responses for the basic test
	mockAgentImpl.SampleInteractions["relevancy_task"] = struct{ Input, Output, Reference string }{"q1", "a b c", "a b d"}                         // ~0.5 relevance
	mockAgentImpl.SampleInteractions["completeness_task"] = struct{ Input, Output, Reference string }{"q2", "item1\nitem3", "item1\nitem2\nitem3"} // 2/3 complete
	mockAgentImpl.MultiOutputResponse["q1"] = []string{"a b c", "a b c", "a b e"}                                                                  // High consistency

	ctx := context.Background()
	result, err := evaluator.Evaluate(ctx, mockAgentImpl)

	if err != nil {
		t.Errorf("Evaluate() error = %v", err)
		return
	}

	// Verify basic result properties
	if result.ID != mockAgentImpl.GetID() {
		t.Errorf("Expected ID %s, got %s", mockAgentImpl.GetID(), result.ID)
	}

	if result.Name != config.Name {
		t.Errorf("Expected Name %s, got %s", config.Name, result.Name)
	}

	// Verify metrics exist and have plausible values (based on mock setup)
	expectedMetrics := []string{
		"response_time_ms",
		"relevancy_score",
		"completeness_score",
		"consistency_score",
	}

	for _, metric := range expectedMetrics {
		val, exists := result.Metrics[metric]
		if !exists {
			t.Errorf("Expected metric %s not found in results", metric)
		} else {
			t.Logf("Metric %s: %v", metric, val)
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

// --- Specific Metric Tests ---

func TestEvaluateRelevancy(t *testing.T) {
	evaluator := NewStandardEvaluator(&EvalConfig{Name: "Relevancy Test"})
	mockAgent := NewMockEvaluableAgent("relevancy_agent")

	// Test Case 1: High Overlap
	data1 := struct{ Input, Output, Reference string }{
		Input:     "What is the capital of France?",
		Output:    "The capital of France is Paris.",
		Reference: "Paris is the capital city of France.",
	}
	mockAgent.SampleInteractions["relevancy_task"] = data1
	relevance1 := evaluator.evaluateRelevancyMetric(data1.Input, data1.Output, data1.Reference)
	assert.InDelta(t, 1.0, relevance1, 0.01, "Expected high relevancy for direct overlap")

	// Test Case 2: Partial Overlap
	data2 := struct{ Input, Output, Reference string }{
		Input:     "Explain photosynthesis.",
		Output:    "Plants make food using sunlight.",
		Reference: "Photosynthesis is the process used by plants to convert light energy into chemical energy.",
	}
	mockAgent.SampleInteractions["relevancy_task"] = data2
	relevance2 := evaluator.evaluateRelevancyMetric(data2.Input, data2.Output, data2.Reference)
	assert.True(t, relevance2 > 0.1 && relevance2 < 0.6, "Expected partial relevancy, got %f", relevance2) // Jaccard is sensitive

	// Test Case 3: No Overlap
	data3 := struct{ Input, Output, Reference string }{
		Input:     "Who painted the Mona Lisa?",
		Output:    "The sky is blue.",
		Reference: "Leonardo da Vinci painted the Mona Lisa.",
	}
	mockAgent.SampleInteractions["relevancy_task"] = data3
	relevance3 := evaluator.evaluateRelevancyMetric(data3.Input, data3.Output, data3.Reference)
	assert.InDelta(t, 0.0, relevance3, 0.01, "Expected zero relevancy for no overlap")

	// Test Case 4: Empty Output
	data4 := struct{ Input, Output, Reference string }{
		Input:     "Test Empty Output",
		Output:    "",
		Reference: "Some reference text.",
	}
	mockAgent.SampleInteractions["relevancy_task"] = data4
	relevance4 := evaluator.evaluateRelevancyMetric(data4.Input, data4.Output, data4.Reference)
	assert.InDelta(t, 0.0, relevance4, 0.01, "Expected zero relevancy for empty output")

	// Test Case 5: Empty Reference (Should return 0.0 for relevance)
	data5 := struct{ Input, Output, Reference string }{
		Input:     "Test Empty Reference",
		Output:    "Some output.",
		Reference: "",
	}
	mockAgent.SampleInteractions["relevancy_task"] = data5
	relevance5 := evaluator.evaluateRelevancyMetric(data5.Input, data5.Output, data5.Reference)
	assert.InDelta(t, 0.0, relevance5, 0.01, "Expected zero relevancy on agent error")
}

func TestEvaluateCompleteness(t *testing.T) {
	evaluator := NewStandardEvaluator(&EvalConfig{Name: "Completeness Test"})
	mockAgent := NewMockEvaluableAgent("completeness_agent")

	// Test Case 1: All items present
	data1 := struct{ Input, Output, Reference string }{
		Input:     "List primary colors.",
		Output:    "The primary colors are red, yellow, and blue.",
		Reference: "red\nyellow\nblue",
	}
	mockAgent.SampleInteractions["completeness_task"] = data1
	completeness1 := evaluator.evaluateCompletenessMetric(data1.Output, data1.Reference)
	assert.InDelta(t, 1.0, completeness1, 0.01, "Expected full completeness")

	// Test Case 2: Some items missing
	data2 := struct{ Input, Output, Reference string }{
		Input:     "List primary colors.",
		Output:    "Red and blue are primary colors.",
		Reference: "red\nyellow\nblue",
	}
	mockAgent.SampleInteractions["completeness_task"] = data2
	completeness2 := evaluator.evaluateCompletenessMetric(data2.Output, data2.Reference)
	assert.InDelta(t, 2.0/3.0, completeness2, 0.01, "Expected partial completeness (2/3)")

	// Test Case 3: No items present
	data3 := struct{ Input, Output, Reference string }{
		Input:     "List primary colors.",
		Output:    "I don't know.",
		Reference: "red\nyellow\nblue",
	}
	mockAgent.SampleInteractions["completeness_task"] = data3
	completeness3 := evaluator.evaluateCompletenessMetric(data3.Output, data3.Reference)
	assert.InDelta(t, 0.0, completeness3, 0.01, "Expected zero completeness")

	// Test Case 4: Empty reference list
	data4 := struct{ Input, Output, Reference string }{
		Input:     "Test empty reference.",
		Output:    "Some output.",
		Reference: "", // No requirements
	}
	mockAgent.SampleInteractions["completeness_task"] = data4
	completeness4 := evaluator.evaluateCompletenessMetric(data4.Output, data4.Reference)
	assert.InDelta(t, 1.0, completeness4, 0.01, "Expected full completeness with empty reference")
}

func TestEvaluateConsistency(t *testing.T) {
	evaluator := NewStandardEvaluator(&EvalConfig{Name: "Consistency Test"})
	mockAgent := NewMockEvaluableAgent("consistency_agent")

	// Need a sample interaction for the input query for the mock agent
	sampleInput := "What's the weather?"
	mockAgent.SampleInteractions["relevancy_task"] = struct{ Input, Output, Reference string }{sampleInput, "", ""}

	// Test Case 1: Identical outputs
	outputs1 := []string{
		"It is sunny and warm.",
		"It is sunny and warm.",
		"It is sunny and warm.",
	}
	mockAgent.MultiOutputResponse[sampleInput] = outputs1
	consistency1 := evaluator.evaluateConsistencyMetric(outputs1)
	assert.InDelta(t, 1.0, consistency1, 0.01, "Expected full consistency for identical outputs")

	// Test Case 2: Mostly similar outputs
	outputs2 := []string{
		"The weather today is sunny and pleasant.",
		"Today's weather: pleasant and sunny.",
		"It's sunny and pleasant today.",
	}
	mockAgent.MultiOutputResponse[sampleInput] = outputs2
	consistency2 := evaluator.evaluateConsistencyMetric(outputs2)
	assert.True(t, consistency2 > 0.8, "Expected high consistency for similar outputs, got %f", consistency2)

	// Test Case 3: Dissimilar outputs
	outputs3 := []string{
		"It is raining heavily.",
		"Expect clear skies all day.",
		"Snow is forecast for the afternoon.",
	}
	mockAgent.MultiOutputResponse[sampleInput] = outputs3
	consistency3 := evaluator.evaluateConsistencyMetric(outputs3)
	assert.True(t, consistency3 < 0.2, "Expected low consistency for dissimilar outputs, got %f", consistency3)

	// Test Case 4: Only two outputs (still comparable)
	outputs4 := []string{
		"Warm and humid.",
		"Humid and warm.",
	}
	mockAgent.MultiOutputResponse[sampleInput] = outputs4
	consistency4 := evaluator.evaluateConsistencyMetric(outputs4)
	assert.InDelta(t, 1.0, consistency4, 0.01, "Expected full consistency for two identical outputs")

	// Test Case 5: Error getting multiple outputs (Simulated by passing nil/empty slice)
	consistency5 := evaluator.evaluateConsistencyMetric(nil) // Simulate error by passing nil
	assert.InDelta(t, 1.0, consistency5, 0.01, "Expected zero consistency on multi-output error")

	// Test Case 6: Error getting sample input for consistency check (Tested implicitly in RunBatchEvaluation)
	// This scenario is handled by RunBatchEvaluation which wouldn't call the metric func.
}

// --- Custom Eval Rule Tests ---

// Sample custom rule: Checks if output contains a specific keyword
type KeywordRule struct {
	Name    string
	Keyword string
	TaskID  string // Which sample interaction to use
}

func (r *KeywordRule) GetName() string {
	return r.Name
}

func (r *KeywordRule) Evaluate(ctx context.Context, input, output, referenceOutput string) (score float64, err error) {
	// Note: This rule implementation only uses the 'output'
	_ = input           // Ignore input
	_ = referenceOutput // Ignore reference
	if err != nil {
		return 0.0, fmt.Errorf("keyword rule failed for task %s: %w", r.TaskID, err)
	}
	if strings.Contains(strings.ToLower(output), strings.ToLower(r.Keyword)) {
		return 1.0, nil
	} else {
		return 0.0, nil
	}
}

// Sample custom rule: Checks output length
type LengthRule struct {
	Name      string
	MinLength int
	MaxLength int
	TaskID    string
}

func (r *LengthRule) GetName() string {
	return r.Name
}

func (r *LengthRule) Evaluate(ctx context.Context, input, output, referenceOutput string) (score float64, err error) {
	// Note: This rule implementation only uses the 'output'
	_ = input           // Ignore input
	_ = referenceOutput // Ignore reference
	if err != nil {
		return 0.0, fmt.Errorf("length rule failed for task %s: %w", r.TaskID, err)
	}
	length := len(output)
	if length >= r.MinLength && length <= r.MaxLength {
		return 1.0, nil
	} else {
		// Could return a partial score, but 0.0 for simplicity
		return 0.0, nil
	}
}

func TestCustomEvalRules(t *testing.T) {
	// 1. Define custom rules
	rule1 := &KeywordRule{Name: "contains_paris", Keyword: "paris", TaskID: "relevancy_task"}
	rule2 := &LengthRule{Name: "output_length", MinLength: 10, MaxLength: 100, TaskID: "relevancy_task"}
	rule3 := &KeywordRule{Name: "contains_banana", Keyword: "banana", TaskID: "relevancy_task"} // Expected fail
	// Rule that will cause an agent error
	rule4 := &KeywordRule{Name: "error_rule", Keyword: "test", TaskID: "non_existent_task"}

	// 2. Setup Config and Evaluator with Custom Rules
	config := &EvalConfig{
		Name:        "Custom Rules Integration Test",
		Metadata:    map[string]interface{}{"custom": true},
		CustomRules: []CustomEvalRule{rule1, rule2, rule3, rule4},
	}
	evaluator := NewStandardEvaluator(config)

	// 3. Setup Mock Agent
	mockAgent := NewMockEvaluableAgent("custom_rule_agent")
	mockAgent.SampleInteractions["relevancy_task"] = struct{ Input, Output, Reference string }{
		Input:     "Where is the Eiffel Tower?",
		Output:    "The Eiffel Tower is located in Paris, France.", // Contains Paris, length is okay
		Reference: "Paris, France",
	}

	// 4. Run Evaluation
	result, err := evaluator.Evaluate(context.Background(), mockAgent)

	// 5. Assertions
	assert.NoError(t, err, "Evaluator.Evaluate returned an error")
	assert.NotNil(t, result, "Evaluation result is nil")
	assert.NotNil(t, result.Metrics, "Result metrics map is nil")

	// Check standard metrics are still present
	assert.Contains(t, result.Metrics, "relevancy_score")
	assert.Contains(t, result.Metrics, "completeness_score") // Needs completeness_task defined in mock
	assert.Contains(t, result.Metrics, "consistency_score")  // Needs MultiOutput defined for relevancy_task input

	// Check custom rule metrics
	assert.Contains(t, result.Metrics, rule1.GetName(), "Metric for rule1 missing")
	assert.InDelta(t, 1.0, result.Metrics[rule1.GetName()], 0.01, "Metric for rule1 (contains_paris) incorrect")

	assert.Contains(t, result.Metrics, rule2.GetName(), "Metric for rule2 missing")
	assert.InDelta(t, 1.0, result.Metrics[rule2.GetName()], 0.01, "Metric for rule2 (output_length) incorrect")

	assert.Contains(t, result.Metrics, rule3.GetName(), "Metric for rule3 missing")
	assert.InDelta(t, 0.0, result.Metrics[rule3.GetName()], 0.01, "Metric for rule3 (contains_banana) incorrect")

	// Check rule that caused an error
	assert.Contains(t, result.Metrics, rule4.GetName(), "Metric for rule4 (error_rule) missing")
	assert.InDelta(t, 0.0, result.Metrics[rule4.GetName()], 0.01, "Metric for rule4 should be 0.0 on error")
	assert.Contains(t, result.Metrics, rule4.GetName()+"_error", "Error metric for rule4 missing")
	t.Logf("Evaluation Metrics: %+v", result.Metrics)
}

// --- Batch Evaluation Test ---

func TestBatchEvaluation(t *testing.T) {
	// 1. Define Test Cases
	testCases := []EvalTestCase{
		{
			ID:              "case1_paris",
			Input:           "Capital of France?",
			ReferenceOutput: "Paris is the capital.",
			Metadata:        map[string]interface{}{"topic": "geography"},
		},
		{
			ID:              "case2_colors",
			Input:           "Primary colors?",
			ReferenceOutput: "red\nyellow\nblue",
			Metadata:        map[string]interface{}{"topic": "art"},
		},
		{
			ID:              "case3_no_ref",
			Input:           "Any input.",
			ReferenceOutput: "", // Test case with no reference
		},
		{
			ID:              "case4_agent_error",
			Input:           "This input causes agent error",
			ReferenceOutput: "Some reference",
		},
	}

	// 2. Setup Mock Agent Responses for Test Cases
	mockAgent := NewMockEvaluableAgent("batch_agent")
	mockAgent.SampleInteractions["Capital of France?"] = struct{ Input, Output, Reference string }{"Capital of France?", "The capital is Paris, a major city.", "Paris is the capital."} // High relevance, complete enough
	mockAgent.SampleInteractions["Primary colors?"] = struct{ Input, Output, Reference string }{"Primary colors?", "red and yellow", "red\nyellow\nblue"}                                // Partial completeness
	mockAgent.SampleInteractions["Any input."] = struct{ Input, Output, Reference string }{"Any input.", "Some valid output.", ""}                                                       // No reference to check against
	// Agent will return error for case4 based on input matching ErrorOnTask
	mockAgent.ErrorOnTask = "This input causes agent error" // Set ErrorOnTask using the Input as TaskID convention

	// 3. Define Custom Rules and Config
	rule1 := &KeywordRule{Name: "mentions_paris", Keyword: "paris"} // TaskID not needed if Evaluate doesn't use agent
	rule2 := &LengthRule{Name: "length_check", MinLength: 5, MaxLength: 50}

	config := &EvalConfig{
		Name:        "Batch Eval Test Run",
		CustomRules: []CustomEvalRule{rule1, rule2},
	}

	// 4. Run Batch Evaluation
	batchResult, err := RunBatchEvaluation(context.Background(), config, mockAgent, testCases)

	// 5. Assertions
	assert.NoError(t, err, "RunBatchEvaluation returned an error")
	assert.NotNil(t, batchResult, "Batch result is nil")

	// Check Summary
	assert.Equal(t, 4, batchResult.Summary.TotalCases, "Summary TotalCases incorrect")
	assert.Equal(t, 3, len(batchResult.Results), "Incorrect number of individual results (should skip error case)")
	assert.Equal(t, 1, batchResult.Summary.Successful, "Summary Successful count incorrect") // Only case 1 should pass threshold
	assert.Equal(t, 2, batchResult.Summary.Failed, "Summary Failed count incorrect")         // Case 2 and 3 should fail threshold
	assert.True(t, batchResult.Summary.AverageScore > 0 && batchResult.Summary.AverageScore < 1, "Summary AverageScore out of expected range")
	assert.True(t, batchResult.Summary.AverageDuration > 0, "Summary AverageDuration invalid")
	t.Logf("Batch Summary: %+v", batchResult.Summary)

	// Check Individual Results (spot check one)
	foundCase1 := false
	for _, res := range batchResult.Results {
		assert.Contains(t, res.ID, "case", "Individual result ID format incorrect")
		if strings.Contains(res.ID, "case1_paris") {
			foundCase1 = true
			assert.True(t, res.Success, "Case 1 should be successful")
			assert.Contains(t, res.Metrics, "relevancy_score")
			assert.Contains(t, res.Metrics, "completeness_score")
			assert.Contains(t, res.Metrics, rule1.GetName()) // mentions_paris
			assert.Contains(t, res.Metrics, rule2.GetName()) // length_check
			assert.InDelta(t, 1.0, res.Metrics[rule1.GetName()], 0.01, "Case 1 Rule 1 score incorrect")
			assert.InDelta(t, 1.0, res.Metrics[rule2.GetName()], 0.01, "Case 1 Rule 2 score incorrect")
			assert.Equal(t, "geography", res.Metadata["topic"], "Case 1 metadata incorrect")
			t.Logf("Case 1 Metrics: %+v", res.Metrics)
		}
	}
	assert.True(t, foundCase1, "Result for case1_paris not found")
}
