package workflow

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestConditionalWorkflow tests the functionality of conditional branching
func TestConditionalWorkflow(t *testing.T) {
	// --- Test Case 1: True Condition ---
	t.Run("TrueCondition", func(t *testing.T) {
		opts := ParallelWorkflowOptions{
			ID:                    "test-conditional-workflow-true",
			Name:                  "Test Conditional Workflow (True Case)",
			Description:           "A test workflow for conditional execution (true case)",
			MaxParallelExecutions: 2,
		}
		workflow, err := NewConditionalWorkflow(opts)
		assert.NoError(t, err)

		// Start step produces a value
		workflow.AddStep(&ParallelStep{
			ID:        "start",
			Name:      "Start Step",
			OutputKey: "decision_input",
			ExecuteFunc: func(ctx context.Context, args *ParallelStepArgs) (string, error) {
				return "go_true", nil
			},
		})

		// True branch step uses input
		trueBranchStep := &Step{
			ID:   "true-branch",
			Name: "True Branch",
			Execute: func(ctx context.Context, input map[string]interface{}, w *Workflow) (map[string]interface{}, error) {
				startInput, _ := input["decision_input"].(string)
				return map[string]interface{}{
					"path_taken": "true",
					"result":     "True path using " + startInput,
				}, nil
			},
		}

		// False branch step uses input
		falseBranchStep := &Step{
			ID:   "false-branch",
			Name: "False Branch",
			Execute: func(ctx context.Context, input map[string]interface{}, w *Workflow) (map[string]interface{}, error) {
				startInput, _ := input["decision_input"].(string)
				return map[string]interface{}{
					"path_taken": "false",
					"result":     "False path using " + startInput,
				}, nil
			},
		}

		// Condition uses input
		trueCondition := func(ctx context.Context, input map[string]interface{}) (bool, error) {
			val, ok := input["decision_input"].(string)
			assert.True(t, ok, "Condition did not receive decision_input")
			return val == "go_true", nil
		}

		workflow.IfThen(
			"test-condition-true",
			"Test Condition (True)",
			trueCondition,
			trueBranchStep,
			falseBranchStep,
			[]string{"start"},
		)

		ctx := context.Background()
		result, err := workflow.Run(ctx, map[string]interface{}{})
		assert.NoError(t, err)
		assert.Equal(t, "true", result["path_taken"])
		assert.Equal(t, "True path using go_true", result["result"])
	})

	// --- Test Case 2: False Condition ---
	t.Run("FalseCondition", func(t *testing.T) {
		opts := ParallelWorkflowOptions{
			ID:                    "test-conditional-workflow-false",
			Name:                  "Test Conditional Workflow (False Case)",
			Description:           "A test workflow for conditional execution (false case)",
			MaxParallelExecutions: 2,
		}
		workflow, err := NewConditionalWorkflow(opts)
		assert.NoError(t, err)

		// Start step produces a value
		workflow.AddStep(&ParallelStep{
			ID:        "start",
			Name:      "Start Step",
			OutputKey: "decision_input",
			ExecuteFunc: func(ctx context.Context, args *ParallelStepArgs) (string, error) {
				return "go_false", nil // Different value for false case
			},
		})

		// True branch step uses input (same as above)
		trueBranchStep := &Step{
			ID: "true-branch",
			Execute: func(ctx context.Context, input map[string]interface{}, w *Workflow) (map[string]interface{}, error) {
				startInput, _ := input["decision_input"].(string)
				return map[string]interface{}{"path_taken": "true", "result": "True path using " + startInput}, nil
			},
		}

		// False branch step uses input (same as above)
		falseBranchStep := &Step{
			ID: "false-branch",
			Execute: func(ctx context.Context, input map[string]interface{}, w *Workflow) (map[string]interface{}, error) {
				startInput, _ := input["decision_input"].(string)
				return map[string]interface{}{"path_taken": "false", "result": "False path using " + startInput}, nil
			},
		}

		// Condition uses input (same logic, different input leads to false)
		falseCondition := func(ctx context.Context, input map[string]interface{}) (bool, error) {
			val, ok := input["decision_input"].(string)
			assert.True(t, ok, "Condition did not receive decision_input")
			return val == "go_true", nil // This will be false now
		}

		workflow.IfThen(
			"test-condition-false",
			"Test Condition (False)",
			falseCondition,
			trueBranchStep,
			falseBranchStep,
			[]string{"start"},
		)

		ctx := context.Background()
		result, err := workflow.Run(ctx, map[string]interface{}{})
		assert.NoError(t, err)
		assert.Equal(t, "false", result["path_taken"])
		assert.Equal(t, "False path using go_false", result["result"])
	})
}

