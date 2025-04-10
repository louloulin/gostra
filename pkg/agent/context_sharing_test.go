package agent

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/google/uuid"
	"github.com/louloulin/gostra/pkg/models"
	"github.com/stretchr/testify/assert"
)

// TestContextSharingAgent is a specialized agent that adds data to the context
type TestContextSharingAgent struct {
	name     string
	data     map[string]interface{}
	calls    int
	seenData map[string]interface{}
}

func NewTestContextSharingAgent(name string, data map[string]interface{}) *TestContextSharingAgent {
	return &TestContextSharingAgent{
		name:     name,
		data:     data,
		calls:    0,
		seenData: make(map[string]interface{}),
	}
}

func (a *TestContextSharingAgent) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *NetworkMessage:
		a.calls++

		// Extract any data we care about from incoming context
		if contextData, ok := msg.Data.(map[string]interface{}); ok {
			for k, v := range contextData {
				a.seenData[k] = v
			}
		}

		// Create response with our data added to context
		contextData := make(map[string]interface{})
		if msg.Data != nil {
			if existingData, ok := msg.Data.(map[string]interface{}); ok {
				// Copy existing context
				for k, v := range existingData {
					contextData[k] = v
				}
			}
		}

		// Add our data to context with agent name prefix
		for k, v := range a.data {
			contextData[fmt.Sprintf("%s.%s", a.name, k)] = v
		}

		// Add call count
		contextData[fmt.Sprintf("%s.calls", a.name)] = a.calls

		// Create response
		response := &NetworkMessage{
			From:    a.name,
			To:      msg.From,
			Content: fmt.Sprintf("Response from %s (call #%d)", a.name, a.calls),
			Data:    contextData,
		}

		ctx.Respond(response)
	}
}

// MockModelProvider for testing
type MockModelProvider struct{}

func (m *MockModelProvider) Generate(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (string, error) {
	return "mock response", nil
}

func (m *MockModelProvider) GenerateWithFunctionCalls(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (*models.ResponseWithFunctionCalls, error) {
	return &models.ResponseWithFunctionCalls{
		Text: "mock response",
	}, nil
}

func (m *MockModelProvider) GetID() string {
	return "mock-model"
}

func (m *MockModelProvider) GetProvider() string {
	return "mock"
}

func (m *MockModelProvider) Stream(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (<-chan string, error) {
	ch := make(chan string, 1)
	go func() {
		ch <- "mock stream response"
		close(ch)
	}()
	return ch, nil
}

func (m *MockModelProvider) StreamWithFunctionCalls(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (<-chan *models.ResponseChunk, error) {
	ch := make(chan *models.ResponseChunk, 1)
	go func() {
		ch <- &models.ResponseChunk{
			Text:       "mock stream response",
			IsFinished: true,
		}
		close(ch)
	}()
	return ch, nil
}

// TestContextSharingBetweenAgents tests that context data is properly shared between agents
func TestContextSharingBetweenAgents(t *testing.T) {
	// Create actor system
	system := actor.NewActorSystem()

	// Create and configure the agent network
	opts := &AgentNetworkOptions{
		ID:          "test-context-sharing-" + uuid.New().String(),
		Name:        "Test Context Sharing Network",
		ActorSystem: system,
	}

	model := &MockModelProvider{}
	network, err := NewAgentNetwork(opts, model)
	assert.NoError(t, err)

	// Create specialized agents
	dataAgent := NewTestContextSharingAgent("data_agent", map[string]interface{}{
		"number": 42,
		"text":   "hello world",
		"flag":   true,
	})

	processorAgent := NewTestContextSharingAgent("processor_agent", map[string]interface{}{
		"processed": true,
		"timestamp": time.Now().Unix(),
	})

	// Register agents
	err = network.RegisterAgent("data_agent", dataAgent)
	assert.NoError(t, err)

	err = network.RegisterAgent("processor_agent", processorAgent)
	assert.NoError(t, err)

	// Create initial context
	initialContext := map[string]interface{}{
		"conversation_id": "test-conversation-" + uuid.New().String(),
		"user_query":      "test query",
	}

	// Test sequential agent calls with context sharing
	t.Run("Sequential context sharing", func(t *testing.T) {
		// Create request
		req := &TransmitRequest{
			Message:      "Process data sequentially",
			Agents:       []string{"data_agent", "processor_agent"},
			ParallelCall: false,
			Context:      initialContext,
		}

		// Send request
		resp, err := network.SendRequest(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)

		// Verify both agents were called
		assert.Len(t, resp.Results, 2)
		assert.Equal(t, "data_agent", resp.Results[0].Agent)
		assert.Equal(t, "processor_agent", resp.Results[1].Agent)

		// Verify context contains data from both agents
		assert.Equal(t, 42, resp.Context["data_agent.number"])
		assert.Equal(t, "hello world", resp.Context["data_agent.text"])
		assert.Equal(t, true, resp.Context["data_agent.flag"])
		assert.Equal(t, true, resp.Context["processor_agent.processed"])
		assert.Contains(t, resp.Context, "processor_agent.timestamp")

		// Verify context tracking
		assert.Contains(t, resp.Context, "conversation_trace")
		assert.Contains(t, resp.Context, "agent_contributions")

		// Verify processor agent saw data from data agent
		assert.Equal(t, 42, processorAgent.seenData["data_agent.number"])
		assert.Equal(t, "hello world", processorAgent.seenData["data_agent.text"])
	})

	// Test parallel agent calls with context merging
	t.Run("Parallel context sharing", func(t *testing.T) {
		// Reset agents' seen data
		dataAgent.seenData = make(map[string]interface{})
		processorAgent.seenData = make(map[string]interface{})

		// Create request with parallel execution
		req := &TransmitRequest{
			Message:      "Process data in parallel",
			Agents:       []string{"data_agent", "processor_agent"},
			ParallelCall: true,
			Context:      initialContext,
		}

		// Send request
		resp, err := network.SendRequest(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)

		// Verify both agents were called
		assert.Len(t, resp.Results, 2)

		// Verify context contains data from both agents despite parallel execution
		assert.Equal(t, 42, resp.Context["data_agent.number"])
		assert.Equal(t, "hello world", resp.Context["data_agent.text"])
		assert.Equal(t, true, resp.Context["data_agent.flag"])
		assert.Equal(t, true, resp.Context["processor_agent.processed"])
		assert.Contains(t, resp.Context, "processor_agent.timestamp")

		// With parallel execution, agents won't see each other's data during execution
		// but the router should merge all context data in the response
	})
}
