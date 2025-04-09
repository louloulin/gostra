package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/yourusername/gostra/pkg/models"
)

// RouterOptions contains configuration options for the RouterAgent
type RouterOptions struct {
	DefaultTimeout    time.Duration // Default timeout for agent communication
	RoutingTimeout    time.Duration // Timeout specifically for LLM routing decisions
	ParallelTimeout   time.Duration // Timeout for parallel agent calls
	SequentialTimeout time.Duration // Timeout for sequential agent calls
}

// DefaultRouterOptions returns the default options for RouterAgent
func DefaultRouterOptions() *RouterOptions {
	return &RouterOptions{
		DefaultTimeout:    30 * time.Second,
		RoutingTimeout:    15 * time.Second,
		ParallelTimeout:   45 * time.Second,
		SequentialTimeout: 30 * time.Second,
	}
}

// RouterAgent handles dynamic message routing in the agent network
type RouterAgent struct {
	network *AgentNetwork
	model   models.ModelProvider
	options *RouterOptions
}

// NewRouterAgent creates a new router agent
func NewRouterAgent(network *AgentNetwork, options *RouterOptions) *RouterAgent {
	if options == nil {
		options = DefaultRouterOptions()
	}

	return &RouterAgent{
		network: network,
		model:   network.model,
		options: options,
	}
}

// Receive handles incoming messages
func (r *RouterAgent) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *NetworkMessage:
		r.handleNetworkMessage(context, msg)
	case *TransmitRequest:
		r.handleTransmitRequest(context, msg)
	case *actor.Started:
		// Initialize router
	case *actor.Stopping:
		// Cleanup
	}
}

// TransmitRequest represents a request to transmit a message to one or more agents
type TransmitRequest struct {
	Message      string                 `json:"message"`      // The content to transmit
	Agents       []string               `json:"agents"`       // Optional: Specific agents to transmit to
	ParallelCall bool                   `json:"parallelCall"` // Whether to call agents in parallel
	Context      map[string]interface{} `json:"context"`      // Shared context
}

// TransmitResponse represents the response from a transmit request
type TransmitResponse struct {
	Results    []AgentResponse        `json:"results"`    // Results from agent calls
	NextAgents []string               `json:"nextAgents"` // Suggested next agents to call
	Context    map[string]interface{} `json:"context"`    // Updated shared context
	Error      string                 `json:"error,omitempty"`
}

// AgentResponse represents a response from a single agent
type AgentResponse struct {
	Agent   string      `json:"agent"`   // The agent that responded
	Content string      `json:"content"` // The response content
	Data    interface{} `json:"data,omitempty"`
}

// handleTransmitRequest processes a transmit request and determines routing
func (r *RouterAgent) handleTransmitRequest(ctx actor.Context, req *TransmitRequest) {
	var response TransmitResponse
	response.Context = req.Context
	if response.Context == nil {
		response.Context = make(map[string]interface{})
	}

	// If specific agents are provided, route to them
	if len(req.Agents) > 0 {
		if req.ParallelCall {
			results, err := r.callAgentsInParallel(ctx, req)
			if err != nil {
				response.Error = err.Error()
				ctx.Respond(&response)
				return
			}
			response.Results = results
		} else {
			// Call agents sequentially
			results, err := r.callAgentsSequentially(ctx, req)
			if err != nil {
				response.Error = err.Error()
				ctx.Respond(&response)
				return
			}
			response.Results = results
		}

		ctx.Respond(&response)
		return
	}

	// Use LLM to determine routing
	agents, err := r.determineRoutingWithLLM(ctx, req)
	if err != nil {
		response.Error = fmt.Sprintf("LLM routing error: %v", err)
		ctx.Respond(&response)
		return
	}

	// Call the determined agents
	req.Agents = agents
	results, err := r.callAgentsSequentially(ctx, req)
	if err != nil {
		response.Error = err.Error()
		ctx.Respond(&response)
		return
	}

	response.Results = results

	// Suggest next agents that might be relevant
	nextAgents, err := r.suggestNextAgents(ctx, req, results)
	if err != nil {
		// Non-critical error, just log
		fmt.Printf("Error suggesting next agents: %v\n", err)
	} else {
		response.NextAgents = nextAgents
	}

	ctx.Respond(&response)
}

