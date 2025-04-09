package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/yourusername/gostra/pkg/agent"
	"github.com/yourusername/gostra/pkg/models/openai"
)

func main() {
	// Create an actor system
	actorSystem := actor.NewActorSystem()

	// Initialize a model provider
	modelProvider, err := openai.NewOpenAIProvider(&openai.Options{
		APIKey: "your-api-key",
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

	network, err := agent.NewAgentNetwork(networkOptions, modelProvider)
	if err != nil {
		log.Fatalf("Failed to create agent network: %v", err)
	}

	// Create and register example agents
	// These would typically be specialized for different tasks
	weatherAgent := createExampleAgent("weather-agent", "I provide weather information", actorSystem)
	travelAgent := createExampleAgent("travel-agent", "I provide travel recommendations", actorSystem)
	researchAgent := createExampleAgent("research-agent", "I research topics and provide information", actorSystem)

	// Register agents with the network
	network.RegisterAgent("weather", weatherAgent)
	network.RegisterAgent("travel", travelAgent)
	network.RegisterAgent("research", researchAgent)

	// Start the network
	if err := network.Start(); err != nil {
		log.Fatalf("Failed to start network: %v", err)
	}

	// Example of using the network with a message
	msg := &agent.NetworkMessage{
		Content: "What's the weather like in New York today and what should I pack for my trip?",
	}

	// Send the message to the network
	err = network.Transmit(context.Background(), msg)
	if err != nil {
		log.Fatalf("Failed to transmit message: %v", err)
	}

	// In a real application, you would wait for and process the response
	fmt.Println("Message transmitted successfully")

	// Stop the network when done
	if err := network.Stop(); err != nil {
		log.Fatalf("Failed to stop network: %v", err)
	}
}

// createExampleAgent creates a simple example agent
func createExampleAgent(id string, description string, system *actor.ActorSystem) *actor.PID {
	// Create an agent
	exampleAgent := &ExampleAgent{
		id:          id,
		description: description,
	}

	// Create props for the agent
	props := actor.PropsFromProducer(func() actor.Actor {
		return exampleAgent
	})

	// Spawn the actor
	pid, err := system.Root.SpawnNamed(props, id)
	if err != nil {
		log.Fatalf("Failed to spawn agent %s: %v", id, err)
	}

	return pid
}

// ExampleAgent is a simple agent implementation
type ExampleAgent struct {
	id          string
	description string
}

// Receive handles messages sent to this agent
func (a *ExampleAgent) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *agent.NetworkMessage:
		// Process the message
		fmt.Printf("Agent %s received: %s\n", a.id, msg.Content)

		// Create a response
		response := &agent.NetworkMessage{
			From:    a.id,
			To:      msg.From,
			Content: fmt.Sprintf("Response from %s: I processed your request about '%s'", a.id, msg.Content),
		}

		// Send response back
		ctx.Respond(response)
	}
}
