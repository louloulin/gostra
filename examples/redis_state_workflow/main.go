package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"gostra/pkg/workflow"

	"github.com/google/uuid"
)

func main() {
	// Get Redis connection address from environment variable
	// Format: "localhost:6379"
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		fmt.Println("REDIS_ADDR environment variable is not set.")
		fmt.Println("Please set it to a valid Redis server address.")
		fmt.Println("Example: localhost:6379")
		os.Exit(1)
	}

	// Get Redis password if provided
	redisPassword := os.Getenv("REDIS_PASSWORD")

	// Create Redis state store with context
	ctx := context.Background()
	stateStore, err := workflow.NewRedisStateStore(ctx, workflow.RedisStateStoreOptions{
		Address:    redisAddr,
		Password:   redisPassword,
		Database:   0,                  // Default database
		KeyPrefix:  "gostra:workflow:", // Namespace keys to avoid collisions
		Expiration: 24 * time.Hour,     // Optional: expire workflow states after 24 hours
	})
	if err != nil {
		fmt.Printf("Failed to create Redis state store: %v\n", err)
		os.Exit(1)
	}
	defer stateStore.Close()

	// Run the example
	fmt.Println("Running Redis workflow state store example...")
	err = runRedisWorkflowExample(stateStore)
	if err != nil {
		fmt.Printf("Example failed: %v\n", err)
		os.Exit(1)
	}
}

