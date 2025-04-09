package actor

import (
	"errors"
	"log"
)

// ActorSystem 是Gostra的核心Actor系统
type ActorSystem struct {
	registry *ActorRegistry
	config   *Configuration
}

// PID 代表Actor的进程ID
type PID struct {
	ID   string
	Type string
}

// Configuration 包含Actor系统的配置
type Configuration struct {
	Agents     map[string]interface{}
	Tools      map[string]interface{}
	Memory     interface{}
	Workflows  map[string]interface{}
	Logger     interface{}
	Deployment interface{}
}

// ActorRegistry 管理所有已注册的Actors
type ActorRegistry struct {
	agents     map[string]*PID
	tools      map[string]*PID
	workflows  map[string]*PID
	memory     *PID
	deployment *PID
}

// NewActorSystem 创建并初始化一个新的Actor系统
func NewActorSystem(config *Configuration) *ActorSystem {
	if config == nil {
		config = &Configuration{
			Agents:    make(map[string]interface{}),
			Tools:     make(map[string]interface{}),
			Workflows: make(map[string]interface{}),
		}
	}

	registry := &ActorRegistry{
		agents:    make(map[string]*PID),
		tools:     make(map[string]*PID),
		workflows: make(map[string]*PID),
	}

	system := &ActorSystem{
		registry: registry,
		config:   config,
	}

	return system
}

// LogMessage 记录消息
func LogMessage(msg interface{}, target *PID) {
	log.Printf("Sending message %T to %s-%s", msg, target.Type, target.ID)
}

// Start 启动Actor系统
func (s *ActorSystem) Start() error {
	log.Println("Starting Actor System...")
	return nil
}

// Stop 停止Actor系统
func (s *ActorSystem) Stop() error {
	log.Println("Stopping Actor System...")
	return nil
}

// RegisterAgent 注册一个Agent Actor
func (s *ActorSystem) RegisterAgent(name string, props interface{}) (*PID, error) {
	if _, exists := s.registry.agents[name]; exists {
		return nil, errors.New("agent already registered: " + name)
	}
	pid := &PID{
		ID:   name,
		Type: "agent",
	}
	s.registry.agents[name] = pid
	return pid, nil
}

// GetAgent 获取已注册的Agent Actor
func (s *ActorSystem) GetAgent(name string) (*PID, error) {
	if pid, exists := s.registry.agents[name]; exists {
		return pid, nil
	}
	return nil, errors.New("agent not found: " + name)
}

// RegisterTool 注册一个Tool Actor
func (s *ActorSystem) RegisterTool(name string, props interface{}) (*PID, error) {
	if _, exists := s.registry.tools[name]; exists {
		return nil, errors.New("tool already registered: " + name)
	}
	pid := &PID{
		ID:   name,
		Type: "tool",
	}
	s.registry.tools[name] = pid
	return pid, nil
}

// GetTool 获取已注册的Tool Actor
func (s *ActorSystem) GetTool(name string) (*PID, error) {
	if pid, exists := s.registry.tools[name]; exists {
		return pid, nil
	}
	return nil, errors.New("tool not found: " + name)
}

// RegisterWorkflow 注册一个Workflow Actor
func (s *ActorSystem) RegisterWorkflow(name string, props interface{}) (*PID, error) {
	if _, exists := s.registry.workflows[name]; exists {
		return nil, errors.New("workflow already registered: " + name)
	}
	pid := &PID{
		ID:   name,
		Type: "workflow",
	}
	s.registry.workflows[name] = pid
	return pid, nil
}

// GetWorkflow 获取已注册的Workflow Actor
func (s *ActorSystem) GetWorkflow(name string) (*PID, error) {
	if pid, exists := s.registry.workflows[name]; exists {
		return pid, nil
	}
	return nil, errors.New("workflow not found: " + name)
}

// SetMemory 设置Memory Actor
func (s *ActorSystem) SetMemory(props interface{}) (*PID, error) {
	pid := &PID{
		ID:   "memory",
		Type: "system",
	}
	s.registry.memory = pid
	return pid, nil
}

// GetMemory 获取Memory Actor
func (s *ActorSystem) GetMemory() (*PID, error) {
	if s.registry.memory == nil {
		return nil, errors.New("memory not set")
	}
	return s.registry.memory, nil
}
