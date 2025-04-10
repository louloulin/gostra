// Example demonstrating context sharing between agents in an AgentNetwork
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/louloulin/gostra/pkg/agent"
	"github.com/louloulin/gostra/pkg/models/openai"
)

// ResearchAgent handles research tasks
type ResearchAgent struct {
	results []string
}

func (a *ResearchAgent) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *agent.NetworkMessage:
		log.Printf("Research agent received message: %s", msg.Content)

		// Simulate doing research
		researchResults := fmt.Sprintf("Research findings on '%s':\n- Key fact 1\n- Key fact 2\n- Key fact 3", msg.Content)

		// Store results in context
		contextData := make(map[string]interface{})
		if msg.Data != nil {
			if existingData, ok := msg.Data.(map[string]interface{}); ok {
				// Copy existing context
				for k, v := range existingData {
					contextData[k] = v
				}
			}
		}

		// Add research results to context
		contextData["research_results"] = researchResults

		// Send response
		response := &agent.NetworkMessage{
			From:    "research",
			To:      msg.From,
			Content: "Research completed",
			Data:    contextData,
		}

		ctx.Respond(response)
	}
}

// SummaryAgent summarizes information
type SummaryAgent struct{}

func (a *SummaryAgent) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *agent.NetworkMessage:
		log.Printf("Summary agent received message: %s", msg.Content)

		// Get research results from context
		var researchResults string
		if msg.Data != nil {
			if contextData, ok := msg.Data.(map[string]interface{}); ok {
				if results, ok := contextData["research_results"].(string); ok {
					researchResults = results
				}
			}
		}

		// Generate summary based on research results
		summary := "Summary of research:\n"
		if researchResults != "" {
			summary += "Based on the research findings, here is a concise summary...\n"
			summary += "The key points are extracted and organized for clarity."
		} else {
			summary += "No research results found in context."
		}

		// Update context with summary
		contextData := make(map[string]interface{})
		if msg.Data != nil {
			if existingData, ok := msg.Data.(map[string]interface{}); ok {
				// Copy existing context
				for k, v := range existingData {
					contextData[k] = v
				}
			}
		}

		// Add summary to context
		contextData["summary"] = summary

		// Send response
		response := &agent.NetworkMessage{
			From:    "summary",
			To:      msg.From,
			Content: summary,
			Data:    contextData,
		}

		ctx.Respond(response)
	}
}

// runContextSharingExample runs the demonstration
func runContextSharingExample() {
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

	// Create agent network with custom timeouts for better UX
	routerOptions := &agent.RouterOptions{
		DefaultTimeout:    30 * time.Second,
		RoutingTimeout:    15 * time.Second,
		ParallelTimeout:   45 * time.Second,
		SequentialTimeout: 30 * time.Second,
	}

	networkOptions := &agent.AgentNetworkOptions{
		ID:            "research-network",
		Name:          "Research Assistant Network",
		Description:   "A network of agents that research topics and summarize information",
		ActorSystem:   actorSystem,
		RouterOptions: routerOptions,
	}

	network, err := agent.NewAgentNetwork(networkOptions, modelProvider)
	if err != nil {
		log.Fatalf("Failed to create agent network: %v", err)
	}

	// Create and register specialized agents
	researchAgent := &ResearchAgent{}
	summaryAgent := &SummaryAgent{}

	err = network.RegisterAgent("research", researchAgent)
	if err != nil {
		log.Fatalf("Failed to register research agent: %v", err)
	}

	err = network.RegisterAgent("summary", summaryAgent)
	if err != nil {
		log.Fatalf("Failed to register summary agent: %v", err)
	}

	// Create a transmit request to the research agent
	researchRequest := &agent.TransmitRequest{
		Message: "artificial intelligence trends",
		Agents:  []string{"research"},
		Context: map[string]interface{}{
			"user_id":     "user123",
			"request_id":  "req456",
			"timestamp":   time.Now().Unix(),
			"preferences": map[string]interface{}{"detail_level": "high"},
		},
	}

	// Send the research request
	fmt.Println("Sending research request...")
	var researchResults *agent.TransmitResponse

	// Send the request through the network
	responseResult, err := network.SendRequest(context.Background(), researchRequest)
	if err != nil {
		log.Fatalf("Research request failed: %v", err)
	}

	researchResults = responseResult
	fmt.Println("Research completed!")
	for _, r := range responseResult.Results {
		fmt.Printf("Agent %s response: %s\n", r.Agent, r.Content)
	}

	// Extract context from research results
	if researchResults != nil && len(researchResults.Results) > 0 {
		// Create a summary request using the context from research
		summaryRequest := &agent.TransmitRequest{
			Message: "summarize the research findings",
			Agents:  []string{"summary"},
			Context: researchResults.Context, // Pass the context from research
		}

		// Send the summary request
		fmt.Println("\nSending summary request...")

		// Send the request through the network
		summaryResponse, err := network.SendRequest(context.Background(), summaryRequest)
		if err != nil {
			log.Fatalf("Summary request failed: %v", err)
		}

		fmt.Println("Summary completed!")
		for _, r := range summaryResponse.Results {
			fmt.Printf("Agent %s response: %s\n", r.Agent, r.Content)
		}

		// Show final context to demonstrate sharing
		fmt.Println("\nFinal shared context:")
		for k, v := range summaryResponse.Context {
			fmt.Printf("- %s: %v\n", k, v)
		}
	}

	fmt.Println("\nAgent network demonstration completed!")
}

func main() {
	fmt.Println("Starting Agent Network Context Sharing Example")

	// Run the example
	runContextSharingExample()

	fmt.Println("Example completed")
}
