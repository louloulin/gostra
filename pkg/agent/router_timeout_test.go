package agent

import (
	"context"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/louloulin/gostra/pkg/models"
	"github.com/stretchr/testify/assert"
)

// TimeoutTestModel is a basic mock for model provider
type TimeoutTestModel struct{}

func (m *TimeoutTestModel) GetID() string {
	return "test-model"
}

func (m *TimeoutTestModel) GetProvider() string {
	return "test-provider"
}

func (m *TimeoutTestModel) Generate(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (string, error) {
	return `{"agents": ["test-agent"]}`, nil
}

func (m *TimeoutTestModel) Stream(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (<-chan string, error) {
	ch := make(chan string, 1)
	ch <- `{"agents": ["test-agent"]}`
	close(ch)
	return ch, nil
}

func (m *TimeoutTestModel) GenerateWithFunctionCalls(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (*models.ResponseWithFunctionCalls, error) {
	return &models.ResponseWithFunctionCalls{
		Text:         `{"agents": ["test-agent"]}`,
		FinishReason: "stop",
	}, nil
}

func (m *TimeoutTestModel) StreamWithFunctionCalls(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (<-chan *models.ResponseChunk, error) {
	ch := make(chan *models.ResponseChunk, 1)
	ch <- &models.ResponseChunk{
		Text:         `{"agents": ["test-agent"]}`,
		IsFinished:   true,
		FinishReason: "stop",
	}
	close(ch)
	return ch, nil
}

// TimeoutTestAgent is a basic mock agent
type TimeoutTestAgent struct {
	received []*NetworkMessage
}

func (a *TimeoutTestAgent) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *NetworkMessage:
		a.received = append(a.received, msg)
		// Echo back the message
		response := &NetworkMessage{
			From:    msg.To,
			To:      msg.From,
			Content: "Response to: " + msg.Content,
		}
		ctx.Respond(response)
	}
}

func TestRouterTimeoutConfiguration(t *testing.T) {
	system := actor.NewActorSystem()
	model := &TimeoutTestModel{}

	t.Run("Default Timeouts", func(t *testing.T) {
		// Create network with default router options
		networkOpts := &AgentNetworkOptions{
			ID:          "test-network",
			Name:        "Test Network",
			ActorSystem: system,
			// RouterOptions left nil to use defaults
		}

		network, err := NewAgentNetwork(networkOpts, model)
		assert.NoError(t, err)
		assert.NotNil(t, network)
		assert.NotNil(t, network.routerPID)

		// Register a test agent
		agent1 := &TimeoutTestAgent{}
		agentProps := actor.PropsFromProducer(func() actor.Actor {
			return agent1
		})
		pid, err := system.Root.SpawnNamed(agentProps, "test-agent")
		assert.NoError(t, err)

		network.agents["test-agent"] = pid

		// Create a request that will be routed
		msg := &NetworkMessage{
			Content: "test message",
		}

		// Send to router
		future := system.Root.RequestFuture(network.routerPID, msg, 2*time.Second)
		result, err := future.Result()
		assert.NoError(t, err)
		assert.NotNil(t, result)

		// Verify message was received by agent
		assert.Len(t, agent1.received, 1)
	})

	t.Run("Custom Timeouts", func(t *testing.T) {
		// Create custom router options with different timeouts
		routerOpts := &RouterOptions{
			DefaultTimeout:    5 * time.Second,
			RoutingTimeout:    3 * time.Second,
			ParallelTimeout:   10 * time.Second,
			SequentialTimeout: 8 * time.Second,
		}

		// Create network with custom router options
		networkOpts := &AgentNetworkOptions{
			ID:            "test-network-custom",
			Name:          "Test Network Custom",
			ActorSystem:   system,
			RouterOptions: routerOpts,
		}

		network, err := NewAgentNetwork(networkOpts, model)
		assert.NoError(t, err)
		assert.NotNil(t, network)
		assert.NotNil(t, network.routerPID)

		// Register a test agent
		agent1 := &TimeoutTestAgent{}
		agentProps := actor.PropsFromProducer(func() actor.Actor {
			return agent1
		})
		pid, err := system.Root.SpawnNamed(agentProps, "test-agent-custom")
		assert.NoError(t, err)

		network.agents["test-agent-custom"] = pid

		// Create a request that will be routed
		msg := &NetworkMessage{
			Content: "test message with custom timeouts",
		}

		// Send to router
		future := system.Root.RequestFuture(network.routerPID, msg, 10*time.Second)
		result, err := future.Result()
		assert.NoError(t, err)
		assert.NotNil(t, result)

		// Verify message was received by agent
		assert.Len(t, agent1.received, 1)
	})
}
