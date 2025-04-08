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
	actorSystem   *actor.ActorSystem
	agents        map[string]*agent.Agent
	tools         map[string]tools.Tool
	modelRegistry *models.ModelRegistry
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
		actorSystem:   actor.NewActorSystem(actorConfig),
		agents:        make(map[string]*agent.Agent),
		tools:         make(map[string]tools.Tool),
		modelRegistry: models.NewModelRegistry(),
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

// RegisterTextModel 注册一个文本模型提供者
func (g *Gostra) RegisterTextModel(name string, provider models.ModelProvider) error {
	if err := g.modelRegistry.RegisterTextModel(name, provider); err != nil {
		return err
	}

	log.Printf("Text model provider registered: %s", name)
	return nil
}

// GetTextModel 获取已注册的文本模型提供者
func (g *Gostra) GetTextModel(name string) (models.ModelProvider, error) {
	return g.modelRegistry.GetTextModel(name)
}

// RegisterImageModel 注册一个图像模型提供者
func (g *Gostra) RegisterImageModel(name string, provider models.ImageProvider) error {
	if err := g.modelRegistry.RegisterImageModel(name, provider); err != nil {
		return err
	}

	log.Printf("Image model provider registered: %s", name)
	return nil
}

// GetImageModel 获取已注册的图像模型提供者
func (g *Gostra) GetImageModel(name string) (models.ImageProvider, error) {
	return g.modelRegistry.GetImageModel(name)
}

// RegisterVoiceModel 注册一个语音模型提供者
func (g *Gostra) RegisterVoiceModel(name string, provider models.VoiceProvider) error {
	if err := g.modelRegistry.RegisterVoiceModel(name, provider); err != nil {
		return err
	}

	log.Printf("Voice model provider registered: %s", name)
	return nil
}

// GetVoiceModel 获取已注册的语音模型提供者
func (g *Gostra) GetVoiceModel(name string) (models.VoiceProvider, error) {
	return g.modelRegistry.GetVoiceModel(name)
}

// ListTextModels 列出所有已注册的文本模型
func (g *Gostra) ListTextModels() []string {
	return g.modelRegistry.ListTextModels()
}

// ListImageModels 列出所有已注册的图像模型
func (g *Gostra) ListImageModels() []string {
	return g.modelRegistry.ListImageModels()
}

// ListVoiceModels 列出所有已注册的语音模型
func (g *Gostra) ListVoiceModels() []string {
	return g.modelRegistry.ListVoiceModels()
}

// RegisterModel 注册一个文本模型提供者（兼容旧的API）
func (g *Gostra) RegisterModel(name string, m models.ModelProvider) error {
	return g.RegisterTextModel(name, m)
}

// GetModel 获取已注册的文本模型提供者（兼容旧的API）
func (g *Gostra) GetModel(name string) (models.ModelProvider, error) {
	return g.GetTextModel(name)
}
