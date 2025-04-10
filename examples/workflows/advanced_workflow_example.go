package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/louloulin/gostra/pkg/agent"
	"github.com/louloulin/gostra/pkg/models/openai"
	"github.com/louloulin/gostra/pkg/workflow"
)

// This example demonstrates how to use advanced workflow features in Gostra:
// - Conditional branching (if/else)
// - Looping (while/until)
// - Nested workflows
// - Parallel execution

func runAdvancedWorkflowExample() {
	// Set up OpenAI API key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	// Create an actor system
	system := actor.NewActorSystem()

	// Create model provider
	modelProvider, err := openai.NewOpenAIProvider(&openai.Options{
		APIKey: apiKey,
		Model:  "gpt-4",
	})
	if err != nil {
		log.Fatalf("Failed to create OpenAI provider: %v", err)
	}

	// Create an agent network
	networkOpts := &agent.AgentNetworkOptions{
		ID:          "advanced-workflow-network",
		Name:        "Advanced Workflow Network",
		ActorSystem: system,
	}

	network, err := agent.NewAgentNetwork(networkOpts, modelProvider)
	if err != nil {
		log.Fatalf("Failed to create agent network: %v", err)
	}

	// 1. First, let's create a simple nested workflow for data processing
	dataProcessingWorkflow, err := createDataProcessingWorkflow(modelProvider, network)
	if err != nil {
		log.Fatalf("Failed to create data processing workflow: %v", err)
	}

	// 2. Create our main workflow with advanced features
	advancedWorkflow, err := createAdvancedWorkflow(modelProvider, network, dataProcessingWorkflow)
	if err != nil {
		log.Fatalf("Failed to create advanced workflow: %v", err)
	}

	// Run the workflow
	ctx := context.Background()
	input := map[string]interface{}{
		"topic":       "Artificial Intelligence",
		"user_level":  "beginner",
		"max_retries": 3,
	}

	fmt.Println("Running Advanced Workflow...")
	result, err := advancedWorkflow.Run(ctx, input)
	if err != nil {
		log.Fatalf("Workflow execution failed: %v", err)
	}

	// Print the results
	fmt.Println("\n=== Final Results ===")
	printResults(result)
}

// Create a nested workflow for data processing
func createDataProcessingWorkflow(modelProvider openai.ModelProvider, network *agent.AgentNetwork) (*workflow.NestedWorkflow, error) {
	// Create the nested workflow for data processing
	dataProcessingOpts := workflow.ParallelWorkflowOptions{
		ID:                    "data-processing-workflow",
		Name:                  "Data Processing Workflow",
		Description:           "A workflow that processes data on a topic",
		MaxParallelExecutions: 2,
	}

	dataWorkflow, err := workflow.NewNestedWorkflow(dataProcessingOpts)
	if err != nil {
		return nil, err
	}

	// Add a step to collect data
	collectDataStep := &workflow.Step{
		ID:          "collect-data",
		Name:        "Collect Data",
		Description: "Collect information on the topic",
		Execute: func(ctx context.Context, input map[string]interface{}, w *workflow.Workflow) (map[string]interface{}, error) {
			topic := input["topic"].(string)
			fmt.Printf("Collecting data on: %s\n", topic)

			// Simulate data collection
			time.Sleep(500 * time.Millisecond)

			return map[string]interface{}{
				"raw_data": fmt.Sprintf("Raw data about %s collected from various sources", topic),
			}, nil
		},
	}

	// Add a step to analyze data
	analyzeDataStep := &workflow.Step{
		ID:          "analyze-data",
		Name:        "Analyze Data",
		Description: "Analyze the collected data",
		Execute: func(ctx context.Context, input map[string]interface{}, w *workflow.Workflow) (map[string]interface{}, error) {
			rawData := input["raw_data"].(string)
			fmt.Printf("Analyzing data: %s\n", rawData)

			// Simulate data analysis
			time.Sleep(500 * time.Millisecond)

			return map[string]interface{}{
				"analyzed_data": fmt.Sprintf("Analysis of %s", rawData),
			}, nil
		},
	}

	// Add a step to format results
	formatResultsStep := &workflow.Step{
		ID:          "format-results",
		Name:        "Format Results",
		Description: "Format the analyzed data into a presentable format",
		Execute: func(ctx context.Context, input map[string]interface{}, w *workflow.Workflow) (map[string]interface{}, error) {
			analyzedData := input["analyzed_data"].(string)
			fmt.Printf("Formatting results: %s\n", analyzedData)

			// Simulate formatting
			time.Sleep(500 * time.Millisecond)

			return map[string]interface{}{
				"formatted_data": fmt.Sprintf("Formatted version of: %s", analyzedData),
			}, nil
		},
	}

	// Add the steps to the workflow
	dataWorkflow.AddStep(&workflow.ParallelStep{
		ID:          "collect-data",
		Name:        "Collect Data",
		Description: "Collect information on the topic",
		OutputKey:   "raw_data",
		InputKeys:   []string{"topic"},
		ExecuteFunc: func(ctx context.Context, args *workflow.ParallelStepArgs) (string, error) {
			topic := args.Context["topic"].(string)
			return fmt.Sprintf("Raw data about %s collected from various sources", topic), nil
		},
	})

	dataWorkflow.AddStep(&workflow.ParallelStep{
		ID:          "analyze-data",
		Name:        "Analyze Data",
		Description: "Analyze the collected data",
		OutputKey:   "analyzed_data",
		InputKeys:   []string{"raw_data"},
		DependsOn:   []string{"collect-data"},
		ExecuteFunc: func(ctx context.Context, args *workflow.ParallelStepArgs) (string, error) {
			rawData := args.StepInputs["raw_data"]
			return fmt.Sprintf("Analysis of %s", rawData), nil
		},
	})

	dataWorkflow.AddStep(&workflow.ParallelStep{
		ID:          "format-results",
		Name:        "Format Results",
		Description: "Format the analyzed data into a presentable format",
		OutputKey:   "formatted_data",
		InputKeys:   []string{"analyzed_data"},
		DependsOn:   []string{"analyze-data"},
		ExecuteFunc: func(ctx context.Context, args *workflow.ParallelStepArgs) (string, error) {
			analyzedData := args.StepInputs["analyzed_data"]
			return fmt.Sprintf("Formatted version of: %s", analyzedData), nil
		},
	})

	return dataWorkflow, nil
}

