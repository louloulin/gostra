package evals

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// StandardEvaluator implements the Evaluator interface with common evaluation metrics
type StandardEvaluator struct {
	*BaseEvaluator
}

// NewStandardEvaluator creates a new StandardEvaluator instance
func NewStandardEvaluator(config *EvalConfig) *StandardEvaluator {
	return &StandardEvaluator{
		BaseEvaluator: NewBaseEvaluator(config),
	}
}

// Evaluate performs the evaluation of an agent on standard internal tasks
func (e *StandardEvaluator) Evaluate(ctx context.Context, agent AgentEvaluable) (*EvalResult, error) {
	startTime := time.Now()

	result := &EvalResult{
		ID:          agent.GetID(),
		Name:        e.config.Name,
		Description: e.config.Description,
		StartTime:   startTime,
		Metadata:    e.config.Metadata,
		Metrics:     make(map[string]float64),
	}

	// --- Fetch data for standard evaluations ---
	// Relevancy data
	relInput, relOutput, relReference, relErr := agent.GetSampleInteraction(ctx, "relevancy_task")
	// Completeness data
	_, compOutput, compReference, compErr := agent.GetSampleInteraction(ctx, "completeness_task")
	// Consistency data (using relevancy input)
	var consistencyOutputs []string
	var consErr error
	if relErr == nil && relInput != "" {
		consistencyOutputs, consErr = agent.GetMultipleOutputs(ctx, relInput, 3)
	} else {
		// If we can't get input for consistency check, set an error or default
		consErr = fmt.Errorf("cannot perform consistency check: failed to get input from relevancy_task: %w", relErr)
	}

	// --- Calculate standard metrics ---
	// Note: We calculate even if fetching data failed, metric funcs should handle errors/empty data
	relevancyScore := e.evaluateRelevancyMetric(relInput, relOutput, relReference) // Use fetched data
	e.metrics.CollectMetric("relevancy_score", relevancyScore)

	completenessScore := e.evaluateCompletenessMetric(compOutput, compReference) // Use fetched data
	e.metrics.CollectMetric("completeness_score", completenessScore)

	consistencyScore := e.evaluateConsistencyMetric(consistencyOutputs) // Use fetched data
	e.metrics.CollectMetric("consistency_score", consistencyScore)

	// --- Execute custom rules ---
	// Custom rules might need their own way to get data if they don't use standard tasks
	// Current CustomEvalRule interface assumes access to AgentEvaluable.
	// TODO: Revisit custom rule data fetching for batch evaluation.
	if e.config.CustomRules != nil && len(e.config.CustomRules) > 0 {
		for _, rule := range e.config.CustomRules {
			ruleName := rule.GetName()
			// Pass data fetched for standard metrics to custom rules for now.
			// A more robust system might allow rules to specify which data they need.
			score, err := rule.Evaluate(ctx, relInput, relOutput, relReference)
			if err != nil {
				log.Printf("Error evaluating custom rule '%s': %v", ruleName, err)
				result.Metrics[ruleName] = 0.0
			} else {
				result.Metrics[ruleName] = score
			}
		}
	}

	// --- Finalize Result ---
	// Collect basic metrics (response time covers fetching + calculation)
	e.metrics.CollectMetric("response_time_ms", float64(time.Since(startTime).Milliseconds()))

	// Calculate final score (weighted average of standard metrics)
	finalScore := (relevancyScore*0.4 + completenessScore*0.3 + consistencyScore*0.3)

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(startTime)
	result.Score = finalScore

	// Add all collected metrics (standard + custom + response_time)
	for key, val := range e.GetMetrics() {
		if _, exists := result.Metrics[key]; !exists {
			result.Metrics[key] = val
		}
	}
	result.Success = finalScore >= 0.7 // Consider successful if score is at least 0.7

	// Optionally add errors from data fetching to the result?
	if relErr != nil {
		result.Error = fmt.Sprintf("Relevancy task error: %v; ", relErr)
	}
	if compErr != nil {
		result.Error += fmt.Sprintf("Completeness task error: %v; ", compErr)
	}
	if consErr != nil {
		result.Error += fmt.Sprintf("Consistency task error: %v; ", consErr)
	}

	return result, nil // Overall Evaluate doesn't fail on metric errors, returns results found
}

