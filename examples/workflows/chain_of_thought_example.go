package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/yourusername/gostra/pkg/agent"
	"github.com/yourusername/gostra/pkg/models"
	"github.com/yourusername/gostra/pkg/models/openai"
	"github.com/yourusername/gostra/pkg/tools"
	"github.com/yourusername/gostra/pkg/tools/search"
	"github.com/yourusername/gostra/pkg/workflow"
)

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
		ID:          "chain-of-thought-network",
		Name:        "Chain of Thought Agent Network",
		ActorSystem: system,
	}

	network, err := agent.NewAgentNetwork(networkOpts, modelProvider)
	if err != nil {
		log.Fatalf("Failed to create agent network: %v", err)
	}

	// Create a RAG agent
	ragAgentProps, err := createRAGAgent(system, modelProvider)
	if err != nil {
		log.Fatalf("Failed to create RAG agent: %v", err)
	}

	// Spawn the agent actor
	ragAgentPID, err := system.Root.SpawnNamed(ragAgentProps, "rag_agent")
	if err != nil {
		log.Fatalf("Failed to spawn RAG agent: %v", err)
	}

	// Register the agent with the network
	network.AddAgentPID("rag_agent", ragAgentPID)

	// Create a Chain of Thought workflow
	cogWorkflow := workflow.BuildRAGWorkflow(
		"Document Analysis Workflow",
		modelProvider,
		"rag_agent",
		network,
	)

	// Run the workflow
	ctx := context.Background()
	input := map[string]interface{}{
		"query": "Summarize the key points from the document about climate change and agriculture.",
		// You would typically include document content here
		"documents": []string{
			"Climate change is affecting agricultural production globally.",
			"Rising temperatures are leading to changes in growing seasons.",
			"Extreme weather events are becoming more frequent and severe.",
			"Farmers are adopting new techniques to adapt to changing conditions.",
			"Some regions are experiencing increased yields, while others see decreases.",
		},
	}

	fmt.Println("Running Chain of Thought workflow...")
	result, err := cogWorkflow.Run(ctx, input)
	if err != nil {
		log.Fatalf("Workflow execution failed: %v", err)
	}

	// Print the results from each step
	fmt.Println("\n=== Analysis ===")
	fmt.Println(result["initialAnalysis"])

	fmt.Println("\n=== Thought Process ===")
	fmt.Println(result["breakdown"])

	fmt.Println("\n=== Connections ===")
	fmt.Println(result["connections"])

	fmt.Println("\n=== Conclusions ===")
	fmt.Println(result["conclusions"])

	fmt.Println("\n=== Final Answer ===")
	fmt.Println(result["answer"])

	fmt.Println("\nWorkflow completed successfully!")
}

// createRAGAgent creates an agent with vector search capabilities
func createRAGAgent(system *actor.ActorSystem, modelProvider models.ModelProvider) (*actor.Props, error) {
	// Create a vector search tool
	vectorSearchOpts := search.VectorSearchOptions{
		Dimension:      1536,
		IndexType:      search.IndexTypeFlat,
		DistanceMetric: "cosine",
	}

	// Create the search tool
	searchTool := search.NewVectorSearchTool(vectorSearchOpts)

	// Create agent options
	agentOpts := &agent.ActorAgentOptions{
		ID:            "rag_agent",
		Name:          "RAG Agent",
		SystemPrompt:  "You are a Retrieval-Augmented Generation agent specialized in analyzing documents. Your task is to carefully read the provided context and respond to questions about the content. You should perform deep analysis, consider implications, and draw connections between different pieces of information.",
		ModelProvider: modelProvider,
		Tools:         []tools.Tool{searchTool},
		MaxTokens:     4000,
		ActorSystem:   system,
	}

	// Create the agent
	return agent.NewActorAgent(agentOpts)
}
