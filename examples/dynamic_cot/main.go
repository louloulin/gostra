package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/google/uuid"
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
		ID:          "dynamic-cot-network",
		Name:        "Dynamic Chain of Thought Agent Network",
		ActorSystem: system,
	}

	network, err := agent.NewAgentNetwork(networkOpts, modelProvider)
	if err != nil {
		log.Fatalf("Failed to create agent network: %v", err)
	}

	// Create RAG agents for different reasoning steps
	agentNames := []string{"research_agent", "reasoning_agent", "synthesis_agent"}
	agents := make(map[string]*actor.PID)

	for _, name := range agentNames {
		agentProps, err := createSpecializedAgent(system, modelProvider, name)
		if err != nil {
			log.Fatalf("Failed to create agent %s: %v", name, err)
		}

		// Spawn the agent actor
		agentPID, err := system.Root.SpawnNamed(agentProps, name)
		if err != nil {
			log.Fatalf("Failed to spawn agent %s: %v", name, err)
		}

		// Register the agent with the network
		network.AddAgentPID(name, agentPID)
		agents[name] = agentPID
	}

	// Define a custom dynamic workflow with specialized agents for each step
	customSteps := []map[string]interface{}{
		{
			"id":          "research",
			"name":        "Research and Context Gathering",
			"description": "Research and gather information about the topic",
			"prompt":      "Research the following topic and gather key information: ${query}\n\nProvide a comprehensive analysis of the available facts.",
			"output_key":  "research_result",
			"input_keys":  []interface{}{"query"},
		},
		{
			"id":          "reasoning",
			"name":        "Reasoning and Analysis",
			"description": "Apply reasoning to analyze the research",
			"prompt":      "Based on the research: ${research_result}\n\nPerform deep analysis and critical thinking about this information. Consider different perspectives, identify patterns, and highlight any contradictions or gaps.",
			"output_key":  "reasoning_result",
			"input_keys":  []interface{}{"research_result"},
		},
		{
			"id":          "synthesis",
			"name":        "Synthesis and Conclusion",
			"description": "Synthesize the reasoning into a coherent conclusion",
			"prompt":      "Based on the research (${research_result}) and reasoning (${reasoning_result}), synthesize a comprehensive conclusion that addresses the original query: ${query}",
			"output_key":  "final_answer",
			"input_keys":  []interface{}{"query", "research_result", "reasoning_result"},
		},
	}

	// Create workflow options with custom steps
	options := &workflow.WorkflowOptions{
		Name:        "Specialized Agent Chain of Thought",
		Description: "Multi-agent chain of thought with context sharing",
		Metadata: map[string]interface{}{
			"steps": customSteps,
		},
	}

	// Build the dynamic workflow
	dynamicWorkflow := workflow.BuildDynamicCOTWorkflow(options, modelProvider, "reasoning_agent", network)

	// Create and configure custom input schema
	inputSchema := workflow.NewSchema(workflow.TypeObject, "Dynamic workflow input schema")

	// Define query field with specific validation rules
	querySchema := workflow.NewSimpleSchema(workflow.TypeString, "The main query to process")
	querySchema.MinLength = 10 // Require meaningful queries
	querySchema.MaxLength = 1000
	inputSchema.AddProperty("query", querySchema, true)

	// Add context_type as an enum
	contextTypeSchema := workflow.NewSimpleSchema(workflow.TypeString, "Type of academic context")
	contextTypeSchema.Enum = []interface{}{"academic", "medical", "technical", "legal", "business"}
	inputSchema.AddProperty("context_type", contextTypeSchema, true)

	// Add depth_level as an enum
	depthSchema := workflow.NewSimpleSchema(workflow.TypeString, "Depth of analysis")
	depthSchema.Enum = []interface{}{"basic", "detailed", "comprehensive"}
	inputSchema.AddProperty("depth_level", depthSchema, true)

	// Set the custom schema on the workflow
	dynamicWorkflow.SetInputSchema(inputSchema)

	// Create and configure output schema
	outputSchema := workflow.NewSchema(workflow.TypeObject, "Dynamic workflow output schema")

	// Each step output should be a string
	researchResultSchema := workflow.NewSimpleSchema(workflow.TypeString, "Research results")
	researchResultSchema.MinLength = 50
	outputSchema.AddProperty("research_result", researchResultSchema, true)

	reasoningResultSchema := workflow.NewSimpleSchema(workflow.TypeString, "Reasoning analysis")
	reasoningResultSchema.MinLength = 50
	outputSchema.AddProperty("reasoning_result", reasoningResultSchema, true)

	finalAnswerSchema := workflow.NewSimpleSchema(workflow.TypeString, "Final answer")
	finalAnswerSchema.MinLength = 100 // Require comprehensive answers
	outputSchema.AddProperty("final_answer", finalAnswerSchema, true)

	// Set the output schema
	dynamicWorkflow.SetOutputSchema(outputSchema)

	// Create a conversation ID for context tracking
	conversationID := fmt.Sprintf("cot-demo-%s", uuid.New().String())

	// Run the workflow with context sharing
	ctx := context.Background()
	input := map[string]interface{}{
		"query":        "What are the ethical implications of using large language models in healthcare?",
		"context_type": "academic",
		"depth_level":  "comprehensive",
	}

	fmt.Println("Running Dynamic Chain of Thought workflow with context sharing...")
	result, err := dynamicWorkflow.RunWithSharedContext(ctx, input, conversationID)
	if err != nil {
		log.Fatalf("Workflow execution failed: %v", err)
	}

	// Print the results from each step
	fmt.Println("\n=== Research ===")
	fmt.Println(result["research_result"])

	fmt.Println("\n=== Reasoning ===")
	fmt.Println(result["reasoning_result"])

	fmt.Println("\n=== Final Answer ===")
	fmt.Println(result["final_answer"])

	fmt.Println("\nWorkflow completed successfully!")

	// Clean up
	system.Root.Stop(agents["research_agent"])
	system.Root.Stop(agents["reasoning_agent"])
	system.Root.Stop(agents["synthesis_agent"])
}