// evaluateRelevancyMetric calculates score based on provided strings
func (e *StandardEvaluator) evaluateRelevancyMetric(input, output, referenceOutput string) float64 {
	if referenceOutput == "" {
		// Cannot evaluate without reference
		return 0.0
	}
	_ = input // Input might be used by more advanced metrics

	// Simple Jaccard index based on word overlap
	outputWords := wordSet(output)
	referenceWords := wordSet(referenceOutput)

	intersection := 0
	for word := range outputWords {
		if referenceWords[word] {
			intersection++
		}
	}

	union := len(outputWords) + len(referenceWords) - intersection
	if union == 0 {
		return 1.0 // Both empty or identical single word
	}

	return float64(intersection) / float64(union)
}

// evaluateCompletenessMetric calculates score based on provided strings
func (e *StandardEvaluator) evaluateCompletenessMetric(output, referenceOutput string) float64 {
	if referenceOutput == "" {
		return 1.0 // No requirements specified, trivially complete
	}

	// Assume referenceOutput contains required items separated by newline
	requiredItems := strings.Split(referenceOutput, "\n")
	foundCount := 0
	totalRequired := 0

	for _, item := range requiredItems {
		trimmedItem := strings.TrimSpace(item)
		if trimmedItem != "" {
			totalRequired++
			if strings.Contains(output, trimmedItem) {
				foundCount++
			}
		}
	}

	if totalRequired == 0 {
		return 1.0 // Reference contained only whitespace/empty lines
	}

	return float64(foundCount) / float64(totalRequired)
}

// evaluateConsistencyMetric calculates score based on provided outputs
func (e *StandardEvaluator) evaluateConsistencyMetric(outputs []string) float64 {
	if len(outputs) < 2 { // Need at least 2 outputs to compare
		// If only 0 or 1 output provided, consider it consistent?
		// Or return 0 as consistency couldn't be measured?
		// Let's return 1.0 assuming single output is trivially consistent.
		return 1.0
	}

	var totalSimilarity float64
	comparisonCount := 0

	for i := 0; i < len(outputs); i++ {
		for j := i + 1; j < len(outputs); j++ {
			set1 := wordSet(outputs[i])
			set2 := wordSet(outputs[j])

			intersection := 0
			for word := range set1 {
				if set2[word] {
					intersection++
				}
			}

			union := len(set1) + len(set2) - intersection
			similarity := 0.0
			if union > 0 {
				similarity = float64(intersection) / float64(union)
			} else if len(set1) == 0 && len(set2) == 0 { // Both empty
				similarity = 1.0
			}

			totalSimilarity += similarity
			comparisonCount++
		}
	}

	if comparisonCount == 0 {
		return 1.0 // Only one output, considered consistent
	}

	return totalSimilarity / float64(comparisonCount)
}

// Helper function to create a set of words from a string
func wordSet(s string) map[string]bool {
	set := make(map[string]bool)
	words := strings.Fields(strings.ToLower(s)) // Simple split by space, lowercased
	for _, word := range words {
		// Basic cleanup - remove common punctuation if needed, depending on desired granularity
		cleanedWord := strings.Trim(word, ".?!,;:\"")
		if cleanedWord != "" {
			set[cleanedWord] = true
		}
	}
	return set
}

// --- Batch Evaluation Function ---

