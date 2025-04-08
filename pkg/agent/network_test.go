package agent

import (
	"context"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
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

func (m *MockModel) Generate(ctx context.Context, messages []models.Message, opts *models.GenerateOptions) (*models.Response, error) {
	return &models.Response{
		Text: "agent1",
	}, nil
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
		CreatedAt: time.Now().Unix(),
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
		ThreadID:  threadID,
		Role:      role,
		Content:   content,
		CreatedAt: time.Now().Unix(),
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
		network := NewAgentNetwork(system, model)
		assert.NotNil(t, network)
		assert.NotNil(t, network.router)
		assert.NotNil(t, network.supervisor)
	})

	t.Run("Agent Registration", func(t *testing.T) {
		network := NewAgentNetwork(system, model)
		agent1 := &MockAgent{}

		err := network.RegisterAgent("agent1", agent1)
		assert.NoError(t, err)

		retrievedAgent, exists := network.GetAgent("agent1")
		assert.True(t, exists)
		assert.NotNil(t, retrievedAgent)
	})

	t.Run("Message Transmission", func(t *testing.T) {
		network := NewAgentNetwork(system, model)
		agent1 := &MockAgent{}
		agent2 := &MockAgent{}

		network.RegisterAgent("agent1", agent1)
		network.RegisterAgent("agent2", agent2)

		msg := &NetworkMessage{
			From:    "agent1",
			To:      "agent2",
			Content: "test message",
		}

		err := network.Transmit(context.Background(), msg)
		assert.NoError(t, err)

		// Allow time for message processing
		time.Sleep(100 * time.Millisecond)

		assert.Len(t, agent2.received, 1)
		assert.Equal(t, msg.Content, agent2.received[0].Content)
	})

	t.Run("Broadcast Message", func(t *testing.T) {
		network := NewAgentNetwork(system, model)
		agent1 := &MockAgent{}
		agent2 := &MockAgent{}
		agent3 := &MockAgent{}

		network.RegisterAgent("agent1", agent1)
		network.RegisterAgent("agent2", agent2)
		network.RegisterAgent("agent3", agent3)

		msg := &NetworkMessage{
			From:    "agent1",
			Content: "broadcast message",
		}

		targets := []string{"agent2", "agent3"}
		err := network.BroadcastMessage(context.Background(), msg, targets)
		assert.NoError(t, err)

		// Allow time for message processing
		time.Sleep(100 * time.Millisecond)

		assert.Len(t, agent2.received, 1)
		assert.Len(t, agent3.received, 1)
		assert.Equal(t, msg.Content, agent2.received[0].Content)
		assert.Equal(t, msg.Content, agent3.received[0].Content)
	})

	t.Run("Agent Removal", func(t *testing.T) {
		network := NewAgentNetwork(system, model)
		agent1 := &MockAgent{}

		network.RegisterAgent("agent1", agent1)
		network.RemoveAgent("agent1")

		_, exists := network.GetAgent("agent1")
		assert.False(t, exists)
	})

	t.Run("Error Handling", func(t *testing.T) {
		network := NewAgentNetwork(system, model)

		msg := &NetworkMessage{
			From:    "agent1",
			To:      "nonexistent",
			Content: "test message",
		}

		err := network.Transmit(context.Background(), msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Target agent not found")
	})
}

func TestRouterAgent(t *testing.T) {
	system := actor.NewActorSystem()
	model := &MockModel{}
	network := NewAgentNetwork(system, model)

	t.Run("Message Routing", func(t *testing.T) {
		agent1 := &MockAgent{}
		network.RegisterAgent("agent1", agent1)

		msg := &NetworkMessage{
			Content: "route this message",
		}

		props := actor.PropsFromProducer(func() actor.Actor {
			return NewRouterAgent(network)
		})

		routerPID := system.Root.Spawn(props)
		future := system.Root.RequestFuture(routerPID, msg, 1*time.Second)
		result, err := future.Result()

		assert.NoError(t, err)
		assert.Nil(t, result)

		// Allow time for message processing
		time.Sleep(100 * time.Millisecond)

		assert.Len(t, agent1.received, 1)
		assert.Equal(t, msg.Content, agent1.received[0].Content)
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