// determineRoutingWithLLM uses the model to determine the best agent(s) to handle the request
func (r *RouterAgent) determineRoutingWithLLM(ctx actor.Context, req *TransmitRequest) ([]string, error) {
	// Create a prompt for the model
	agentDescriptions := r.getAgentDescriptions()

	prompt := fmt.Sprintf(`You are a router in a multi-agent system. Based on the user request, determine which agent(s) would be best to handle it.
	
User Request: %s
	
Available Agents:
%s
	
Context:
%v
	
Return only the name(s) of the agent(s) that should handle this request in JSON format like {"agents": ["agent1", "agent2"]}`,
		req.Message,
		agentDescriptions,
		req.Context)

	// Call the model
	generateRequest := []models.Message{
		{
			Role:    "system",
			Content: "You are a router agent that determines which specialized agents should process a request.",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}

	responseText, err := r.model.Generate(context.Background(), generateRequest, &models.GenerateOptions{
		Temperature: 0.1, // Low temperature for more deterministic routing
	})

	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	// Parse the response
	var result struct {
		Agents []string `json:"agents"`
	}

	err = json.Unmarshal([]byte(responseText), &result)
	if err != nil {
		// Fallback: try to parse the text directly if JSON parsing fails
		// This handles cases where the model didn't return valid JSON
		return []string{responseText}, nil
	}

	return result.Agents, nil
}

// callAgentsInParallel calls multiple agents in parallel and collects their responses
func (r *RouterAgent) callAgentsInParallel(ctx actor.Context, req *TransmitRequest) ([]AgentResponse, error) {
	var wg sync.WaitGroup
	results := make([]AgentResponse, len(req.Agents))
	errors := make([]error, len(req.Agents))

	// Create a shared context map (thread-safe map for parallel updates)
	type contextUpdate struct {
		agentName string
		data      map[string]interface{}
	}
	contextChan := make(chan contextUpdate, len(req.Agents))

	// Add a conversation trace if not present
	updatedContext := make(map[string]interface{})
	if req.Context != nil {
		for k, v := range req.Context {
			updatedContext[k] = v
		}
	}

	if _, hasTrace := updatedContext["conversation_trace"]; !hasTrace {
		updatedContext["conversation_trace"] = []string{}
	}

	if _, hasContribs := updatedContext["agent_contributions"]; !hasContribs {
		updatedContext["agent_contributions"] = make(map[string]interface{})
	}

	for i, agentName := range req.Agents {
		wg.Add(1)
		go func(index int, name string) {
			defer wg.Done()

			agent, err := r.network.GetAgent(name)
			if err != nil {
				errors[index] = fmt.Errorf("agent %s not found", name)
				return
			}

			// Clone the context to avoid race conditions
			agentContext := make(map[string]interface{})
			for k, v := range updatedContext {
				agentContext[k] = v
			}

			// Update conversation trace for this agent
			if trace, ok := agentContext["conversation_trace"].([]string); ok {
				traceClone := make([]string, len(trace))
				copy(traceClone, trace)
				traceClone = append(traceClone, fmt.Sprintf("router -> %s", name))
				agentContext["conversation_trace"] = traceClone
			}

			// Create message for agent
			message := &NetworkMessage{
				From:    "router",
				To:      name,
				Content: req.Message,
				Data:    agentContext,
			}

			// Send message to agent
			// Use the parallel timeout for parallel calls
			timeout := r.options.ParallelTimeout
			future := ctx.RequestFuture(agent, message, timeout)
			result, err := future.Result()
			if err != nil {
				errors[index] = err
				return
			}

			// Process response
			if response, ok := result.(*NetworkMessage); ok {
				results[index] = AgentResponse{
					Agent:   name,
					Content: response.Content,
					Data:    response.Data,
				}

				// Update conversation trace
				if trace, ok := agentContext["conversation_trace"].([]string); ok {
					traceClone := make([]string, len(trace))
					copy(traceClone, trace)
					traceClone = append(traceClone, fmt.Sprintf("%s -> router", name))
					agentContext["conversation_trace"] = traceClone
				}

				// Get response data for context update
				if responseData, ok := response.Data.(map[string]interface{}); ok {
					// Send this agent's context update to the channel
					contextChan <- contextUpdate{
						agentName: name,
						data:      responseData,
					}
				}
			} else {
				errors[index] = fmt.Errorf("unexpected response type from agent %s", name)
			}
		}(i, agentName)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(contextChan)

	// Process all the context updates
	allContextUpdates := make(map[string]map[string]interface{})
	contributions := make(map[string]interface{})

	// Extract any existing contributions
	if existingContribs, ok := updatedContext["agent_contributions"].(map[string]interface{}); ok {
		for k, v := range existingContribs {
			contributions[k] = v
		}
	}

	// Collect context updates from all agents
	for update := range contextChan {
		allContextUpdates[update.agentName] = update.data

		// Record this agent's contribution
		contributions[update.agentName] = map[string]interface{}{
			"timestamp": time.Now().Unix(),
			"content":   results[getIndexForAgent(update.agentName, req.Agents)].Content,
		}
	}

	// Update the agent_contributions in the context
	updatedContext["agent_contributions"] = contributions

	// Merge all context updates
	for _, contextData := range allContextUpdates {
		for k, v := range contextData {
			// Don't overwrite conversation_trace or agent_contributions
			if k != "conversation_trace" && k != "agent_contributions" {
				updatedContext[k] = v
			}
		}
	}

	// Update conversation trace with all steps
	var allTraces []string
	if trace, ok := updatedContext["conversation_trace"].([]string); ok {
		allTraces = trace
	}

	// Collect all traces from agent context updates
	for agentName, contextData := range allContextUpdates {
		if trace, ok := contextData["conversation_trace"].([]string); ok {
			for _, step := range trace {
				// Only add steps that aren't already in the trace
				if !contains(allTraces, step) &&
					(strings.HasPrefix(step, "router -> "+agentName) ||
						strings.HasPrefix(step, agentName+" -> router")) {
					allTraces = append(allTraces, step)
				}
			}
		}
	}
	updatedContext["conversation_trace"] = allTraces

	// Update the request context
	req.Context = updatedContext

	// Check for errors
	for _, err := range errors {
		if err != nil {
			return results, err
		}
	}

	return results, nil
}

// getIndexForAgent returns the index of the agent name in the agents slice
func getIndexForAgent(agentName string, agents []string) int {
	for i, name := range agents {
		if name == agentName {
			return i
		}
	}
	return -1
}

// contains checks if a string is present in a slice
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// callAgentsSequentially calls multiple agents in sequence and collects their responses
func (r *RouterAgent) callAgentsSequentially(ctx actor.Context, req *TransmitRequest) ([]AgentResponse, error) {
	results := make([]AgentResponse, 0, len(req.Agents))

	// Create a copy of the context to pass between agents
	updatedContext := make(map[string]interface{})
	if req.Context != nil {
		for k, v := range req.Context {
			updatedContext[k] = v
		}
	} else {
		updatedContext = make(map[string]interface{})
	}

	// Add a conversation trace if not present
	if _, hasTrace := updatedContext["conversation_trace"]; !hasTrace {
		updatedContext["conversation_trace"] = []string{}
	}

	for _, agentName := range req.Agents {
		agent, err := r.network.GetAgent(agentName)
		if err != nil {
			return results, fmt.Errorf("agent %s not found", agentName)
		}

		// Update conversation trace
		if trace, ok := updatedContext["conversation_trace"].([]string); ok {
			trace = append(trace, fmt.Sprintf("router -> %s", agentName))
			updatedContext["conversation_trace"] = trace
		}

		// Create message for agent
		message := &NetworkMessage{
			From:    "router",
			To:      agentName,
			Content: req.Message,
			Data:    updatedContext,
		}

		// Send message to agent
		// Use the sequential timeout for sequential calls
		timeout := r.options.SequentialTimeout
		future := ctx.RequestFuture(agent, message, timeout)
		result, err := future.Result()
		if err != nil {
			return results, err
		}

		// Process response
		if response, ok := result.(*NetworkMessage); ok {
			// Create agent response
			agentResp := AgentResponse{
				Agent:   agentName,
				Content: response.Content,
				Data:    response.Data,
			}
			results = append(results, agentResp)

			// Log this agent's processing in the trace
			if trace, ok := updatedContext["conversation_trace"].([]string); ok {
				trace = append(trace, fmt.Sprintf("%s -> router", agentName))
				updatedContext["conversation_trace"] = trace
			}

			// Add this agent's contribution to the context history
			agentContributions, hasContributions := updatedContext["agent_contributions"].(map[string]interface{})
			if !hasContributions {
				agentContributions = make(map[string]interface{})
			}
			agentContributions[agentName] = map[string]interface{}{
				"timestamp": time.Now().Unix(),
				"content":   response.Content,
			}
			updatedContext["agent_contributions"] = agentContributions

			// Update context with agent's response data
			if response.Data != nil {
				contextData, isMap := response.Data.(map[string]interface{})
				if isMap {
					for k, v := range contextData {
						updatedContext[k] = v
					}
				}
			}
		} else {
			return results, fmt.Errorf("unexpected response type from agent %s", agentName)
		}
	}

	// Update the request context with all accumulated context
	req.Context = updatedContext

	return results, nil
}

// suggestNextAgents uses the model to suggest which agents might be relevant next
func (r *RouterAgent) suggestNextAgents(ctx actor.Context, req *TransmitRequest, results []AgentResponse) ([]string, error) {
	// Create a prompt for the model
	agentDescriptions := r.getAgentDescriptions()

	// Combine all results for context
	combinedResults := ""
	for _, result := range results {
		combinedResults += fmt.Sprintf("Agent %s response: %s\n", result.Agent, result.Content)
	}

	prompt := fmt.Sprintf(`Based on the user request and the responses from agents so far, suggest which agent(s) might be relevant to call next:
	
User Request: %s

Agent Responses:
%s

Available Agents:
%s

Context:
%v

Return only the name(s) of the agent(s) that should be called next in JSON format like {"agents": ["agent1", "agent2"]}`,
		req.Message,
		combinedResults,
		agentDescriptions,
		req.Context)

	// Call the model
	generateRequest := []models.Message{
		{
			Role:    "system",
			Content: "You are a router agent that determines which specialized agents should be called next in a workflow.",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}

	responseText, err := r.model.Generate(context.Background(), generateRequest, &models.GenerateOptions{
		Temperature: 0.2,
	})

	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	// Parse the response
	var result struct {
		Agents []string `json:"agents"`
	}

	err = json.Unmarshal([]byte(responseText), &result)
	if err != nil {
		// If JSON parsing fails, return empty list (non-critical error)
		return []string{}, nil
	}

	return result.Agents, nil
}

// getAgentDescriptions returns descriptions of all registered agents
func (r *RouterAgent) getAgentDescriptions() string {
	r.network.mu.RLock()
	defer r.network.mu.RUnlock()

	descriptions := ""
	for name := range r.network.agents {
		// TODO: Get actual descriptions from agent metadata
		descriptions += fmt.Sprintf("- %s\n", name)
	}

	return descriptions
}

// handleNetworkMessage processes network messages and determines routing
func (r *RouterAgent) handleNetworkMessage(ctx actor.Context, msg *NetworkMessage) {
	// If the message has a specific target, route directly
	if msg.To != "" {
		agent, err := r.network.GetAgent(msg.To)
		if err != nil {
			ctx.Respond(fmt.Errorf("target agent %s not found", msg.To))
			return
		}

		// Forward message to target agent
		// Use the default timeout for direct message routing
		timeout := r.options.DefaultTimeout
		future := ctx.RequestFuture(agent, msg, timeout)
		result, err := future.Result()
		if err != nil {
			ctx.Respond(err)
			return
		}
		ctx.Respond(result)
		return
	}

	// Create routing request for LLM-based routing
	req := &TransmitRequest{
		Message: msg.Content,
		Context: nil,
	}

	// Convert message data to context if possible
	if msg.Data != nil {
		contextData, isMap := msg.Data.(map[string]interface{})
		if isMap {
			req.Context = contextData
		}
	}

	// Handle as a transmit request
	r.handleTransmitRequest(ctx, req)
}

// GetAgentList returns a list of available agents
func (r *RouterAgent) GetAgentList() []string {
	r.network.mu.RLock()
	defer r.network.mu.RUnlock()

	agents := make([]string, 0, len(r.network.agents))
	for name := range r.network.agents {
		agents = append(agents, name)
	}
	return agents
}
