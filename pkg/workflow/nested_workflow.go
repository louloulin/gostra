package workflow

import (
	"context"
	"fmt"
)

// WorkflowRefStep represents a step that references another workflow
type WorkflowRefStep struct {
	ID            string                      // Unique identifier for this step
	Name          string                      // Human-readable name
	Description   string                      // Optional description
	Workflow      IWorkflow                   // Referenced workflow
	InputMapping  map[string]string           // Maps parent workflow inputs to child workflow inputs
	OutputMapping map[string]OutputMappingDef // Maps child workflow outputs to parent workflow outputs
	DependsOn     []string                    // IDs of steps this step depends on
}

// OutputMappingDef defines how to map a nested workflow output to parent workflow context
type OutputMappingDef struct {
	SourceKey string // Key in the child workflow's output
	TargetKey string // Key to use in parent workflow's context
}

// NestedWorkflow extends LoopWorkflow to support nested workflows
type NestedWorkflow struct {
	*LoopWorkflow // Embed LoopWorkflow as a pointer
	nestedSteps   map[string]*WorkflowRefStep
}

// NewNestedWorkflow creates a new workflow with nested workflow support
func NewNestedWorkflow(opts ParallelWorkflowOptions) (*NestedWorkflow, error) {
	base, err := NewLoopWorkflow(opts)
	if err != nil {
		return nil, err
	}

	return &NestedWorkflow{
		LoopWorkflow: base, // Assign the pointer directly
		nestedSteps:  make(map[string]*WorkflowRefStep),
	}, nil
}

// AddNestedWorkflow adds a nested workflow step
func (w *NestedWorkflow) AddNestedWorkflow(step *WorkflowRefStep) *NestedWorkflow {
	w.nestedSteps[step.ID] = step

	// Create a regular step that will handle executing the nested workflow
	workflowStep := &Step{
		ID:          step.ID,
		Name:        step.Name,
		Description: step.Description,
		Execute:     w.createNestedWorkflowExecutor(step),
		Metadata: map[string]interface{}{
			"type":            "nested_workflow",
			"nested_workflow": step.Workflow.GetID(),
			"depends_on":      step.DependsOn,
			"is_nested":       true,
		},
	}

	// Add the nested workflow step to the base workflow
	w.workflowSteps[step.ID] = workflowStep

	// If this is the first step with no dependencies, set it as the start step
	if len(step.DependsOn) == 0 && w.startStep == "" {
		w.startStep = step.ID
	}

	return w
}

// createNestedWorkflowExecutor generates an executor function for a nested workflow step
func (w *NestedWorkflow) createNestedWorkflowExecutor(step *WorkflowRefStep) StepExecuteFunc {
	return func(ctx context.Context, input map[string]interface{}, workflow *Workflow) (map[string]interface{}, error) {
		// Prepare input for the nested workflow
		nestedInput := make(map[string]interface{})

		// Apply input mapping
		for parentKey, childKey := range step.InputMapping {
			if value, exists := input[parentKey]; exists {
				nestedInput[childKey] = value
			}
		}

		// Include all inputs if no mapping specified
		if len(step.InputMapping) == 0 {
			for k, v := range input {
				nestedInput[k] = v
			}
		}

		// Execute the nested workflow
		fmt.Printf("Executing nested workflow '%s' in step '%s'\n", step.Workflow.GetID(), step.ID)
		nestedResult, err := step.Workflow.Run(ctx, nestedInput)
		if err != nil {
			return nil, fmt.Errorf("error executing nested workflow '%s' in step '%s': %w",
				step.Workflow.GetID(), step.ID, err)
		}

		// Prepare the result
		result := map[string]interface{}{
			"_nested_workflow_id": step.Workflow.GetID(),
		}

		// Apply output mapping
		for _, mapping := range step.OutputMapping {
			if value, exists := nestedResult[mapping.SourceKey]; exists {
				result[mapping.TargetKey] = value
			}
		}

		// Include all outputs if no mapping specified
		if len(step.OutputMapping) == 0 {
			for k, v := range nestedResult {
				result[k] = v
			}
		}

		// Also include the full nested result for reference
		result["_nested_result"] = nestedResult

		return result, nil
	}
}

// WithWorkflow adds a nested workflow step
func (w *NestedWorkflow) WithWorkflow(
	id string,
	name string,
	nestedWorkflow IWorkflow,
	inputMapping map[string]string,
	outputMapping map[string]OutputMappingDef,
	dependsOn []string,
) *NestedWorkflow {
	step := &WorkflowRefStep{
		ID:            id,
		Name:          name,
		Description:   fmt.Sprintf("Nested workflow %s (%s)", name, nestedWorkflow.GetID()),
		Workflow:      nestedWorkflow,
		InputMapping:  inputMapping,
		OutputMapping: outputMapping,
		DependsOn:     dependsOn,
	}

	return w.AddNestedWorkflow(step)
}
