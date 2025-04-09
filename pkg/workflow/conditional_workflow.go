package workflow

import (
	"context"
	"fmt"
)

// ConditionFunc defines a function that evaluates to determine which branch to take
type ConditionFunc func(ctx context.Context, input map[string]interface{}) (bool, error)

// ConditionalStep represents a step with if/else branching
type ConditionalStep struct {
	ID            string            // Unique identifier for this step
	Name          string            // Human-readable name
	Description   string            // Optional description
	Condition     ConditionFunc     // Function to evaluate the condition
	IfBranch      *Step             // Step to execute if condition is true
	ElseBranch    *Step             // Step to execute if condition is false
	DependsOn     []string          // IDs of steps this step depends on
	ResultMapping map[string]string // Map to store results from either branch
}

// ConditionalWorkflow extends ParallelWorkflow with conditional branching
type ConditionalWorkflow struct {
	ParallelWorkflow // Embed ParallelWorkflow for base functionality
	conditions       map[string]*ConditionalStep
}

// NewConditionalWorkflow creates a new workflow with conditional branching support
func NewConditionalWorkflow(opts ParallelWorkflowOptions) (*ConditionalWorkflow, error) {
	base, err := NewParallelWorkflow(opts)
	if err != nil {
		return nil, err
	}

	return &ConditionalWorkflow{
		ParallelWorkflow: *base,
		conditions:       make(map[string]*ConditionalStep),
	}, nil
}

// AddConditionalStep adds a conditional branching step to the workflow
func (w *ConditionalWorkflow) AddConditionalStep(step *ConditionalStep) *ConditionalWorkflow {
	w.conditions[step.ID] = step

	// Create a regular step that will handle the condition and branching
	workflowStep := &Step{
		ID:          step.ID,
		Name:        step.Name,
		Description: step.Description,
		Execute:     w.createConditionalExecutor(step),
		Metadata: map[string]interface{}{
			"type":         "conditional_step",
			"if_branch":    step.IfBranch.ID,
			"else_branch":  step.ElseBranch.ID,
			"depends_on":   step.DependsOn,
			"is_condition": true,
		},
	}

	// Add the conditional step to the base workflow
	w.workflowSteps[step.ID] = workflowStep

	// Add the actual branches to the steps map as well
	w.workflowSteps[step.IfBranch.ID] = step.IfBranch
	if step.ElseBranch != nil {
		w.workflowSteps[step.ElseBranch.ID] = step.ElseBranch
	}

	// If this is the first step with no dependencies, set it as the start step
	if len(step.DependsOn) == 0 && w.startStep == "" {
		w.startStep = step.ID
	}

	return w
}

// createConditionalExecutor generates an executor function for a conditional step
func (w *ConditionalWorkflow) createConditionalExecutor(step *ConditionalStep) StepExecuteFunc {
	return func(ctx context.Context, input map[string]interface{}, workflow *Workflow) (map[string]interface{}, error) {
		// Evaluate the condition
		conditionResult, err := step.Condition(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("error evaluating condition for step %s: %w", step.ID, err)
		}

		// Select the branch to execute based on the condition
		var selectedBranch *Step
		var branchName string
		if conditionResult {
			selectedBranch = step.IfBranch
			branchName = "if_branch"
		} else if step.ElseBranch != nil {
			selectedBranch = step.ElseBranch
			branchName = "else_branch"
		} else {
			// If condition is false and there's no else branch, return empty result
			return map[string]interface{}{
				"_condition_result": conditionResult,
				"_branch_taken":     "none",
			}, nil
		}

		// Execute the selected branch
		fmt.Printf("Condition for step '%s' evaluated to %v, executing %s\n", step.ID, conditionResult, branchName)

		// Create a dummy workflow for the step executor
		dummyWorkflow := NewDummyWorkflow()

		branchResult, err := selectedBranch.Execute(ctx, input, dummyWorkflow)
		if err != nil {
			return nil, fmt.Errorf("error executing %s for step %s: %w", branchName, step.ID, err)
		}

		// Add metadata about which branch was taken
		result := map[string]interface{}{
			"_condition_result": conditionResult,
			"_branch_taken":     branchName,
		}

		// Map results from the branch using the result mapping
		for resultKey, mappedKey := range step.ResultMapping {
			if value, exists := branchResult[resultKey]; exists {
				result[mappedKey] = value
			}
		}

		// Include all results from the branch
		for k, v := range branchResult {
			if _, exists := result[k]; !exists {
				result[k] = v
			}
		}

		return result, nil
	}
}

// IfThen adds a conditional step with if/else branches
func (w *ConditionalWorkflow) IfThen(
	id string,
	name string,
	condition ConditionFunc,
	ifStep *Step,
	elseStep *Step,
	dependsOn []string,
) *ConditionalWorkflow {
	step := &ConditionalStep{
		ID:            id,
		Name:          name,
		Description:   fmt.Sprintf("Conditional branch for %s", name),
		Condition:     condition,
		IfBranch:      ifStep,
		ElseBranch:    elseStep,
		DependsOn:     dependsOn,
		ResultMapping: make(map[string]string),
	}

	return w.AddConditionalStep(step)
}
