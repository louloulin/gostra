package agent

import (
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
	"github.com/yourusername/gostra/pkg/memory"
)

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
	// 创建Actor系统
	system := actor.NewActorSystem()

	// 创建模拟的模型和内存提供者
	modelProvider := &mockModelProvider{}
	memoryProvider := newMockMemoryProvider()

	// 创建两个Agent
	agent1, err := NewAgent(&Options{
		ID:             "agent1",
		Name:           "Agent One",
		SystemPrompt:   "You are Agent One",
		ModelProvider:  modelProvider,
		MemoryProvider: memoryProvider,
	})
	assert.NoError(t, err)

	agent2, err := NewAgent(&Options{
		ID:             "agent2",
		Name:           "Agent Two",
		SystemPrompt:   "You are Agent Two",
		ModelProvider:  modelProvider,
		MemoryProvider: memoryProvider,
	})
	assert.NoError(t, err)

	// 创建Agent Actor
	props1, err := NewActorAgent(&ActorAgentOptions{
		ID:             agent1.ID,
		Name:           agent1.Name,
		SystemPrompt:   agent1.SystemPrompt,
		ModelProvider:  modelProvider,
		MemoryProvider: memoryProvider,
		ActorSystem:    system,
	})
	assert.NoError(t, err)

	props2, err := NewActorAgent(&ActorAgentOptions{
		ID:             agent2.ID,
		Name:           agent2.Name,
		SystemPrompt:   agent2.SystemPrompt,
		ModelProvider:  modelProvider,
		MemoryProvider: memoryProvider,
		ActorSystem:    system,
	})
	assert.NoError(t, err)

	// 启动Agent Actor
	rootContext := actor.NewRootContext(system, nil)
	pid1, err := rootContext.SpawnNamed(props1, "agent-agent1")
	assert.NoError(t, err)

	pid2, err := rootContext.SpawnNamed(props2, "agent-agent2")
	assert.NoError(t, err)

	// 等待Actor启动
	time.Sleep(100 * time.Millisecond)

	// 创建自定义的ActorSystem包装
	customSystem := &mockActorSystem{
		system:    system,
		rootCtx:   rootContext,
		agentPIDs: make(map[string]*actor.PID),
	}

	// 注册Agent
	customSystem.agentPIDs["agent1"] = pid1
	customSystem.agentPIDs["agent2"] = pid2

	// 创建网络
	network, err := NewAgentNetwork(&AgentNetworkOptions{
		ID:          "network1",
		Name:        "Test Network",
		Description: "Network for testing",
		AgentIDs:    []string{"agent1", "agent2"},
		ActorSystem: customSystem,
	})
	assert.NoError(t, err)

	// 启动网络
	err = network.Start()
	assert.NoError(t, err)

	// 连接两个Agent
	err = network.ConnectAgents("agent1", "agent2")
	assert.NoError(t, err)

	// 验证连接
	connections, err := network.GetConnections("agent1")
	assert.NoError(t, err)
	assert.Contains(t, connections, "agent2")

	// 发送消息
	err = network.SendMessage("agent1", "agent2", "Hello from Agent 1")
	assert.NoError(t, err)

	// 等待消息处理
	time.Sleep(100 * time.Millisecond)

	// 断开连接
	err = network.DisconnectAgents("agent1", "agent2")
	assert.NoError(t, err)

	// 验证连接已断开
	connections, err = network.GetConnections("agent1")
	assert.NoError(t, err)
	assert.NotContains(t, connections, "agent2")

	// 停止网络
	err = network.Stop()
	assert.NoError(t, err)

	// 停止Agent
	rootContext.Stop(pid1)
	rootContext.Stop(pid2)

	// 等待Actor停止
	time.Sleep(100 * time.Millisecond)
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