func runRedisWorkflowExample(stateStore workflow.WorkflowStateStore) error {
	// Create a simple event workflow
	eventWorkflow, err := workflow.NewEventWorkflow(workflow.EventWorkflowOptions{
		ID:          "redis-example-" + uuid.New().String()[:8],
		Name:        "Redis Example Workflow",
		Description: "A simple example demonstrating Redis state persistence",
		StartStepID: "step1",
		StateStore:  stateStore, // Use our Redis state store
	})
	if err != nil {
		return fmt.Errorf("failed to create workflow: %w", err)
	}

	// Add steps to the workflow
	// Step 1: Initial step
	err = eventWorkflow.AddStep(&workflow.EventStep{
		ID:          "step1",
		Name:        "Initial Step",
		Description: "The first step in our workflow",
		Handler:     handleStep1,
		NextSteps: map[string]string{
			"continue": "step2", // On "continue" event, go to step2
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add step1: %w", err)
	}

	// Step 2: Middle step that will suspend the workflow
	err = eventWorkflow.AddStep(&workflow.EventStep{
		ID:          "step2",
		Name:        "Process Step",
		Description: "A step that processes data and suspends",
		Handler:     handleStep2,
		NextSteps: map[string]string{
			"completed": "step3", // On "completed" event, go to step3
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add step2: %w", err)
	}

	// Step 3: Final step
	err = eventWorkflow.AddStep(&workflow.EventStep{
		ID:          "step3",
		Name:        "Final Step",
		Description: "Completes the workflow",
		Handler:     handleStep3,
		NextSteps:   map[string]string{}, // No next steps, this is the end
	})
	if err != nil {
		return fmt.Errorf("failed to add step3: %w", err)
	}

	// Start the workflow with initial data
	instanceID, err := eventWorkflow.StartWorkflow(context.Background(), map[string]interface{}{
		"message": "Hello from Redis example!",
	})
	if err != nil {
		return fmt.Errorf("failed to start workflow: %w", err)
	}

	fmt.Printf("Started workflow instance: %s\n", instanceID)

	// Wait a moment for the workflow to reach the suspended state
	time.Sleep(1 * time.Second)

	// Check the workflow state
	state, err := eventWorkflow.GetState(instanceID)
	if err != nil {
		return fmt.Errorf("failed to get workflow state: %w", err)
	}

	fmt.Printf("Workflow state: %s\n", state.Status)
	fmt.Printf("Current step: %s\n", state.CurrentStepID)

	// If workflow is suspended, resume it with additional data
	if state.Status == workflow.EventStatusSuspended {
		fmt.Println("Workflow is suspended. Resuming...")

		// Resume the workflow with the "approval" event and additional data
		err = eventWorkflow.SendEvent(instanceID, "approval", map[string]interface{}{
			"approved": true,
			"notes":    "Approved by Redis example",
		})
		if err != nil {
			return fmt.Errorf("failed to resume workflow: %w", err)
		}

		// Wait for the workflow to continue processing
		time.Sleep(1 * time.Second)

		// Check the final state
		finalState, err := eventWorkflow.GetState(instanceID)
		if err != nil {
			return fmt.Errorf("failed to get final workflow state: %w", err)
		}

		fmt.Printf("Final workflow state: %s\n", finalState.Status)
		fmt.Printf("Results: %v\n", finalState.Results)
	}

	// List all workflow states
	states, err := stateStore.ListWorkflowStates()
	if err != nil {
		return fmt.Errorf("failed to list workflow states: %w", err)
	}

	fmt.Printf("Found %d workflow states in Redis\n", len(states))
	for i, s := range states {
		fmt.Printf("%d. %s (%s) - Status: %s\n", i+1, s.WorkflowName, s.WorkflowID, s.Status)
	}

	return nil
}

// STEP HANDLERS

// handleStep1 is the handler for the first step
func handleStep1(ctx context.Context, eventData interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
	fmt.Println("Executing Step 1: Initial Step")

	// Get input message if available
	var message string
	if msg, ok := stepData["message"].(string); ok {
		message = msg
	} else {
		message = "Default message"
	}

	// Return results and trigger "continue" event to move to next step
	return map[string]interface{}{
		"step1_result": fmt.Sprintf("Processed message: %s", message),
		"timestamp":    time.Now().Format(time.RFC3339),
		"__event":      "continue", // Special field to trigger the next step
	}, nil
}

// handleStep2 is the handler for the second step that will suspend the workflow
func handleStep2(ctx context.Context, eventData interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
	fmt.Println("Executing Step 2: Process Step")

	// Get results from previous step
	var prevResult string
	if res, ok := stepData["step1_result"].(string); ok {
		prevResult = res
	} else {
		prevResult = "No previous result"
	}

	// Check if this is a resume after suspension
	if eventData != nil {
		fmt.Println("Workflow resumed with event data")

		// Extract approval data if available
		approved := false
		notes := ""

		if eventMap, ok := eventData.(map[string]interface{}); ok {
			if app, ok := eventMap["approved"].(bool); ok {
				approved = app
			}
			if n, ok := eventMap["notes"].(string); ok {
				notes = n
			}
		}

		if approved {
			fmt.Println("Approval received! Continuing workflow.")
			return map[string]interface{}{
				"step2_result": fmt.Sprintf("Approved with notes: %s", notes),
				"approval":     true,
				"__event":      "completed", // Trigger the final step
			}, nil
		} else {
			fmt.Println("Approval denied. Ending workflow.")
			return map[string]interface{}{
				"step2_result": "Approval denied",
				"approval":     false,
			}, nil
		}
	}

	// If not resumed, suspend the workflow
	fmt.Println("Suspending workflow, waiting for approval...")
	fmt.Println("The workflow state will be persisted to Redis")

	// Return suspend signal to pause workflow execution
	// Workflow will be persisted to Redis and can be resumed later
	return map[string]interface{}{
		"intermediate_result": fmt.Sprintf("Processed: %s", prevResult),
		"awaiting_approval":   true,
		"__suspend":           true, // Special field to suspend the workflow
	}, nil
}

// handleStep3 is the handler for the final step
func handleStep3(ctx context.Context, eventData interface{}, stepData map[string]interface{}) (map[string]interface{}, error) {
	fmt.Println("Executing Step 3: Final Step")

	// Combine all previous results
	step1Result := ""
	if res, ok := stepData["step1_result"].(string); ok {
		step1Result = res
	}

	step2Result := ""
	if res, ok := stepData["step2_result"].(string); ok {
		step2Result = res
	}

	// Return final results
	return map[string]interface{}{
		"final_result": fmt.Sprintf("Workflow completed successfully. Steps: [%s] -> [%s]",
			step1Result, step2Result),
		"completion_time": time.Now().Format(time.RFC3339),
	}, nil
}
