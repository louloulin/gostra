package workflow

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestConditionalWorkflow tests the functionality of conditional branching
func TestConditionalWorkflow(t *testing.T) {
	// Create a workflow with conditional branches
	opts := ParallelWorkflowOptions{
		ID:                    "test-conditional-workflow",
		Name:                  "Test Conditional Workflow",
		Description:           "A test workflow for conditional execution",
		MaxParallelExecutions: 2,
	}

	workflow, err := NewConditionalWorkflow(opts)
	assert.NoError(t, err)

	// Add a starting step
	workflow.AddStep(&ParallelStep{
		ID:          "start",
		Name:        "Start Step",
		Description: "Initial step that sets a test value",
		OutputKey:   "test_value",
		ExecuteFunc: func(ctx context.Context, args *ParallelStepArgs) (string, error) {
			return "test", nil
		},
	})

	// Create true and false path steps
	trueBranchStep := &Step{
		ID:          "true-branch",
		Name:        "True Branch",
		Description: "Executed when condition is true",
		Execute: func(ctx context.Context, input map[string]interface{}, w *Workflow) (map[string]interface{}, error) {
			return map[string]interface{}{
				"path_taken": "true",
				"result":     "True path result",
			}, nil
		},
	}

	falseBranchStep := &Step{
		ID:          "false-branch",
		Name:        "False Branch",
		Description: "Executed when condition is false",
		Execute: func(ctx context.Context, input map[string]interface{}, w *Workflow) (map[string]interface{}, error) {
			return map[string]interface{}{
				"path_taken": "false",
				"result":     "False path result",
			}, nil
		},
	}

	// Test case 1: True condition
	trueCondition := func(ctx context.Context, input map[string]interface{}) (bool, error) {
		return true, nil
	}

	// Add the conditional step for true case
	workflow.IfThen(
		"test-condition-true",
		"Test Condition (True)",
		trueCondition,
		trueBranchStep,
		falseBranchStep,
		[]string{"start"},
	)

	// Execute the workflow
	ctx := context.Background()
	result, err := workflow.Run(ctx, map[string]interface{}{})
	assert.NoError(t, err)

	// Verify the true path was taken
	assert.Equal(t, "true", result["path_taken"])
	assert.Equal(t, "True path result", result["result"])

	// Test case 2: False condition
	opts = ParallelWorkflowOptions{
		ID:                    "test-conditional-workflow-false",
		Name:                  "Test Conditional Workflow (False Case)",
		Description:           "A test workflow for conditional execution (false case)",
		MaxParallelExecutions: 2,
	}

	workflow, err = NewConditionalWorkflow(opts)
	assert.NoError(t, err)

	// Add a starting step
	workflow.AddStep(&ParallelStep{
		ID:          "start",
		Name:        "Start Step",
		Description: "Initial step that sets a test value",
		OutputKey:   "test_value",
		ExecuteFunc: func(ctx context.Context, args *ParallelStepArgs) (string, error) {
			return "test", nil
		},
	})

	// False condition
	falseCondition := func(ctx context.Context, input map[string]interface{}) (bool, error) {
		return false, nil
	}

	// Add the conditional step for false case
	workflow.IfThen(
		"test-condition-false",
		"Test Condition (False)",
		falseCondition,
		trueBranchStep,
		falseBranchStep,
		[]string{"start"},
	)

	// Execute the workflow
	result, err = workflow.Run(ctx, map[string]interface{}{})
	assert.NoError(t, err)

	// Verify the false path was taken
	assert.Equal(t, "false", result["path_taken"])
	assert.Equal(t, "False path result", result["result"])
}

