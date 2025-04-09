package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/yourusername/gostra/pkg/agent"
	"github.com/yourusername/gostra/pkg/models/openai"
	"github.com/yourusername/gostra/pkg/workflow"
)

// This example demonstrates how to create a parallel workflow using Gostra
// It follows the Mastra parallel workflow pattern but adapts it to Go and the Actor model

func main() {
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
		ID:          "product-research-network",
		Name:        "Product Research Network",
		ActorSystem: system,
	}

	network, err := agent.NewAgentNetwork(networkOpts, modelProvider)
	if err != nil {
		log.Fatalf("Failed to create agent network: %v", err)
	}

	// Create a parallel workflow with the new ParallelWorkflowOptions
	parallelOptions := workflow.ParallelWorkflowOptions{
		ID:                    "product-research-workflow",
		Name:                  "Product Research Workflow",
		Description:           "A workflow that conducts parallel research on a product and creates a comprehensive summary",
		MaxParallelExecutions: 3,
		Metadata: map[string]interface{}{
			"version": "1.0",
			"author":  "Gostra Team",
		},
		InputSchema: map[string]interface{}{
			"productName": map[string]interface{}{
				"type":        "string",
				"description": "Name of the product to research",
			},
			"industry": map[string]interface{}{
				"type":        "string",
				"description": "Industry sector for the product",
			},
		},
		OutputSchema: map[string]interface{}{
			"marketResearch": map[string]interface{}{
				"type":        "string",
				"description": "Market research findings",
			},
			"competitorAnalysis": map[string]interface{}{
				"type":        "string",
				"description": "Analysis of competitors",
			},
			"pricingStrategy": map[string]interface{}{
				"type":        "string",
				"description": "Recommended pricing strategy",
			},
			"summary": map[string]interface{}{
				"type":        "string",
				"description": "Comprehensive summary of all research",
			},
		},
	}

	parallelWorkflow, err := workflow.NewParallelWorkflow(parallelOptions)
	if err != nil {
		log.Fatalf("Failed to create parallel workflow: %v", err)
	}

	// Add Market Research Step (Parallel Step 1)
	parallelWorkflow.AddStep(&workflow.ParallelStep{
		ID:            "market-research",
		Name:          "Market Research",
		Description:   "Research the market for the given product",
		PromptFormat:  "Conduct a market research for ${productName} in the ${industry} industry. Analyze current market trends, potential market size, and growth opportunities.",
		OutputKey:     "marketResearch",
		InputKeys:     []string{"productName", "industry"},
		ExecuteFunc:   nil, // Using default model execution
		ModelProvider: modelProvider,
		AgentID:       "",
		Network:       network,
		DependsOn:     []string{}, // No dependencies, can run in parallel
		IsParallel:    true,
		MaxRetries:    2,
		RetryDelay:    time.Second * 5,
	})

	// Add Competitor Analysis Step (Parallel Step 2)
	parallelWorkflow.AddStep(&workflow.ParallelStep{
		ID:            "competitor-analysis",
		Name:          "Competitor Analysis",
		Description:   "Analyze competitors for the product",
		PromptFormat:  "Identify and analyze the top competitors for ${productName} in the ${industry} industry. Include their strengths, weaknesses, market share, and pricing strategies.",
		OutputKey:     "competitorAnalysis",
		InputKeys:     []string{"productName", "industry"},
		ExecuteFunc:   nil, // Using default model execution
		ModelProvider: modelProvider,
		AgentID:       "",
		Network:       network,
		DependsOn:     []string{}, // No dependencies, can run in parallel
		IsParallel:    true,
		MaxRetries:    2,
		RetryDelay:    time.Second * 5,
	})

	// Add Pricing Strategy Step (Parallel Step 3)
	parallelWorkflow.AddStep(&workflow.ParallelStep{
		ID:            "pricing-strategy",
		Name:          "Pricing Strategy",
		Description:   "Develop a pricing strategy for the product",
		PromptFormat:  "Develop a pricing strategy for ${productName} in the ${industry} industry. Consider the perceived value, cost structure, competitor pricing, and market demand.",
		OutputKey:     "pricingStrategy",
		InputKeys:     []string{"productName", "industry"},
		ExecuteFunc:   nil, // Using default model execution
		ModelProvider: modelProvider,
		AgentID:       "",
		Network:       network,
		DependsOn:     []string{}, // No dependencies, can run in parallel
		IsParallel:    true,
		MaxRetries:    2,
		RetryDelay:    time.Second * 5,
	})

	// Add Summary Step (Depends on all parallel steps)
	parallelWorkflow.AddStep(&workflow.ParallelStep{
		ID:          "comprehensive-summary",
		Name:        "Comprehensive Summary",
		Description: "Create a comprehensive summary of all research findings",
		PromptFormat: "Create a comprehensive summary based on the following research findings:\n\n" +
			"Market Research:\n${marketResearch}\n\n" +
			"Competitor Analysis:\n${competitorAnalysis}\n\n" +
			"Pricing Strategy:\n${pricingStrategy}\n\n" +
			"Provide a holistic view of the market position for ${productName} in the ${industry} industry, with actionable recommendations.",
		OutputKey:     "summary",
		InputKeys:     []string{"marketResearch", "competitorAnalysis", "pricingStrategy", "productName", "industry"},
		ExecuteFunc:   nil, // Using default model execution
		ModelProvider: modelProvider,
		AgentID:       "",
		Network:       network,
		DependsOn:     []string{"market-research", "competitor-analysis", "pricing-strategy"}, // Depends on all parallel steps
		IsParallel:    false,
		MaxRetries:    2,
		RetryDelay:    time.Second * 5,
	})

	// Run the workflow with parallel execution
	ctx := context.Background()
	input := map[string]interface{}{
		"productName": "Smart Home Security System",
		"industry":    "Home Automation",
	}

	fmt.Println("Running Parallel Workflow for Product Research...")
	result, err := parallelWorkflow.RunWithParallelExecution(ctx, input)
	if err != nil {
		log.Fatalf("Workflow execution failed: %v", err)
	}

	// Print the results
	fmt.Println("\n=== Market Research ===")
	if marketResearch, ok := result["marketResearch"].(string); ok {
		fmt.Println(marketResearch)
	}

	fmt.Println("\n=== Competitor Analysis ===")
	if compAnalysis, ok := result["competitorAnalysis"].(string); ok {
		fmt.Println(compAnalysis)
	}

	fmt.Println("\n=== Pricing Strategy ===")
	if pricingStrategy, ok := result["pricingStrategy"].(string); ok {
		fmt.Println(pricingStrategy)
	}

	fmt.Println("\n=== Comprehensive Summary ===")
	if summary, ok := result["summary"].(string); ok {
		fmt.Println(summary)
	}

	fmt.Println("\nParallel Workflow completed successfully!")
}