// Create the main advanced workflow
func createAdvancedWorkflow(modelProvider openai.ModelProvider, network *agent.AgentNetwork, dataProcessingWorkflow *workflow.NestedWorkflow) (*workflow.NestedWorkflow, error) {
	// Create the main workflow
	advancedOpts := workflow.ParallelWorkflowOptions{
		ID:                    "advanced-workflow",
		Name:                  "Advanced Workflow Example",
		Description:           "A workflow demonstrating conditional branching, looping, and nested workflows",
		MaxParallelExecutions: 3,
	}

	advancedWorkflow, err := workflow.NewNestedWorkflow(advancedOpts)
	if err != nil {
		return nil, err
	}

	// Initial step to validate input
	validateInputStep := &workflow.Step{
		ID:          "validate-input",
		Name:        "Validate Input",
		Description: "Validate the input parameters",
		Execute: func(ctx context.Context, input map[string]interface{}, w *workflow.Workflow) (map[string]interface{}, error) {
			topic, topicExists := input["topic"].(string)
			userLevel, levelExists := input["user_level"].(string)

			if !topicExists || topic == "" {
				return nil, fmt.Errorf("topic is required")
			}

			if !levelExists || userLevel == "" {
				userLevel = "intermediate" // Default if not provided
				fmt.Println("User level not provided, defaulting to 'intermediate'")
			}

			return map[string]interface{}{
				"topic":      topic,
				"user_level": userLevel,
				"is_valid":   true,
			}, nil
		},
	}

	// Add the validate input step to the workflow
	advancedWorkflow.AddStep(&workflow.ParallelStep{
		ID:          "validate-input",
		Name:        "Validate Input",
		Description: "Validate the input parameters",
		OutputKey:   "validation_result",
		InputKeys:   []string{"topic", "user_level"},
		ExecuteFunc: func(ctx context.Context, args *workflow.ParallelStepArgs) (string, error) {
			topic, topicExists := args.Context["topic"].(string)
			userLevel, levelExists := args.Context["user_level"].(string)

			if !topicExists || topic == "" {
				return "", fmt.Errorf("topic is required")
			}

			if !levelExists || userLevel == "" {
				args.Context["user_level"] = "intermediate" // Default if not provided
				fmt.Println("User level not provided, defaulting to 'intermediate'")
			}

			return "Input validation successful", nil
		},
	})

	// Conditional step - Different paths based on user level
	beginnerPathStep := &workflow.Step{
		ID:          "beginner-path",
		Name:        "Beginner Path",
		Description: "Processing for beginner users",
		Execute: func(ctx context.Context, input map[string]interface{}, w *workflow.Workflow) (map[string]interface{}, error) {
			topic := input["topic"].(string)
			fmt.Printf("Processing %s for beginners\n", topic)

			// Simulate processing
			time.Sleep(500 * time.Millisecond)

			return map[string]interface{}{
				"content": fmt.Sprintf("Basic introduction to %s for beginners", topic),
			}, nil
		},
	}

	advancedPathStep := &workflow.Step{
		ID:          "advanced-path",
		Name:        "Advanced Path",
		Description: "Processing for advanced users",
		Execute: func(ctx context.Context, input map[string]interface{}, w *workflow.Workflow) (map[string]interface{}, error) {
			topic := input["topic"].(string)
			fmt.Printf("Processing %s for advanced users\n", topic)

			// Simulate more complex processing
			time.Sleep(1000 * time.Millisecond)

			return map[string]interface{}{
				"content": fmt.Sprintf("In-depth analysis of %s for advanced users", topic),
			}, nil
		},
	}

	// Add conditional branching
	userLevelCondition := func(ctx context.Context, input map[string]interface{}) (bool, error) {
		userLevel, exists := input["user_level"].(string)
		return exists && userLevel == "beginner", nil
	}

	advancedWorkflow.IfThen(
		"user-level-branch",
		"User Level Branch",
		userLevelCondition,
		beginnerPathStep,
		advancedPathStep,
		[]string{"validate-input"},
	)

	// Add a loop step to retry if needed
	retryStep := &workflow.Step{
		ID:          "retry-step",
		Name:        "Retry Step",
		Description: "Step that can be retried multiple times",
		Execute: func(ctx context.Context, input map[string]interface{}, w *workflow.Workflow) (map[string]interface{}, error) {
			iteration, _ := input["_iteration"].(int)
			fmt.Printf("Executing retry step, iteration %d\n", iteration)

			// Simulate occasional failures for demo purposes
			if iteration < 2 {
				fmt.Println("Simulating a failure in the retry step")
				return map[string]interface{}{
					"success": false,
					"attempt": iteration + 1,
				}, nil
			}

			fmt.Println("Retry step succeeded")
			return map[string]interface{}{
				"success": true,
				"attempt": iteration + 1,
				"result":  "Successfully completed after retries",
			}, nil
		},
	}

	// Add until loop - retry until success or max attempts
	untilCondition := func(ctx context.Context, input map[string]interface{}, iterations int) (bool, error) {
		// Stop when success is true or we hit max retries
		success, _ := input["success"].(bool)
		maxRetries, _ := input["max_retries"].(int)

		return success || iterations >= maxRetries, nil
	}

	advancedWorkflow.Until(
		"retry-until-success",
		"Retry Until Success",
		untilCondition,
		retryStep,
		5, // Max iterations as a safeguard
		[]string{"user-level-branch"},
	)

	// Now add a nested workflow step to process data
	// Define input/output mappings for the nested workflow
	inputMapping := map[string]string{
		"topic": "topic", // Map parent's topic to child's topic
	}

	outputMapping := map[string]workflow.OutputMappingDef{
		"formatted_data": {
			SourceKey: "formatted_data",
			TargetKey: "processed_data",
		},
	}

	// Add the nested workflow as a step
	advancedWorkflow.WithWorkflow(
		"data-processing",
		"Data Processing",
		dataProcessingWorkflow,
		inputMapping,
		outputMapping,
		[]string{"retry-until-success"},
	)

	// Add a final step to prepare the output
	advancedWorkflow.AddStep(&workflow.ParallelStep{
		ID:          "prepare-output",
		Name:        "Prepare Output",
		Description: "Prepare the final output combining all previous results",
		OutputKey:   "final_output",
		InputKeys:   []string{"content", "processed_data"},
		DependsOn:   []string{"data-processing"},
		ExecuteFunc: func(ctx context.Context, args *workflow.ParallelStepArgs) (string, error) {
			content := args.StepInputs["content"]
			processedData := args.StepInputs["processed_data"]

			return fmt.Sprintf("Final output:\n\n%s\n\nWith additional data:\n%s", content, processedData), nil
		},
	})

	return advancedWorkflow, nil
}

// Helper function to print workflow results
func printResults(results map[string]interface{}) {
	for key, value := range results {
		if key != "_nested_result" { // Skip the full nested result for cleaner output
			fmt.Printf("%s: %v\n", key, value)
		}
	}
}

func main() {
	runAdvancedWorkflowExample()
}