// RunBatchEvaluation evaluates an agent against a batch of test cases.
func RunBatchEvaluation(ctx context.Context, config *EvalConfig, agent AgentEvaluable, testCases []EvalTestCase) (*BatchEvalResult, error) {
	batchStartTime := time.Now()
	individualResults := make([]*EvalResult, 0, len(testCases))
	var totalScore float64
	var totalDuration time.Duration
	successfulCount := 0

	// Create an evaluator instance to use its metric functions
	// TODO: Consider if evaluator should be passed in, or if metric funcs should be static/exported?
	// Using a temporary evaluator instance for now.
	evaluator := NewStandardEvaluator(config)

	for i, tc := range testCases {
		evaluator.Reset() // Reset metrics for each test case
		caseStartTime := time.Now()

		// Get agent's output for this test case input
		// TODO: Handle agent errors more gracefully (e.g., mark case as failed?)
		// Use GetSampleInteraction convention for now, assuming it maps ID/Input
		_, actualOutput, _, agentErr := agent.GetSampleInteraction(ctx, tc.Input) // Using Input as TaskID convention here
		if agentErr != nil {
			// Log or handle error - skip this case or mark as failed?
			log.Printf("Agent error on test case %d ('%s'): %v", i, tc.ID, agentErr)
			// Create a minimal failed result? For now, skip aggregation.
			continue
		}

		caseMetrics := make(map[string]float64)

		// Calculate standard metrics for this case
		relevancyScore := evaluator.evaluateRelevancyMetric(tc.Input, actualOutput, tc.ReferenceOutput)
		caseMetrics["relevancy_score"] = relevancyScore

		completenessScore := evaluator.evaluateCompletenessMetric(actualOutput, tc.ReferenceOutput)
		caseMetrics["completeness_score"] = completenessScore

		// Consistency requires multiple outputs - how to fit into batch?
		// Option 1: Run agent multiple times per test case (slow).
		// Option 2: Have a separate consistency check outside the main batch loop.
		// Option 3: Assume consistency is evaluated separately or not part of per-case eval.
		// Let's skip consistency for per-case batch results for now.
		// caseMetrics["consistency_score"] = evaluator.evaluateConsistencyMetric(...)

		// Execute custom rules for this case
		if config.CustomRules != nil {
			for _, rule := range config.CustomRules {
				ruleName := rule.GetName()
				score, err := rule.Evaluate(ctx, tc.Input, actualOutput, tc.ReferenceOutput)
				if err != nil {
					log.Printf("Error evaluating custom rule '%s' on case %d: %v", ruleName, i, err)
					caseMetrics[ruleName] = 0.0
				} else {
					caseMetrics[ruleName] = score
				}
			}
		}

		// Calculate final score for this case (adjust weights if consistency is excluded)
		caseScore := (relevancyScore*0.6 + completenessScore*0.4) // Example adjusted weights
		caseEndTime := time.Now()
		caseDuration := caseEndTime.Sub(caseStartTime)
		caseMetrics["response_time_ms"] = float64(caseDuration.Milliseconds()) // Per-case duration

		caseResult := &EvalResult{
			ID:          fmt.Sprintf("%s_%s", agent.GetID(), tc.ID), // Combine agent and case ID
			Name:        fmt.Sprintf("%s - Case: %s", config.Name, tc.ID),
			Description: tc.Input, // Use input as description?
			StartTime:   caseStartTime,
			EndTime:     caseEndTime,
			Duration:    caseDuration,
			Success:     caseScore >= 0.7, // Case success threshold
			Score:       caseScore,
			Metrics:     caseMetrics,
			Metadata:    tc.Metadata, // Include case metadata
			// Error: Should we store agentErr here?
		}
		individualResults = append(individualResults, caseResult)

		// Aggregate for summary
		totalScore += caseScore
		totalDuration += caseDuration
		if caseResult.Success {
			successfulCount++
		}
	}

	batchEndTime := time.Now()
	totalCasesRun := len(individualResults)
	batchSummary := BatchEvalSummary{
		TotalCases: len(testCases),
		Successful: successfulCount,
		Failed:     totalCasesRun - successfulCount, // Cases that ran but didn't meet success threshold
		// Note: Cases skipped due to agent error are not included in success/fail counts here
	}
	if totalCasesRun > 0 {
		batchSummary.AverageScore = totalScore / float64(totalCasesRun)
		batchSummary.AverageDuration = totalDuration / time.Duration(totalCasesRun)
	}

	batchResult := &BatchEvalResult{
		AgentID:       agent.GetID(),
		EvalConfig:    config,
		StartTime:     batchStartTime,
		EndTime:       batchEndTime,
		TotalDuration: batchEndTime.Sub(batchStartTime),
		Results:       individualResults,
		Summary:       batchSummary,
	}

	return batchResult, nil
}
