package workflow

import (
	"context"
	"fmt"
)

// LoopConditionFunc defines a function that evaluates whether to continue looping
type LoopConditionFunc func(ctx context.Context, input map[string]interface{}, iterations int) (bool, error)

// LoopStep represents a step that executes repeatedly based on a condition
type LoopStep struct {
	ID                string            // Unique identifier for this step
	Name              string            // Human-readable name
	Description       string            // Optional description
	StepToRepeat      *Step             // Step to execute in a loop
	WhileCondition    LoopConditionFunc // Function that returns true when the loop should continue (while semantics)
	UntilCondition    LoopConditionFunc // Function that returns true when the loop should stop (until semantics)
	MaxIterations     int               // Maximum number of iterations to prevent infinite loops
	DependsOn         []string          // IDs of steps this step depends on
	AccumulateResults bool              // Whether to accumulate results from all iterations
}

// LoopWorkflow extends ConditionalWorkflow with looping capabilities
type LoopWorkflow struct {
	*ConditionalWorkflow // Embed ConditionalWorkflow as a pointer
	loopSteps            map[string]*LoopStep
}

// NewLoopWorkflow creates a new workflow with looping support
func NewLoopWorkflow(opts ParallelWorkflowOptions) (*LoopWorkflow, error) {
	base, err := NewConditionalWorkflow(opts)
	if err != nil {
		return nil, err
	}

	return &LoopWorkflow{
		ConditionalWorkflow: base, // Assign the pointer directly
		loopSteps:           make(map[string]*LoopStep),
	}, nil
}

// AddLoopStep adds a looping step to the workflow
func (w *LoopWorkflow) AddLoopStep(step *LoopStep) *LoopWorkflow {
	w.loopSteps[step.ID] = step

	// Set default max iterations if not specified
	if step.MaxIterations <= 0 {
		step.MaxIterations = 10 // Reasonable default to prevent infinite loops
	}

	// Create a regular step that will handle the looping logic
	workflowStep := &Step{
		ID:          step.ID,
		Name:        step.Name,
		Description: step.Description,
		Execute:     w.createLoopExecutor(step),
		Metadata: map[string]interface{}{
			"type":           "loop_step",
			"step_to_repeat": step.StepToRepeat.ID,
			"max_iterations": step.MaxIterations,
			"depends_on":     step.DependsOn,
			"is_loop":        true,
		},
	}

	// Add the loop step to the base workflow
	w.workflowSteps[step.ID] = workflowStep

	// Add the step to repeat to the steps map as well
	w.workflowSteps[step.StepToRepeat.ID] = step.StepToRepeat

	// If this is the first step with no dependencies, set it as the start step
	if len(step.DependsOn) == 0 && w.startStep == "" {
		w.startStep = step.ID
	}

	return w
}

// createLoopExecutor generates an executor function for a loop step
func (w *LoopWorkflow) createLoopExecutor(step *LoopStep) StepExecuteFunc {
	return func(ctx context.Context, input map[string]interface{}, workflow *Workflow) (map[string]interface{}, error) {
		iterations := 0
		results := make([]map[string]interface{}, 0, step.MaxIterations)

		// Current loop context, starts with input and gets updated with each iteration's results
		loopCtx := make(map[string]interface{})
		for k, v := range input {
			loopCtx[k] = v
		}

		// Add iteration counter to the context
		loopCtx["_iteration"] = 0

		// Create a dummy workflow for the step executor
		dummyWorkflow := NewDummyWorkflow()

		for {
			// Check if we've reached the maximum number of iterations
			if iterations >= step.MaxIterations {
				fmt.Printf("Loop step '%s' reached maximum iterations (%d)\n", step.ID, step.MaxIterations)
				break
			}

			// Check while condition (continue while true)
			if step.WhileCondition != nil {
				shouldContinue, err := step.WhileCondition(ctx, loopCtx, iterations)
				if err != nil {
					return nil, fmt.Errorf("error evaluating while condition for step %s: %w", step.ID, err)
				}
				if !shouldContinue {
					fmt.Printf("Loop step '%s' while condition evaluated to false after %d iterations\n", step.ID, iterations)
					break
				}
			}

			// Check until condition (stop when true)
			if step.UntilCondition != nil {
				shouldStop, err := step.UntilCondition(ctx, loopCtx, iterations)
				if err != nil {
					return nil, fmt.Errorf("error evaluating until condition for step %s: %w", step.ID, err)
				}
				if shouldStop {
					fmt.Printf("Loop step '%s' until condition evaluated to true after %d iterations\n", step.ID, iterations)
					break
				}
			}

			// Update iteration counter in context
			loopCtx["_iteration"] = iterations

			// Execute the step
			fmt.Printf("Executing iteration %d of loop step '%s'\n", iterations, step.ID)
			iterResult, err := step.StepToRepeat.Execute(ctx, loopCtx, dummyWorkflow)
			if err != nil {
				return nil, fmt.Errorf("error executing iteration %d of loop step %s: %w", iterations, step.ID, err)
			}

			// Store the result of this iteration
			results = append(results, iterResult)

			// Update the loop context with the latest results
			for k, v := range iterResult {
				loopCtx[k] = v
			}

			iterations++
		}

		// Prepare the final result
		finalResult := map[string]interface{}{
			"_iterations": iterations,
		}

		// If accumulating results, include all iterations' results
		if step.AccumulateResults {
			finalResult["_all_results"] = results
		}

		// Include the last iteration's results in the final output
		if iterations > 0 {
			lastResult := results[iterations-1]
			for k, v := range lastResult {
				if _, exists := finalResult[k]; !exists {
					finalResult[k] = v
				}
			}
		}

		return finalResult, nil
	}
}

// While adds a loop step that continues while the condition is true
func (w *LoopWorkflow) While(
	id string,
	name string,
	condition LoopConditionFunc,
	stepToRepeat *Step,
	maxIterations int,
	dependsOn []string,
) *LoopWorkflow {
	step := &LoopStep{
		ID:                id,
		Name:              name,
		Description:       fmt.Sprintf("While loop for %s", name),
		StepToRepeat:      stepToRepeat,
		WhileCondition:    condition,
		MaxIterations:     maxIterations,
		DependsOn:         dependsOn,
		AccumulateResults: false,
	}

	return w.AddLoopStep(step)
}

// Until adds a loop step that continues until the condition is true
func (w *LoopWorkflow) Until(
	id string,
	name string,
	condition LoopConditionFunc,
	stepToRepeat *Step,
	maxIterations int,
	dependsOn []string,
) *LoopWorkflow {
	step := &LoopStep{
		ID:                id,
		Name:              name,
		Description:       fmt.Sprintf("Until loop for %s", name),
		StepToRepeat:      stepToRepeat,
		UntilCondition:    condition,
		MaxIterations:     maxIterations,
		DependsOn:         dependsOn,
		AccumulateResults: false,
	}

	return w.AddLoopStep(step)
}