// TestLoopWorkflow tests the functionality of looping (while/until)
func TestLoopWorkflow(t *testing.T) {
	// Create a workflow with a loop
	opts := ParallelWorkflowOptions{
		ID:                    "test-loop-workflow",
		Name:                  "Test Loop Workflow",
		Description:           "A test workflow for loop execution",
		MaxParallelExecutions: 2,
	}

	workflow, err := NewLoopWorkflow(opts)
	assert.NoError(t, err)

	// Add a starting step
	workflow.AddStep(&ParallelStep{
		ID:          "start",
		Name:        "Start Step",
		Description: "Initial step that sets counter to 0",
		OutputKey:   "counter",
		ExecuteFunc: func(ctx context.Context, args *ParallelStepArgs) (string, error) {
			return "0", nil
		},
	})

	// Create a step that will be executed in a loop
	incrementStep := &Step{
		ID:          "increment",
		Name:        "Increment Counter",
		Description: "Increments the counter by 1",
		Execute: func(ctx context.Context, input map[string]interface{}, w *Workflow) (map[string]interface{}, error) {
			// Get current counter value
			var counter int
			if counterStr, ok := input["counter"].(string); ok {
				fmt.Sscanf(counterStr, "%d", &counter)
			} else if counterVal, ok := input["counter"].(int); ok {
				counter = counterVal
			}

			// Increment counter
			counter++

			return map[string]interface{}{
				"counter":       counter,
				"current_value": fmt.Sprintf("Counter value: %d", counter),
			}, nil
		},
	}

	// Test While loop - continue while counter < 5
	whileCondition := func(ctx context.Context, input map[string]interface{}, iterations int) (bool, error) {
		var counter int
		if counterVal, ok := input["counter"].(int); ok {
			counter = counterVal
		}
		return counter < 5, nil
	}

	workflow.While(
		"test-while-loop",
		"Test While Loop",
		whileCondition,
		incrementStep,
		10, // Max iterations as a safeguard
		[]string{"start"},
	)

	// Execute the workflow
	ctx := context.Background()
	result, err := workflow.Run(ctx, map[string]interface{}{})
	assert.NoError(t, err)

	// Verify the loop executed the expected number of times
	assert.Equal(t, 5, result["counter"])
	assert.Equal(t, 5, result["_iterations"])

	// Test Until loop
	opts = ParallelWorkflowOptions{
		ID:                    "test-until-workflow",
		Name:                  "Test Until Workflow",
		Description:           "A test workflow for until loop execution",
		MaxParallelExecutions: 2,
	}

	workflow, err = NewLoopWorkflow(opts)
	assert.NoError(t, err)

	// Add a starting step
	workflow.AddStep(&ParallelStep{
		ID:          "start",
		Name:        "Start Step",
		Description: "Initial step that sets counter to 0",
		OutputKey:   "counter",
		ExecuteFunc: func(ctx context.Context, args *ParallelStepArgs) (string, error) {
			return "0", nil
		},
	})

	// Test Until loop - continue until counter >= 3
	untilCondition := func(ctx context.Context, input map[string]interface{}, iterations int) (bool, error) {
		var counter int
		if counterVal, ok := input["counter"].(int); ok {
			counter = counterVal
		}
		return counter >= 3, nil
	}

	workflow.Until(
		"test-until-loop",
		"Test Until Loop",
		untilCondition,
		incrementStep,
		10, // Max iterations as a safeguard
		[]string{"start"},
	)

	// Execute the workflow
	result, err = workflow.Run(ctx, map[string]interface{}{})
	assert.NoError(t, err)

	// Verify the loop executed the expected number of times
	assert.Equal(t, 3, result["counter"])
	assert.Equal(t, 3, result["_iterations"])
}

