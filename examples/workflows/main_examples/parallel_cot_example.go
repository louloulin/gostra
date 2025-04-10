package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/louloulin/gostra/pkg/actor"
	"github.com/louloulin/gostra/pkg/agent"
	"github.com/louloulin/gostra/pkg/models"
	"github.com/louloulin/gostra/pkg/schema"
	"github.com/louloulin/gostra/pkg/workflow"
)

func main() {
	// Initialize actor system
	actorSystem := actor.NewActorSystem(actor.ActorSystemOptions{})

	// Create an agent network for parallelism
	network := agent.NewAgentNetwork(actorSystem, &agent.AgentNetworkOptions{
		Name: "Parallel Workflow Network",
	})

	// Create the model provider
	modelProvider := models.NewOpenAIModelProvider(&models.OpenAIModelProviderOptions{
		ApiKey:   "YOUR_API_KEY",
		Model:    "gpt-4",
		MaxRetry: 3,
		Timeout:  time.Second * 60,
	})

	// Create input schema
	inputSchema := schema.NewSchema()
	inputSchema.Properties["query"] = &schema.Property{
		Type:        schema.TypeString,
		Description: "User query to analyze",
		Required:    true,
	}

	// Create output schema
	outputSchema := schema.NewSchema()
	outputSchema.Properties["final_answer"] = &schema.Property{
		Type:        schema.TypeString,
		Description: "Final synthesized answer to the query",
		Required:    true,
	}
	outputSchema.Properties["context_analysis"] = &schema.Property{
		Type:        schema.TypeString,
		Description: "Analysis of the query context",
		Required:    true,
	}
	outputSchema.Properties["topic_exploration"] = &schema.Property{
		Type:        schema.TypeString,
		Description: "Exploration of main topics in the query",
		Required:    true,
	}
	outputSchema.Properties["critical_analysis"] = &schema.Property{
		Type:        schema.TypeString,
		Description: "Critical analysis of the query",
		Required:    true,
	}
	outputSchema.Properties["pros_analysis"] = &schema.Property{
		Type:        schema.TypeString,
		Description: "Analysis of positive aspects",
		Required:    true,
	}
	outputSchema.Properties["cons_analysis"] = &schema.Property{
		Type:        schema.TypeString,
		Description: "Analysis of negative aspects",
		Required:    true,
	}

	// Create workflow options with schema validation
	workflowOptions := &workflow.WorkflowOptions{
		Name:        "Parallel Critical Analysis Workflow",
		Description: "A workflow that performs critical analysis with parallel steps",
		Metadata: map[string]interface{}{
			"version": "1.0.0",
			"type":    "analysis",
		},
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
	}

	// Build the parallel workflow
	cotWorkflow := workflow.BuildParallelCOTWorkflow(
		workflowOptions,
		modelProvider,
		"parallel-cot-agent",
		network,
	)

	// Start the network
	if err := network.Start(context.Background()); err != nil {
		log.Fatalf("Failed to start agent network: %v", err)
	}
	defer network.Shutdown(context.Background())

	// Test the workflow with different inputs
	testQueries := []string{
		"What are the implications of AI in healthcare?",
		"Should governments regulate cryptocurrency?",
	}

	for i, query := range testQueries {
		fmt.Printf("\n\n===== Running Test %d: %s =====\n\n", i+1, query)

		// Create input data
		inputData := map[string]interface{}{
			"query": query,
		}

		// Set start time to measure performance improvement with parallelism
		startTime := time.Now()

		// Run the workflow with parallel execution
		result, err := cotWorkflow.RunWithParallelSteps(context.Background(), inputData)
		if err != nil {
			log.Printf("Error running parallel workflow: %v", err)
			continue
		}

		// Calculate execution time
		executionTime := time.Since(startTime)

		// Print results
		fmt.Printf("Execution time: %v\n", executionTime)
		fmt.Println("Final Answer:")
		fmt.Println(result["final_answer"])

		// Output all results as JSON for detailed inspection
		resultBytes, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println("\nFull Results:")
		fmt.Println(string(resultBytes))
	}

	fmt.Println("\nWorkflow execution completed successfully!")
}
