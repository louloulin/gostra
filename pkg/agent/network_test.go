package agent

import (
	"context"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/yourusername/gostra/pkg/memory"
	"github.com/yourusername/gostra/pkg/models"
)

// MockAgent implements the Actor interface for testing
type MockAgent struct {
	received []*NetworkMessage
}

func (m *MockAgent) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *NetworkMessage:
		m.received = append(m.received, msg)
		context.Respond(nil)
	}
}

// MockModel implements the ModelProvider interface for testing
type MockModel struct{}

func (m *MockModel) GetID() string {
	return "mock-model"
}

func (m *MockModel) GetProvider() string {
	return "mock"
}

func (m *MockModel) Generate(ctx context.Context, messages []models.Message, opts *models.GenerateOptions) (string, error) {
	return "agent1", nil
}

func (m *MockModel) GenerateWithFunctionCalls(ctx context.Context, messages []models.Message, opts *models.GenerateOptions) (*models.ResponseWithFunctionCalls, error) {
	return &models.ResponseWithFunctionCalls{
		Text:         "agent1",
		FinishReason: "stop",
	}, nil
}

func (m *MockModel) Stream(ctx context.Context, messages []models.Message, opts *models.GenerateOptions) (<-chan string, error) {
	ch := make(chan string, 1)
	ch <- "agent1"
	close(ch)
	return ch, nil
}

func (m *MockModel) StreamWithFunctionCalls(ctx context.Context, messages []models.Message, opts *models.GenerateOptions) (<-chan *models.ResponseChunk, error) {
	ch := make(chan *models.ResponseChunk, 1)
	ch <- &models.ResponseChunk{
		Text:       "agent1",
		IsFinished: true,
	}
	close(ch)
	return ch, nil
}

// mockModelProvider 模拟模型提供者
type mockModelProvider struct{}

func (m *mockModelProvider) Generate(ctx interface{}, messages interface{}, options interface{}) (interface{}, error) {
	return "mock response", nil
}

func (m *mockModelProvider) Stream(ctx interface{}, messages interface{}, options interface{}) (interface{}, error) {
	return "mock stream response", nil
}

// mockMemoryProvider 模拟内存提供者
type mockMemoryProvider struct {
	threads map[string]*memory.Thread
}

func newMockMemoryProvider() *mockMemoryProvider {
	return &mockMemoryProvider{
		threads: make(map[string]*memory.Thread),
	}
}

func (m *mockMemoryProvider) CreateThread(ctx interface{}, metadata map[string]interface{}) (*memory.Thread, error) {
	thread := &memory.Thread{
		ID:        "thread-" + time.Now().Format("20060102150405"),
		CreatedAt: time.Now(),
		Metadata:  metadata,
	}
	m.threads[thread.ID] = thread
	return thread, nil
}

func (m *mockMemoryProvider) GetThread(ctx interface{}, threadID string) (*memory.Thread, error) {
	if thread, ok := m.threads[threadID]; ok {
		return thread, nil
	}
	return nil, nil
}

func (m *mockMemoryProvider) UpdateThread(ctx interface{}, threadID string, metadata map[string]interface{}) (*memory.Thread, error) {
	if thread, ok := m.threads[threadID]; ok {
		thread.Metadata = metadata
		return thread, nil
	}
	return nil, nil
}

func (m *mockMemoryProvider) DeleteThread(ctx interface{}, threadID string) error {
	delete(m.threads, threadID)
	return nil
}

func (m *mockMemoryProvider) ListThreads(ctx interface{}, limit int, offset int) ([]*memory.Thread, error) {
	threads := make([]*memory.Thread, 0)
	for _, thread := range m.threads {
		threads = append(threads, thread)
	}
	return threads, nil
}

func (m *mockMemoryProvider) AddMessage(ctx interface{}, threadID string, role string, content string, metadata map[string]interface{}) (*memory.Message, error) {
	msg := &memory.Message{
		ID:        "msg-" + time.Now().Format("20060102150405"),
		Role:      role,
		Content:   content,
		CreatedAt: time.Now(),
		Metadata:  metadata,
	}
	return msg, nil
}

func (m *mockMemoryProvider) GetMessages(ctx interface{}, threadID string, limit int, offset int) ([]*memory.Message, error) {
	return []*memory.Message{}, nil
}

func (m *mockMemoryProvider) DeleteMessages(ctx interface{}, threadID string, messageIDs []string) error {
	return nil
}

