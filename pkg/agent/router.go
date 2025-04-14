package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/louloulin/gostra/pkg/models"
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
	var finalContext map[string]interface{} // Variable to store the final context
	var results []AgentResponse
	var err error

	initialContext := req.Context
	if initialContext == nil {
		initialContext = make(map[string]interface{})
	}

	// If specific agents are provided, route to them
	if len(req.Agents) > 0 {
		if req.ParallelCall {
			results, finalContext, err = r.callAgentsInParallel(ctx, req)
			if err != nil {
				response.Error = err.Error()
				response.Context = initialContext // Return initial context on error
				ctx.Respond(&response)
				return
			}
			response.Results = results
		} else {
			// Call agents sequentially
			results, finalContext, err = r.callAgentsSequentially(ctx, req)
			if err != nil {
				response.Error = err.Error()
				response.Context = initialContext // Return initial context on error
				ctx.Respond(&response)
				return
			}
			response.Results = results
		}

		response.Context = finalContext // Assign the final merged context
		ctx.Respond(&response)
		return
	}

	// Use LLM to determine routing
	agents, err := r.determineRoutingWithLLM(ctx, req)
	if err != nil {
		response.Error = fmt.Sprintf("LLM routing error: %v", err)
		response.Context = initialContext // Return initial context on error
		ctx.Respond(&response)
		return
	}

	// Call the determined agents sequentially (default for LLM routing)
	req.Agents = agents
	results, finalContext, err = r.callAgentsSequentially(ctx, req)
	if err != nil {
		response.Error = err.Error()
		response.Context = initialContext // Return initial context on error
		ctx.Respond(&response)
		return
	}

	response.Results = results
	response.Context = finalContext // Assign the final merged context

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

// mergeMaps performs a deep merge of two maps. Keys in src will overwrite keys in dst.
// Nested maps are also merged recursively.
// Special handling for 'conversation_trace' to append slices.
func mergeMaps(dst, src map[string]interface{}) map[string]interface{} {
	if dst == nil {
		dst = make(map[string]interface{})
	}
	for key, srcVal := range src {
		if dstVal, ok := dst[key]; ok {
			// Special case for conversation_trace: append slices
			if key == "conversation_trace" {
				dstSlice, dstOk := dstVal.([]string)
				srcSlice, srcOk := srcVal.([]string)
				if dstOk && srcOk {
					// Append src to dst, could add duplicate check if needed
					dst[key] = append(dstSlice, srcSlice...)
					continue // Skip default handling
				}
			}

			srcMap, srcIsMap := srcVal.(map[string]interface{})
			dstMap, dstIsMap := dstVal.(map[string]interface{})
			if srcIsMap && dstIsMap {
				// Recursively merge nested maps
				dst[key] = mergeMaps(dstMap, srcMap)
			} else {
				// Overwrite if types don't match or not maps
				dst[key] = srcVal
			}
		} else {
			// Add new key
			dst[key] = srcVal
		}
	}
	return dst
}

// callAgentsInParallel calls multiple agents in parallel and collects their responses
// Returns results, the final merged context, and any error.
func (r *RouterAgent) callAgentsInParallel(ctx actor.Context, req *TransmitRequest) ([]AgentResponse, map[string]interface{}, error) {
	var wg sync.WaitGroup
	results := make([]AgentResponse, len(req.Agents))
	errors := make([]error, len(req.Agents))
	resultLock := sync.Mutex{}

	// Create a shared context map (thread-safe map for parallel updates)
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

	// Use a mutex for safe access to the shared updatedContext
	contextLock := sync.Mutex{}

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
				resultLock.Lock()
				results[index] = AgentResponse{
					Agent:   name,
					Content: response.Content,
					Data:    response.Data,
				}
				resultLock.Unlock()

				// Update conversation trace
				if trace, ok := agentContext["conversation_trace"].([]string); ok {
					traceClone := make([]string, len(trace))
					copy(traceClone, trace)
					traceClone = append(traceClone, fmt.Sprintf("%s -> router", name))
					agentContext["conversation_trace"] = traceClone
				}

				// Get response data for context update
				if responseData, ok := response.Data.(map[string]interface{}); ok {
					// Merge context safely using mutex
					contextLock.Lock()
					updatedContext = mergeMaps(updatedContext, responseData)
					contextLock.Unlock()
				}
			} else {
				errors[index] = fmt.Errorf("unexpected response type from agent %s", name)
			}
		}(i, agentName)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Consolidate errors
	var combinedError error
	for _, err := range errors {
		if err != nil {
			if combinedError == nil {
				combinedError = err
			} else {
				combinedError = fmt.Errorf("%v; %w", combinedError, err)
			}
		}
	}

	return results, updatedContext, combinedError // Return final context
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

// callAgentsSequentially calls agents one after another, passing context along
// Returns results, the final merged context, and any error.
func (r *RouterAgent) callAgentsSequentially(ctx actor.Context, req *TransmitRequest) ([]AgentResponse, map[string]interface{}, error) {
	results := make([]AgentResponse, 0, len(req.Agents))

	// Start with the initial context from the request
	currentContext := make(map[string]interface{})
	if req.Context != nil {
		for k, v := range req.Context {
			currentContext[k] = v
		}
	}

	if _, hasTrace := currentContext["conversation_trace"]; !hasTrace {
		currentContext["conversation_trace"] = []string{}
	}

	for _, agentName := range req.Agents {
		agent, err := r.network.GetAgent(agentName)
		if err != nil {
			return results, currentContext, fmt.Errorf("agent %s not found", agentName)
		}

		// Update conversation trace
		if trace, ok := currentContext["conversation_trace"].([]string); ok {
			trace = append(trace, fmt.Sprintf("router -> %s", agentName))
			currentContext["conversation_trace"] = trace
		}

		// Create message for agent
		message := &NetworkMessage{
			From:    "router",
			To:      agentName,
			Content: req.Message,
			Data:    currentContext,
		}

		// Send message to agent
		// Use the sequential timeout for sequential calls
		timeout := r.options.SequentialTimeout
		future := ctx.RequestFuture(agent, message, timeout)
		result, err := future.Result()
		if err != nil {
			return results, currentContext, err
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
			if trace, ok := currentContext["conversation_trace"].([]string); ok {
				trace = append(trace, fmt.Sprintf("%s -> router", agentName))
				currentContext["conversation_trace"] = trace
			}

			// Add this agent's contribution to the context history
			agentContributions, hasContributions := currentContext["agent_contributions"].(map[string]interface{})
			if !hasContributions {
				agentContributions = make(map[string]interface{})
			}
			agentContributions[agentName] = map[string]interface{}{
				"timestamp": time.Now().Unix(),
				"content":   response.Content,
			}
			currentContext["agent_contributions"] = agentContributions

			// Extract and merge context data from the agent's response
			if response.Data != nil {
				if responseMap, ok := response.Data.(map[string]interface{}); ok {
					// Deep merge the result context into the current context
					currentContext = mergeMaps(currentContext, responseMap)
				}
			}

		} else {
			return results, currentContext, fmt.Errorf("unexpected response type from agent %s: %T", agentName, result)
		}
	}

	return results, currentContext, nil // Return final context
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