// TestNestedWorkflow tests the functionality of nested workflows
func TestNestedWorkflow(t *testing.T) {
	// First, create a child workflow
	childOpts := ParallelWorkflowOptions{
		ID:                    "test-child-workflow",
		Name:                  "Test Child Workflow",
		Description:           "A test child workflow",
		MaxParallelExecutions: 2,
	}

	childWorkflow, err := NewNestedWorkflow(childOpts)
	assert.NoError(t, err)

	// Add steps to the child workflow
	childWorkflow.AddStep(&ParallelStep{
		ID:          "child-step-1",
		Name:        "Child Step 1",
		Description: "First step in child workflow",
		OutputKey:   "child_result_1",
		InputKeys:   []string{"input_value"},
		ExecuteFunc: func(ctx context.Context, args *ParallelStepArgs) (string, error) {
			inputVal, _ := args.Context["input_value"].(string)
			return fmt.Sprintf("Processed %s in child step 1", inputVal), nil
		},
	})

	childWorkflow.AddStep(&ParallelStep{
		ID:          "child-step-2",
		Name:        "Child Step 2",
		Description: "Second step in child workflow",
		OutputKey:   "child_result_2",
		InputKeys:   []string{"child_result_1"},
		DependsOn:   []string{"child-step-1"},
		ExecuteFunc: func(ctx context.Context, args *ParallelStepArgs) (string, error) {
			prevResult := args.StepInputs["child_result_1"]
			return fmt.Sprintf("Final result: %s", prevResult), nil
		},
	})

	// Now create a parent workflow that will use the child workflow
	parentOpts := ParallelWorkflowOptions{
		ID:                    "test-parent-workflow",
		Name:                  "Test Parent Workflow",
		Description:           "A test parent workflow that uses a child workflow",
		MaxParallelExecutions: 2,
	}

	parentWorkflow, err := NewNestedWorkflow(parentOpts)
	assert.NoError(t, err)

	// Add a starting step to the parent workflow
	parentWorkflow.AddStep(&ParallelStep{
		ID:          "parent-start",
		Name:        "Parent Start",
		Description: "Initial step in parent workflow",
		OutputKey:   "start_value",
		ExecuteFunc: func(ctx context.Context, args *ParallelStepArgs) (string, error) {
			return "test input", nil
		},
	})

	// Define input/output mappings for the nested workflow
	inputMapping := map[string]string{
		"start_value": "input_value", // Map parent's start_value to child's input_value
	}

	outputMapping := map[string]OutputMappingDef{
		"child_result_2": {
			SourceKey: "child_result_2",
			TargetKey: "nested_output",
		},
	}

	// Add the nested workflow as a step
	parentWorkflow.WithWorkflow(
		"nested-workflow-step",
		"Nested Workflow Step",
		childWorkflow,
		inputMapping,
		outputMapping,
		[]string{"parent-start"},
	)

	// Add a final step that uses the nested workflow's output
	parentWorkflow.AddStep(&ParallelStep{
		ID:          "parent-final",
		Name:        "Parent Final Step",
		Description: "Final step that uses nested workflow output",
		OutputKey:   "parent_final_result",
		InputKeys:   []string{"nested_output"},
		DependsOn:   []string{"nested-workflow-step"},
		ExecuteFunc: func(ctx context.Context, args *ParallelStepArgs) (string, error) {
			nestedOutput := args.StepInputs["nested_output"]
			return fmt.Sprintf("Parent workflow completed with nested result: %s", nestedOutput), nil
		},
	})

	// Execute the parent workflow
	ctx := context.Background()
	result, err := parentWorkflow.Run(ctx, map[string]interface{}{})
	assert.NoError(t, err)

	// Verify the nested workflow was executed correctly and results were properly mapped
	assert.Contains(t, result["nested_output"], "Final result: Processed test input")
	assert.Contains(t, result["parent_final_result"], "Parent workflow completed with nested result")
}