// TestConditionalWorkflowConditionError tests error handling in the condition function
func TestConditionalWorkflowConditionError(t *testing.T) {
	opts := ParallelWorkflowOptions{
		ID:                    "test-condition-error",
		Name:                  "Test Condition Error Workflow",
		MaxParallelExecutions: 1,
	}
	workflow, err := NewConditionalWorkflow(opts)
	assert.NoError(t, err)

	// Dummy branch steps (won't be executed)
	trueBranchStep := &Step{ID: "true", Execute: func(ctx context.Context, i map[string]interface{}, w *Workflow) (map[string]interface{}, error) {
		return map[string]interface{}{"path": "true"}, nil
	}}
	falseBranchStep := &Step{ID: "false", Execute: func(ctx context.Context, i map[string]interface{}, w *Workflow) (map[string]interface{}, error) {
		return map[string]interface{}{"path": "false"}, nil
	}}

	// Condition that returns an error
	conditionWithError := func(ctx context.Context, input map[string]interface{}) (bool, error) {
		return false, errors.New("condition evaluation failed")
	}

	workflow.IfThen(
		"error-condition",
		"Error Condition Step",
		conditionWithError,
		trueBranchStep,
		falseBranchStep,
		nil, // No dependencies
	)

	// Execute the workflow
	ctx := context.Background()
	result, err := workflow.Run(ctx, map[string]interface{}{})

	// Verify that the workflow run failed with the expected error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error evaluating condition for step error-condition")
	assert.Contains(t, err.Error(), "condition evaluation failed")
	assert.Nil(t, result) // Result should likely be nil on error
}

