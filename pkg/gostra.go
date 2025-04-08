package pkg

import (
	"context"
	"errors"
	"log"

	"github.com/yourusername/gostra/pkg/actor"
	"github.com/yourusername/gostra/pkg/agent"
	"github.com/yourusername/gostra/pkg/models"
	"github.com/yourusername/gostra/pkg/tools"
)

// Gostra 是框架的主入口类
type Gostra struct {
	actorSystem *actor.ActorSystem
	agents      map[string]*agent.Agent
	tools       map[string]tools.Tool
	models      map[string]models.ModelProvider
}

// Config 是Gostra配置选项
type Config struct {
	ActorSystemConfig *actor.Configuration
}

// New 创建一个新的Gostra实例
func New(config *Config) *Gostra {
	var actorConfig *actor.Configuration
	if config != nil {
		actorConfig = config.ActorSystemConfig
	}

	g := &Gostra{
		actorSystem: actor.NewActorSystem(actorConfig),
		agents:      make(map[string]*agent.Agent),
		tools:       make(map[string]tools.Tool),
		models:      make(map[string]models.ModelProvider),
	}

	return g
}

// Start 启动Gostra系统
func (g *Gostra) Start(ctx context.Context) error {
	log.Println("Starting Gostra...")

	// 启动Actor系统
	if err := g.actorSystem.Start(); err != nil {
		return err
	}

	return nil
}

// Stop 停止Gostra系统
func (g *Gostra) Stop() error {
	log.Println("Stopping Gostra...")

	// 停止Actor系统
	if err := g.actorSystem.Stop(); err != nil {
		return err
	}

	return nil
}

// RegisterAgent 注册一个Agent
func (g *Gostra) RegisterAgent(name string, a *agent.Agent) error {
	if a == nil {
		return errors.New("agent cannot be nil")
	}

	if _, exists := g.agents[name]; exists {
		return errors.New("agent already registered: " + name)
	}

	// 创建Agent Actor
	props := agent.NewAgentActor(a)

	// 注册到Actor系统
	pid, err := g.actorSystem.RegisterAgent(name, props)
	if err != nil {
		return err
	}

	// 存储Agent实例
	g.agents[name] = a

	log.Printf("Agent registered: %s (PID: %s)", name, pid.String())

	return nil
}

// GetAgent 获取已注册的Agent
func (g *Gostra) GetAgent(name string) (*agent.Agent, error) {
	if a, exists := g.agents[name]; exists {
		return a, nil
	}
	return nil, errors.New("agent not found: " + name)
}

// RegisterTool 注册一个工具
func (g *Gostra) RegisterTool(name string, t tools.Tool) error {
	if t == nil {
		return errors.New("tool cannot be nil")
	}

	if _, exists := g.tools[name]; exists {
		return errors.New("tool already registered: " + name)
	}

	// 存储工具实例
	g.tools[name] = t

	log.Printf("Tool registered: %s", name)

	return nil
}

// GetTool 获取已注册的工具
func (g *Gostra) GetTool(name string) (tools.Tool, error) {
	if t, exists := g.tools[name]; exists {
		return t, nil
	}
	return nil, errors.New("tool not found: " + name)
}

// RegisterModel 注册一个模型提供者
func (g *Gostra) RegisterModel(name string, m models.ModelProvider) error {
	if m == nil {
		return errors.New("model provider cannot be nil")
	}

	if _, exists := g.models[name]; exists {
		return errors.New("model provider already registered: " + name)
	}

	// 存储模型实例
	g.models[name] = m

	log.Printf("Model provider registered: %s", name)

	return nil
}

// GetModel 获取已注册的模型提供者
func (g *Gostra) GetModel(name string) (models.ModelProvider, error) {
	if m, exists := g.models[name]; exists {
		return m, nil
	}
	return nil, errors.New("model provider not found: " + name)
}