// createSpecializedAgent creates an agent with a specialized role
func createSpecializedAgent(system *actor.ActorSystem, modelProvider models.ModelProvider, role string) (*actor.Props, error) {
	// Customize the agent based on its role
	var systemPrompt string
	var toolList []tools.Tool

	// Create vector search tool that all agents can use
	vectorSearchOpts := search.VectorSearchOptions{
		Dimension:      1536,
		IndexType:      search.IndexTypeFlat,
		DistanceMetric: "cosine",
	}
	searchTool := search.NewVectorSearchTool(vectorSearchOpts)
	toolList = append(toolList, searchTool)

	// Set specialized system prompts based on agent role
	switch role {
	case "research_agent":
		systemPrompt = "You are a specialized research agent focused on gathering and analyzing information. Your primary goal is to collect relevant facts and data about a topic, ensuring comprehensive coverage of the subject matter. Present information in an organized, factual manner."
	case "reasoning_agent":
		systemPrompt = "You are a specialized reasoning agent focused on critical thinking and analysis. Your primary goal is to examine information critically, identify patterns, contradictions, and logical connections. Consider multiple perspectives and provide balanced analytical insights."
	case "synthesis_agent":
		systemPrompt = "You are a specialized synthesis agent focused on bringing together diverse information into coherent conclusions. Your primary goal is to create a comprehensive understanding from various inputs, forming well-reasoned conclusions that address the core question."
	default:
		systemPrompt = "You are a specialized agent in an agent network. Your goal is to provide expert assistance based on your specialized knowledge and capabilities."
	}

	// Create agent options
	agentOpts := &agent.ActorAgentOptions{
		ID:            role,
		Name:          fmt.Sprintf("%s Agent", role),
		SystemPrompt:  systemPrompt,
		ModelProvider: modelProvider,
		Tools:         toolList,
		MaxTokens:     4000,
		ActorSystem:   system,
	}

	// Create the agent
	return agent.NewActorAgent(agentOpts)
}