// TestConditionalWorkflowMissingElse tests behavior when condition is false and no else branch exists
func TestConditionalWorkflowMissingElse(t *testing.T) {
	opts := ParallelWorkflowOptions{
		ID:                    "test-missing-else",
		Name:                  "Test Missing Else Workflow",
		MaxParallelExecutions: 1,
	}
	workflow, err := NewConditionalWorkflow(opts)
	assert.NoError(t, err)

	// Start step
	workflow.AddStep(&ParallelStep{
		ID:        "start",
		OutputKey: "start_val",
		ExecuteFunc: func(ctx context.Context, args *ParallelStepArgs) (string, error) {
			return "started", nil
		},
	})

	// True branch step (should not be executed)
	trueBranchStep := &Step{ID: "true", Execute: func(ctx context.Context, i map[string]interface{}, w *Workflow) (map[string]interface{}, error) {
		t.Log("True branch executed - this should not happen")
		return map[string]interface{}{"path": "true", "result": "TRUE"}, nil
	}}

	// Condition that returns false
	falseCondition := func(ctx context.Context, input map[string]interface{}) (bool, error) {
		return false, nil
	}

	// Add conditional step with nil else branch
	workflow.IfThen(
		"missing-else-condition",
		"Missing Else Condition Step",
		falseCondition,
		trueBranchStep,
		nil, // No else branch provided
		[]string{"start"},
	)

	// Add a final step to ensure workflow continues after the conditional step
	workflow.AddStep(&ParallelStep{
		ID:        "final",
		Name:      "Final Step",
		OutputKey: "final_result",
		DependsOn: []string{"missing-else-condition"},                          // Depends on the conditional step wrapper
		InputKeys: []string{"start_val", "_condition_result", "_branch_taken"}, // Include conditional metadata
		ExecuteFunc: func(ctx context.Context, args *ParallelStepArgs) (string, error) {
			// We expect start_val, _condition_result=false, _branch_taken="none"
			return fmt.Sprintf("Final: %s, Cond: %v, Branch: %s",
					args.StepInputs["start_val"],
					args.StepInputs["_condition_result"],
					args.StepInputs["_branch_taken"]),
				nil
		},
	})

	// Execute the workflow
	ctx := context.Background()
	result, err := workflow.Run(ctx, map[string]interface{}{})

	// Verify successful execution
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Verify true branch was not taken
	assert.Nil(t, result["path"], "Path key from true branch should not exist")
	assert.Nil(t, result["result"], "Result key from true branch should not exist")

	// Verify conditional step metadata indicates no branch was taken
	assert.Equal(t, false, result["_condition_result"])
	assert.Equal(t, "none", result["_branch_taken"])

	// Verify final step executed correctly
	assert.Equal(t, "Final: started, Cond: false, Branch: none", result["final_result"])
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

// TestLoopWorkflowMaxIterations tests that the loop stops at MaxIterations
func TestLoopWorkflowMaxIterations(t *testing.T) {
	opts := ParallelWorkflowOptions{
		ID:                    "test-loop-max-iter",
		Name:                  "Test Loop Max Iterations",
		MaxParallelExecutions: 1,
	}
	workflow, err := NewLoopWorkflow(opts)
	assert.NoError(t, err)

	// Start step (not strictly needed but good practice)
	workflow.AddStep(&ParallelStep{
		ID: "start", OutputKey: "start_val",
		ExecuteFunc: func(ctx context.Context, args *ParallelStepArgs) (string, error) { return "go", nil },
	})

	// Loop body step
	loopBody := &Step{
		ID: "loop-body",
		Execute: func(ctx context.Context, input map[string]interface{}, w *Workflow) (map[string]interface{}, error) {
			iter := 0
			if i, ok := input["_iterations"].(int); ok { // _iterations is 1-based from the loop executor
				iter = i
			}
			return map[string]interface{}{"last_iter": iter}, nil
		},
	}

	// Condition that is always true
	alwaysTrueCondition := func(ctx context.Context, input map[string]interface{}, iterations int) (bool, error) {
		return true, nil // Always try to continue
	}

	maxIterations := 3
	workflow.While(
		"always-true-loop",
		"Always True Loop",
		alwaysTrueCondition,
		loopBody,
		maxIterations, // Set max iterations
		[]string{"start"},
	)

	// Add a final step to confirm execution flow after loop
	workflow.AddStep(&ParallelStep{
		ID:        "final",
		OutputKey: "final_reached",
		DependsOn: []string{"always-true-loop"},
		InputKeys: []string{"last_iter", "_iterations"},
		ExecuteFunc: func(ctx context.Context, args *ParallelStepArgs) (string, error) {
			return fmt.Sprintf("Finished after %v iterations (last body iter: %v)", args.StepInputs["_iterations"], args.StepInputs["last_iter"]), nil
		},
	})

	// Execute
	ctx := context.Background()
	result, err := workflow.Run(ctx, map[string]interface{}{})

	// Verify it stopped due to max iterations
	assert.NoError(t, err) // Should not error, just stop looping gracefully
	assert.NotNil(t, result)
	assert.Equal(t, maxIterations, result["_iterations"], "Should have run exactly maxIterations times")
	// The loop body's result map will reflect the iteration number *it ran in* (1-based)
	assert.Equal(t, maxIterations, result["last_iter"], "Last iteration recorded should match maxIterations")
	assert.Contains(t, result["final_reached"], fmt.Sprintf("Finished after %d iterations", maxIterations))

	// Optional: Check workflow status - should be completed
	// statusInfo, _ := workflow.GetStatusInfo() // Assuming a method like this exists
	// assert.Equal(t, StatusCompleted, statusInfo.Status)
}

// TestLoopWorkflowConditionError tests error handling in the loop condition function
func TestLoopWorkflowConditionError(t *testing.T) {
	opts := ParallelWorkflowOptions{
		ID:                    "test-loop-cond-error",
		Name:                  "Test Loop Condition Error",
		MaxParallelExecutions: 1,
	}
	workflow, err := NewLoopWorkflow(opts)
	assert.NoError(t, err)

	// Start step
	workflow.AddStep(&ParallelStep{
		ID: "start", OutputKey: "counter",
		ExecuteFunc: func(ctx context.Context, args *ParallelStepArgs) (string, error) { return "0", nil },
	})

	// Loop body step
	loopBody := &Step{
		ID: "loop-body",
		Execute: func(ctx context.Context, input map[string]interface{}, w *Workflow) (map[string]interface{}, error) {
			counter := 0
			if val, ok := input["counter"].(int); ok {
				counter = val
			}
			counter++
			return map[string]interface{}{"counter": counter}, nil
		},
	}

	// Condition that returns an error on the 3rd iteration (iterations are 1-based)
	conditionWithError := func(ctx context.Context, input map[string]interface{}, iterations int) (bool, error) {
		if iterations == 3 {
			return false, errors.New("loop condition failed on iteration 3")
		}
		// Continue loop otherwise (up to max iterations)
		return true, nil
	}

	workflow.While(
		"error-condition-loop",
		"Error Condition Loop",
		conditionWithError,
		loopBody,
		5, // Max iterations
		[]string{"start"},
	)

	// Execute
	ctx := context.Background()
	result, err := workflow.Run(ctx, map[string]interface{}{})

	// Verify that the workflow run failed with the expected error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error evaluating while condition for step error-condition-loop")
	assert.Contains(t, err.Error(), "loop condition failed on iteration 3")
	assert.Nil(t, result) // Result should likely be nil on error
}

// TestLoopWorkflowZeroIterations tests loops that should not execute their body
func TestLoopWorkflowZeroIterations(t *testing.T) {
	loopBodyExecuted := false
	loopBody := &Step{
		ID: "loop-body",
		Execute: func(ctx context.Context, input map[string]interface{}, w *Workflow) (map[string]interface{}, error) {
			loopBodyExecuted = true
			t.Error("Loop body should not have been executed")
			return map[string]interface{}{"executed": true}, nil
		},
	}

	// --- Test Case 1: While loop with initially false condition ---
	t.Run("WhileZeroIterations", func(t *testing.T) {
		loopBodyExecuted = false // Reset flag
		opts := ParallelWorkflowOptions{ID: "test-while-zero"}
		workflow, err := NewLoopWorkflow(opts)
		assert.NoError(t, err)

		// Condition that is immediately false
		initiallyFalseCondition := func(ctx context.Context, input map[string]interface{}, iterations int) (bool, error) {
			return false, nil
		}

		workflow.While("while-zero", "While Zero", initiallyFalseCondition, loopBody, 5, nil)

		// Execute
		result, err := workflow.Run(context.Background(), map[string]interface{}{})
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, loopBodyExecuted, "While loop body executed when it shouldn't have")
		assert.Equal(t, 0, result["_iterations"], "While loop should have 0 iterations")
		assert.Nil(t, result["executed"], "Loop body output should not be present")
	})

	// --- Test Case 2: Until loop with initially true condition ---
	t.Run("UntilZeroIterations", func(t *testing.T) {
		loopBodyExecuted = false // Reset flag
		opts := ParallelWorkflowOptions{ID: "test-until-zero"}
		workflow, err := NewLoopWorkflow(opts)
		assert.NoError(t, err)

		// Condition that is immediately true (stop condition)
		initiallyTrueCondition := func(ctx context.Context, input map[string]interface{}, iterations int) (bool, error) {
			return true, nil
		}

		workflow.Until("until-zero", "Until Zero", initiallyTrueCondition, loopBody, 5, nil)

		// Execute
		result, err := workflow.Run(context.Background(), map[string]interface{}{})
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, loopBodyExecuted, "Until loop body executed when it shouldn't have")
		assert.Equal(t, 0, result["_iterations"], "Until loop should have 0 iterations")
		assert.Nil(t, result["executed"], "Loop body output should not be present")
	})
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