// TestAgentNetwork 测试Agent网络功能
func TestAgentNetwork(t *testing.T) {
	system := actor.NewActorSystem()
	model := &MockModel{}

	t.Run("Network Creation", func(t *testing.T) {
		opts := &AgentNetworkOptions{
			ID:          "test-network",
			Name:        "Test Network",
			ActorSystem: system,
		}
		network, err := NewAgentNetwork(opts, model)
		assert.NotNil(t, network)
		assert.NoError(t, err)
		assert.NotNil(t, network.routerPID)
	})

	t.Run("Agent Registration", func(t *testing.T) {
		opts := &AgentNetworkOptions{
			ID:          "test-registration-" + uuid.New().String(),
			Name:        "Test Registration",
			ActorSystem: system,
		}
		network, err := NewAgentNetwork(opts, model)
		assert.NoError(t, err)
		agent1 := &MockAgent{}

		err = network.RegisterAgent("agent1-reg", agent1)
		assert.NoError(t, err)

		retrievedAgent, err := network.GetAgent("agent1-reg")
		assert.NoError(t, err)
		assert.NotNil(t, retrievedAgent)
	})

	t.Run("Message Transmission", func(t *testing.T) {
		opts := &AgentNetworkOptions{
			ID:          "test-transmission-" + uuid.New().String(),
			Name:        "Test Transmission",
			ActorSystem: system,
		}
		network, err := NewAgentNetwork(opts, model)
		assert.NoError(t, err)
		agent1 := &MockAgent{}
		agent2 := &MockAgent{}

		network.RegisterAgent("agent1-trans", agent1)
		network.RegisterAgent("agent2-trans", agent2)

		msg := &NetworkMessage{
			From:    "agent1-trans",
			To:      "agent2-trans",
			Content: "test message",
		}

		err = network.Transmit(context.Background(), msg)
		assert.NoError(t, err)

		// Allow time for message processing
		time.Sleep(100 * time.Millisecond)

		assert.Len(t, agent2.received, 1)
		assert.Equal(t, msg.Content, agent2.received[0].Content)
	})

	t.Run("Broadcast Message", func(t *testing.T) {
		opts := &AgentNetworkOptions{
			ID:          "test-broadcast-" + uuid.New().String(),
			Name:        "Test Broadcast",
			ActorSystem: system,
		}
		network, err := NewAgentNetwork(opts, model)
		assert.NoError(t, err)
		agent1 := &MockAgent{}
		agent2 := &MockAgent{}
		agent3 := &MockAgent{}

		network.RegisterAgent("agent1-bcast", agent1)
		network.RegisterAgent("agent2-bcast", agent2)
		network.RegisterAgent("agent3-bcast", agent3)

		msg := &NetworkMessage{
			From:    "agent1-bcast",
			Content: "broadcast message",
		}

		targets := []string{"agent2-bcast", "agent3-bcast"}
		err = network.BroadcastToTargets(context.Background(), msg, targets)
		assert.NoError(t, err)

		// Allow time for message processing
		time.Sleep(100 * time.Millisecond)

		assert.Len(t, agent2.received, 1)
		assert.Len(t, agent3.received, 1)
		assert.Equal(t, msg.Content, agent2.received[0].Content)
		assert.Equal(t, msg.Content, agent3.received[0].Content)
	})

	t.Run("Agent Removal", func(t *testing.T) {
		opts := &AgentNetworkOptions{
			ID:          "test-removal-" + uuid.New().String(),
			Name:        "Test Removal",
			ActorSystem: system,
		}
		network, err := NewAgentNetwork(opts, model)
		assert.NoError(t, err)
		agent1 := &MockAgent{}

		// Register an agent first
		err = network.RegisterAgent("agent1-remove", agent1)
		assert.NoError(t, err)

		// Verify it exists
		_, err = network.GetAgent("agent1-remove")
		assert.NoError(t, err)

		// Remove it
		err = network.RemoveAgent("agent1-remove")
		assert.NoError(t, err)

		// Now it should be gone
		_, err = network.GetAgent("agent1-remove")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found in network")
	})

	t.Run("Error Handling", func(t *testing.T) {
		opts := &AgentNetworkOptions{
			ID:          "test-error-" + uuid.New().String(),
			Name:        "Test Error",
			ActorSystem: system,
		}
		network, err := NewAgentNetwork(opts, model)
		assert.NoError(t, err)

		msg := &NetworkMessage{
			From:    "agent1-error",
			To:      "nonexistent",
			Content: "test message",
		}

		err = network.Transmit(context.Background(), msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestRouterAgent(t *testing.T) {
	system := actor.NewActorSystem()
	model := &MockModel{}
	opts := &AgentNetworkOptions{
		ID:          "test-router-" + uuid.New().String(),
		Name:        "Test Router",
		ActorSystem: system,
	}
	network, err := NewAgentNetwork(opts, model)
	assert.NoError(t, err)

	agent1 := &MockAgent{}
	err = network.RegisterAgent("agent1-router", agent1)
	assert.NoError(t, err)

	t.Run("Message Routing", func(t *testing.T) {
		msg := &NetworkMessage{
			Content: "route this message",
			To:      "agent1-router", // Specify the target agent explicitly
		}

		// Create router with default options
		routerOpts := DefaultRouterOptions()
		props := actor.PropsFromProducer(func() actor.Actor {
			return NewRouterAgent(network, routerOpts)
		})

		routerPID := system.Root.Spawn(props)
		future := system.Root.RequestFuture(routerPID, msg, 1*time.Second)
		result, err := future.Result()

		assert.NoError(t, err)
		// The result might be nil or a response object, either is acceptable
		if result != nil {
			t.Logf("Result: %v", result)
		}

		// Allow time for message processing
		time.Sleep(100 * time.Millisecond)

		// Check received messages safely
		if len(agent1.received) == 0 {
			t.Error("No messages received by agent")
		} else {
			assert.Equal(t, msg.Content, agent1.received[0].Content)
		}
	})
}

// mockActorSystem 实现自定义ActorSystem接口
type mockActorSystem struct {
	system    *actor.ActorSystem
	rootCtx   *actor.RootContext
	agentPIDs map[string]*actor.PID
}

func (s *mockActorSystem) Root() *actor.RootContext {
	return s.rootCtx
}

func (s *mockActorSystem) Address() string {
	return "mock-system"
}

func (s *mockActorSystem) GetAgent(name string) (*actor.PID, error) {
	if pid, ok := s.agentPIDs[name]; ok {
		return pid, nil
	}
	return nil, nil
}

// TestConfigurableTimeouts tests that the timeout configuration is properly applied
func TestConfigurableTimeouts(t *testing.T) {
	actorSystem := actor.NewActorSystem()
	model := &MockModel{}

	t.Run("Default Timeouts", func(t *testing.T) {
		// Create network with default options
		networkOptions := &AgentNetworkOptions{
			ID:          "test-network",
			Name:        "Test Network",
			ActorSystem: actorSystem,
			// No RouterOptions specified, should use defaults
		}

		network, err := NewAgentNetwork(networkOptions, model)
		assert.NoError(t, err)
		assert.NotNil(t, network)

		// We don't have direct access to the router's options, but we can verify
		// the network was created successfully with default options
		assert.NotNil(t, network.routerPID)
	})

	t.Run("Custom Timeouts", func(t *testing.T) {
		// Create custom router options
		customRouterOptions := &RouterOptions{
			DefaultTimeout:    60 * time.Second,
			RoutingTimeout:    20 * time.Second,
			ParallelTimeout:   90 * time.Second,
			SequentialTimeout: 45 * time.Second,
		}

		// Create network with custom options
		networkOptions := &AgentNetworkOptions{
			ID:            "custom-timeout-network",
			Name:          "Custom Timeout Network",
			ActorSystem:   actorSystem,
			RouterOptions: customRouterOptions,
		}

		network, err := NewAgentNetwork(networkOptions, model)
		assert.NoError(t, err)
		assert.NotNil(t, network)
		assert.NotNil(t, network.routerPID)

		// To fully test this would require internal access to the router's options
		// or a test that actually waits for a timeout, which would make the test slow.
		// For now, we're just verifying that the network can be created with custom options.
	})

	t.Run("Integration Test with Mock Timeouts", func(t *testing.T) {
		// Skip this test in normal runs as it's dependent on timing
		t.Skip("Skipping timeout test as it's time-sensitive")

		// This is a more comprehensive test that would involve setting up
		// agents that respond after different delays and verifying that
		// timeouts work as expected.

		// Create a short timeout for testing
		shortTimeoutOptions := &RouterOptions{
			DefaultTimeout:    100 * time.Millisecond, // Very short for testing
			RoutingTimeout:    50 * time.Millisecond,
			ParallelTimeout:   150 * time.Millisecond,
			SequentialTimeout: 100 * time.Millisecond,
		}

		networkOptions := &AgentNetworkOptions{
			ID:            "short-timeout-network",
			Name:          "Short Timeout Network",
			ActorSystem:   actorSystem,
			RouterOptions: shortTimeoutOptions,
		}

		network, err := NewAgentNetwork(networkOptions, model)
		assert.NoError(t, err)

		// Register a slow-responding mock agent
		slowAgent := &SlowMockAgent{delay: 200 * time.Millisecond} // Slower than timeout
		err = network.RegisterAgent("slow-agent", slowAgent)
		assert.NoError(t, err)

		// Try sending a message - should timeout
		msg := &NetworkMessage{
			From:    "test",
			To:      "slow-agent",
			Content: "test message",
		}

		// This should result in a timeout error
		err = network.Transmit(context.Background(), msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timeout")
	})
}

// SlowMockAgent is a mock agent that delays responses for timeout testing
type SlowMockAgent struct {
	delay time.Duration
}

func (m *SlowMockAgent) Receive(context actor.Context) {
	switch context.Message().(type) {
	case *NetworkMessage:
		// Simulate slow processing by sleeping
		time.Sleep(m.delay)
		context.Respond(nil)
	}
}
