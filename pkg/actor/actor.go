package actor

import (
	"errors"
	"log"

	"github.com/asynkron/protoactor-go/actor"
)

// ActorSystem 是Gostra的核心Actor系统
type ActorSystem struct {
	context     actor.Context
	registry    *ActorRegistry
	config      *Configuration
	rootContext *actor.RootContext
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
	agents     map[string]*actor.PID
	tools      map[string]*actor.PID
	workflows  map[string]*actor.PID
	memory     *actor.PID
	deployment *actor.PID
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
		agents:    make(map[string]*actor.PID),
		tools:     make(map[string]*actor.PID),
		workflows: make(map[string]*actor.PID),
	}

	system := &ActorSystem{
		registry:    registry,
		config:      config,
		rootContext: actor.NewRootContext(actor.WithSenderMiddleware(LoggingMiddleware)),
	}

	return system
}

// LoggingMiddleware 是一个Actor中间件，记录所有发送的消息
func LoggingMiddleware(next actor.SenderFunc) actor.SenderFunc {
	return func(c actor.Context, target *actor.PID, envelope *actor.MessageEnvelope) {
		log.Printf("Sending message %T to %s", envelope.Message, target.String())
		next(c, target, envelope)
	}
}

// Start 启动Actor系统
func (s *ActorSystem) Start() error {
	log.Println("Starting Actor System...")
	// 此处可以初始化特定Actor，例如监督Actor
	return nil
}

// Stop 停止Actor系统
func (s *ActorSystem) Stop() error {
	log.Println("Stopping Actor System...")
	// 停止所有注册的Actor
	for _, pid := range s.registry.agents {
		s.rootContext.Stop(pid)
	}
	for _, pid := range s.registry.tools {
		s.rootContext.Stop(pid)
	}
	for _, pid := range s.registry.workflows {
		s.rootContext.Stop(pid)
	}
	if s.registry.memory != nil {
		s.rootContext.Stop(s.registry.memory)
	}
	if s.registry.deployment != nil {
		s.rootContext.Stop(s.registry.deployment)
	}
	return nil
}

// RegisterAgent 注册一个Agent Actor
func (s *ActorSystem) RegisterAgent(name string, props *actor.Props) (*actor.PID, error) {
	if _, exists := s.registry.agents[name]; exists {
		return nil, errors.New("agent already registered: " + name)
	}
	pid, err := s.rootContext.SpawnNamed(props, "agent-"+name)
	if err != nil {
		return nil, err
	}
	s.registry.agents[name] = pid
	return pid, nil
}

// GetAgent 获取已注册的Agent Actor
func (s *ActorSystem) GetAgent(name string) (*actor.PID, error) {
	if pid, exists := s.registry.agents[name]; exists {
		return pid, nil
	}
	return nil, errors.New("agent not found: " + name)
}

// RegisterTool 注册一个Tool Actor
func (s *ActorSystem) RegisterTool(name string, props *actor.Props) (*actor.PID, error) {
	if _, exists := s.registry.tools[name]; exists {
		return nil, errors.New("tool already registered: " + name)
	}
	pid, err := s.rootContext.SpawnNamed(props, "tool-"+name)
	if err != nil {
		return nil, err
	}
	s.registry.tools[name] = pid
	return pid, nil
}

// GetTool 获取已注册的Tool Actor
func (s *ActorSystem) GetTool(name string) (*actor.PID, error) {
	if pid, exists := s.registry.tools[name]; exists {
		return pid, nil
	}
	return nil, errors.New("tool not found: " + name)
}

// RegisterWorkflow 注册一个Workflow Actor
func (s *ActorSystem) RegisterWorkflow(name string, props *actor.Props) (*actor.PID, error) {
	if _, exists := s.registry.workflows[name]; exists {
		return nil, errors.New("workflow already registered: " + name)
	}
	pid, err := s.rootContext.SpawnNamed(props, "workflow-"+name)
	if err != nil {
		return nil, err
	}
	s.registry.workflows[name] = pid
	return pid, nil
}

// GetWorkflow 获取已注册的Workflow Actor
func (s *ActorSystem) GetWorkflow(name string) (*actor.PID, error) {
	if pid, exists := s.registry.workflows[name]; exists {
		return pid, nil
	}
	return nil, errors.New("workflow not found: " + name)
}

// SetMemory 设置Memory Actor
func (s *ActorSystem) SetMemory(props *actor.Props) (*actor.PID, error) {
	pid, err := s.rootContext.SpawnNamed(props, "memory")
	if err != nil {
		return nil, err
	}
	s.registry.memory = pid
	return pid, nil
}

// GetMemory 获取Memory Actor
func (s *ActorSystem) GetMemory() (*actor.PID, error) {
	if s.registry.memory == nil {
		return nil, errors.New("memory not set")
	}
	return s.registry.memory, nil
}

// GetRootContext 获取根上下文
func (s *ActorSystem) GetRootContext() *actor.RootContext {
	return s.rootContext
}