// TestNestedWorkflowErrorHandling tests how errors from nested workflows are handled
func TestNestedWorkflowErrorHandling(t *testing.T) {
	// Child workflow that will produce an error
	childOpts := ParallelWorkflowOptions{ID: "error-child"}
	childWorkflow, err := NewNestedWorkflow(childOpts)
	assert.NoError(t, err)

	childError := errors.New("child workflow failed")
	childWorkflow.AddStep(&ParallelStep{
		ID:        "child-error-step",
		OutputKey: "child_error",
		ExecuteFunc: func(ctx context.Context, args *ParallelStepArgs) (string, error) {
			return "", childError // Return an error
		},
	})

	// Parent workflow
	parentOpts := ParallelWorkflowOptions{ID: "error-parent"}
	parentWorkflow, err := NewNestedWorkflow(parentOpts)
	assert.NoError(t, err)

	parentWorkflow.AddStep(&ParallelStep{
		ID:        "parent-start",
		OutputKey: "start_val",
		ExecuteFunc: func(ctx context.Context, args *ParallelStepArgs) (string, error) {
			return "start", nil
		},
	})

	// Add nested workflow step
	parentWorkflow.WithWorkflow(
		"nested-error-step",
		"Nested Error Step",
		childWorkflow,
		nil, // No input mapping needed
		nil, // No output mapping needed
		[]string{"parent-start"},
	)

	// Final parent step (should not be reached)
	parentWorkflow.AddStep(&ParallelStep{
		ID:        "parent-final",
		OutputKey: "final_result",
		DependsOn: []string{"nested-error-step"},
		ExecuteFunc: func(ctx context.Context, args *ParallelStepArgs) (string, error) {
			t.Log("Parent final step executed - this should not happen")
			return "final", nil
		},
	})

	// Execute parent workflow
	ctx := context.Background()
	result, err := parentWorkflow.Run(ctx, map[string]interface{}{})

	// Verify the error from the child workflow is propagated
	assert.Error(t, err)
	// The error might be wrapped, so check for the underlying error
	assert.ErrorIs(t, err, childError, "Expected error from child workflow")
	assert.Contains(t, err.Error(), "child workflow failed")
	assert.Nil(t, result, "Result should be nil when an error occurs")
}

