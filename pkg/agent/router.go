package agent

import (
	"context"
	"fmt"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/yourusername/gostra/pkg/models"
)

// RouterAgent handles dynamic message routing in the agent network
type RouterAgent struct {
	network *AgentNetwork
	model   models.ModelProvider
}

// NewRouterAgent creates a new router agent
func NewRouterAgent(network *AgentNetwork) *RouterAgent {
	return &RouterAgent{
		network: network,
		model:   network.model,
	}
}

// Receive handles incoming messages
func (r *RouterAgent) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *NetworkMessage:
		r.handleNetworkMessage(context, msg)
	case *actor.Started:
		// Initialize router
	case *actor.Stopping:
		// Cleanup
	}
}

// handleNetworkMessage processes network messages and determines routing
func (r *RouterAgent) handleNetworkMessage(ctx actor.Context, msg *NetworkMessage) {
	// Create routing prompt
	prompt := `Given the following message and available agents, determine the best agent(s) to handle this request:
Message: ${msg.Content}
Available Agents: ${r.network.GetAgentList()}

Please respond with the name of the agent that should handle this message.`

	// Get routing decision from model
	response, err := r.model.Generate(context.Background(), prompt, nil)
	if err != nil {
		ctx.Respond(err)
		return
	}

	// Parse response and route message
	targetAgent := response.Text
	if agent, exists := r.network.GetAgent(targetAgent); exists {
		// Forward message to target agent
		future := ctx.RequestFuture(agent.(*actor.PID), msg, timeout)
		result, err := future.Result()
		if err != nil {
			ctx.Respond(err)
			return
		}
		ctx.Respond(result)
	} else {
		ctx.Respond(fmt.Errorf("target agent %s not found", targetAgent))
	}
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