// TestComplexWorkflow tests a combination of conditional, loop, and nested workflow features
func TestComplexWorkflow(t *testing.T) {
	// Create a complex workflow that combines all features
	opts := ParallelWorkflowOptions{
		ID:                    "test-complex-workflow",
		Name:                  "Test Complex Workflow",
		Description:           "A test workflow combining conditional, loop, and nested features",
		MaxParallelExecutions: 3,
	}

	workflow, err := NewNestedWorkflow(opts)
	assert.NoError(t, err)

	// Add a starting step
	workflow.AddStep(&ParallelStep{
		ID:          "complex-start",
		Name:        "Complex Start",
		Description: "Initial step for complex workflow",
		OutputKey:   "test_input",
		ExecuteFunc: func(ctx context.Context, args *ParallelStepArgs) (string, error) {
			return "complex test", nil
		},
	})

	// Create a simple child workflow
	childOpts := ParallelWorkflowOptions{
		ID:                    "complex-child-workflow",
		Name:                  "Complex Child Workflow",
		Description:           "Child workflow for complex test",
		MaxParallelExecutions: 2,
	}

	childWorkflow, err := NewNestedWorkflow(childOpts)
	assert.NoError(t, err)

	childWorkflow.AddStep(&ParallelStep{
		ID:          "child-process",
		Name:        "Child Process",
		Description: "Process data in child workflow",
		OutputKey:   "child_output",
		InputKeys:   []string{"child_input"},
		ExecuteFunc: func(ctx context.Context, args *ParallelStepArgs) (string, error) {
			inputVal, _ := args.Context["child_input"].(string)
			return fmt.Sprintf("Child processed: %s", inputVal), nil
		},
	})

	// Create branch steps
	pathAStep := &Step{
		ID:          "path-a",
		Name:        "Path A",
		Description: "Executed for path A",
		Execute: func(ctx context.Context, input map[string]interface{}, w *Workflow) (map[string]interface{}, error) {
			testInput := input["test_input"].(string)
			return map[string]interface{}{
				"path_result": fmt.Sprintf("Path A result with %s", testInput),
				"path":        "A",
			}, nil
		},
	}

	pathBStep := &Step{
		ID:          "path-b",
		Name:        "Path B",
		Description: "Executed for path B",
		Execute: func(ctx context.Context, input map[string]interface{}, w *Workflow) (map[string]interface{}, error) {
			testInput := input["test_input"].(string)
			return map[string]interface{}{
				"path_result": fmt.Sprintf("Path B result with %s", testInput),
				"path":        "B",
			}, nil
		},
	}

	// Add conditional branching - choose Path A for this test
	pathCondition := func(ctx context.Context, input map[string]interface{}) (bool, error) {
		return true, nil // Always choose path A for this test
	}

	workflow.IfThen(
		"path-condition",
		"Path Condition",
		pathCondition,
		pathAStep,
		pathBStep,
		[]string{"complex-start"},
	)

	// Create a step for looping
	counterStep := &Step{
		ID:          "counter-step",
		Name:        "Counter Step",
		Description: "Increments a counter",
		Execute: func(ctx context.Context, input map[string]interface{}, w *Workflow) (map[string]interface{}, error) {
			// Initialize or increment counter
			counter := 0
			if val, exists := input["counter"]; exists {
				if counterVal, ok := val.(int); ok {
					counter = counterVal
				}
			}
			counter++

			// Get path information
			path := input["path"].(string)

			return map[string]interface{}{
				"counter":     counter,
				"loop_result": fmt.Sprintf("Counter at %d for path %s", counter, path),
				"path":        path, // Preserve path information
			}, nil
		},
	}

	// Add a loop that executes 3 times
	untilCondition := func(ctx context.Context, input map[string]interface{}, iterations int) (bool, error) {
		var counter int
		if counterVal, ok := input["counter"].(int); ok {
			counter = counterVal
		}
		return counter >= 3, nil
	}

	workflow.Until(
		"loop-step",
		"Loop Step",
		untilCondition,
		counterStep,
		5, // Max iterations
		[]string{"path-condition"},
	)

	// Add the nested workflow with proper mappings
	inputMapping := map[string]string{
		"path_result": "child_input", // Map the path result to child input
	}

	outputMapping := map[string]OutputMappingDef{
		"child_output": {
			SourceKey: "child_output",
			TargetKey: "nested_result",
		},
	}

	workflow.WithWorkflow(
		"nested-step",
		"Nested Step",
		childWorkflow,
		inputMapping,
		outputMapping,
		[]string{"loop-step"},
	)

	// Add a final step
	workflow.AddStep(&ParallelStep{
		ID:          "final-step",
		Name:        "Final Step",
		Description: "Final step that combines all results",
		OutputKey:   "final_result",
		InputKeys:   []string{"nested_result", "loop_result", "path"},
		DependsOn:   []string{"nested-step"},
		ExecuteFunc: func(ctx context.Context, args *ParallelStepArgs) (string, error) {
			nestedResult := args.StepInputs["nested_result"]
			loopResult := args.StepInputs["loop_result"]
			path := args.StepInputs["path"]

			return fmt.Sprintf("Final complex result: Path %s, Loop %s, Nested %s",
				path, loopResult, nestedResult), nil
		},
	})

	// Execute the workflow with timing
	startTime := time.Now()
	ctx := context.Background()
	result, err := workflow.Run(ctx, map[string]interface{}{})
	duration := time.Since(startTime)

	// Log the execution time
	t.Logf("Complex workflow execution time: %v", duration)

	assert.NoError(t, err)

	// Verify results contain all expected components
	assert.Contains(t, result["nested_result"], "Child processed")
	assert.Contains(t, result["loop_result"], "Counter at 3")
	assert.Equal(t, "A", result["path"])
	assert.Contains(t, result["final_result"], "Final complex result")

	// Additional verification
	assert.Equal(t, 3, result["counter"])
	assert.Equal(t, 3, result["_iterations"])
}
