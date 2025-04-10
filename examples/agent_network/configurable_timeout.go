package main

import (
	"fmt"
	"log"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/louloulin/gostra/pkg/agent"
	"github.com/louloulin/gostra/pkg/models/openai"
)

func main() {
	// Create an actor system
	actorSystem := actor.NewActorSystem()

	// Initialize a model provider
	modelProvider, err := openai.NewOpenAIProvider(&openai.Options{
		APIKey: "your-api-key", // Replace with actual API key in production
		Model:  "gpt-3.5-turbo",
	})
	if err != nil {
		log.Fatalf("Failed to create OpenAI provider: %v", err)
	}

	// Create custom router options with different timeouts
	routerOptions := &agent.RouterOptions{
		DefaultTimeout:    30 * time.Second, // Default timeout for agent communication
		RoutingTimeout:    15 * time.Second, // Timeout for LLM routing decisions
		ParallelTimeout:   45 * time.Second, // Timeout for parallel agent calls
		SequentialTimeout: 30 * time.Second, // Timeout for sequential agent calls
	}

	// Create agent network with custom timeouts
	networkOptions := &agent.AgentNetworkOptions{
		ID:            "example-network",
		Name:          "Example Network",
		Description:   "An example network with configurable timeouts",
		ActorSystem:   actorSystem,
		RouterOptions: routerOptions,
	}

	// This example just demonstrates timeout configuration
	// In a real application, you would create and register agent actors
	_, err = agent.NewAgentNetwork(networkOptions, modelProvider)
	if err != nil {
		log.Fatalf("Failed to create agent network: %v", err)
	}

	fmt.Println("Agent network created with the following timeouts:")
	fmt.Printf("- Default timeout: %s\n", routerOptions.DefaultTimeout)
	fmt.Printf("- Routing timeout: %s\n", routerOptions.RoutingTimeout)
	fmt.Printf("- Parallel calls timeout: %s\n", routerOptions.ParallelTimeout)
	fmt.Printf("- Sequential calls timeout: %s\n", routerOptions.SequentialTimeout)

	// In a real application, you would register agents and send messages
	// This example only demonstrates the configuration setup
}