// TestNestedWorkflowWithParallelChild tests running a parent workflow with a child that has parallel steps
func TestNestedWorkflowWithParallelChild(t *testing.T) {
	childDelay1 := 100 * time.Millisecond
	childDelay2 := 150 * time.Millisecond

	// Child workflow with parallel steps
	childOpts := ParallelWorkflowOptions{
		ID:                    "parallel-child",
		Name:                  "Parallel Child Workflow",
		MaxParallelExecutions: 2,
	}
	childWorkflow, err := NewNestedWorkflow(childOpts)
	assert.NoError(t, err)

	// Child step 1 (parallel)
	childWorkflow.AddStep(&ParallelStep{
		ID:         "child-p1",
		OutputKey:  "cp1_result",
		IsParallel: true,
		ExecuteFunc: func(ctx context.Context, args *ParallelStepArgs) (string, error) {
			time.Sleep(childDelay1)
			return "child parallel 1", nil
		},
	})
	// Child step 2 (parallel)
	childWorkflow.AddStep(&ParallelStep{
		ID:         "child-p2",
		OutputKey:  "cp2_result",
		IsParallel: true,
		ExecuteFunc: func(ctx context.Context, args *ParallelStepArgs) (string, error) {
			time.Sleep(childDelay2)
			return "child parallel 2", nil
		},
	})
	// Child step 3 (depends on parallel steps)
	childWorkflow.AddStep(&ParallelStep{
		ID:        "child-final",
		OutputKey: "child_final",
		DependsOn: []string{"child-p1", "child-p2"},
		InputKeys: []string{"cp1_result", "cp2_result"},
		ExecuteFunc: func(ctx context.Context, args *ParallelStepArgs) (string, error) {
			return args.StepInputs["cp1_result"] + " & " + args.StepInputs["cp2_result"], nil
		},
	})

	// Parent workflow
	parentOpts := ParallelWorkflowOptions{ID: "parallel-parent"}
	parentWorkflow, err := NewNestedWorkflow(parentOpts)
	assert.NoError(t, err)

	parentWorkflow.AddStep(&ParallelStep{
		ID:        "parent-start",
		OutputKey: "start_val",
		ExecuteFunc: func(ctx context.Context, args *ParallelStepArgs) (string, error) {
			return "start", nil
		},
	})

	// Add nested workflow step
	outputMapping := map[string]OutputMappingDef{
		"child_final": {SourceKey: "child_final", TargetKey: "nested_result"},
	}
	parentWorkflow.WithWorkflow(
		"nested-parallel-step",
		"Nested Parallel Step",
		childWorkflow,
		nil, // No input mapping
		outputMapping,
		[]string{"parent-start"},
	)

	// Execute parent workflow and measure time
	ctx := context.Background()
	startTime := time.Now()
	result, err := parentWorkflow.Run(ctx, map[string]interface{}{})
	executionTime := time.Since(startTime)

	// Verify no error and results are correct
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "child parallel 1 & child parallel 2", result["nested_result"])

	// Verify execution time reflects child's parallelism
	// Expected time is roughly the longest child branch (childDelay2) plus overhead.
	expectedMinDuration := childDelay2
	expectedMaxDuration := expectedMinDuration + (100 * time.Millisecond) // Generous buffer
	t.Logf("Nested Parallel Execution Time: %v, Expected Min: %v, Expected Max: %v", executionTime, expectedMinDuration, expectedMaxDuration)
	assert.GreaterOrEqual(t, executionTime, expectedMinDuration)
	assert.LessOrEqual(t, executionTime, expectedMaxDuration, "Execution time suggests child did not run in parallel")
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
