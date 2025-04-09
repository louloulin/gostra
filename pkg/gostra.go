package pkg

import (
	"context"
	"errors"
	"log"
	"sync"

	"github.com/google/uuid"
	"github.com/yourusername/gostra/pkg/actor"
	"github.com/yourusername/gostra/pkg/models"
	"github.com/yourusername/gostra/pkg/tools"
)

// Agent represents an AI agent that can process messages and generate responses
type Agent interface {
	// Generate processes a message and returns a response
	Generate(message string, options GenerateOptions) (*GenerateResponse, error)

	// Stream processes a message and streams the response
	Stream(message string, options StreamOptions) (*StreamResponse, error)

	// GetInfo returns information about the agent
	GetInfo() AgentInfo
}

// Message represents a message in a conversation
type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ToolCall represents a tool call in a message
type ToolCall struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

// GenerateOptions contains options for generating a response
type GenerateOptions struct {
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	Tools       []Tool    `json:"tools,omitempty"`
	History     []Message `json:"history,omitempty"`
}

// StreamOptions contains options for streaming a response
type StreamOptions struct {
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	Tools       []Tool    `json:"tools,omitempty"`
	History     []Message `json:"history,omitempty"`
}

// Tool represents a tool that an agent can use
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Schema      any    `json:"schema,omitempty"`
}

// GenerateResponse contains the response from generating a message
type GenerateResponse struct {
	Message Message `json:"message"`
	Usage   Usage   `json:"usage"`
}

// StreamResponse contains the response from streaming a message
type StreamResponse struct {
	Message Message `json:"message"`
	Usage   Usage   `json:"usage"`
}

// Usage contains token usage information
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// AgentInfo contains information about an agent
type AgentInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ModelName   string `json:"model_name,omitempty"`
}

// Gostra is the main entry point for the Gostra framework
type Gostra struct {
	actorSystem   *actor.ActorSystem
	agents        map[string]Agent
	agentsMu      sync.RWMutex
	tools         map[string]tools.Tool
	modelRegistry *models.ModelRegistry
	options       *Options
}

// Options contains configuration options for Gostra
type Options struct {
	DefaultModelProvider string
	MaxConcurrency       int
	Debug                bool
}

// DefaultOptions returns the default options for Gostra
func DefaultOptions() *Options {
	return &Options{
		DefaultModelProvider: "openai",
		MaxConcurrency:       10,
		Debug:                false,
	}
}

// Config is the configuration for Gostra
type Config struct {
	ActorSystemConfig *actor.Configuration
	Options           *Options
}

// NewGostra creates a new Gostra instance
func NewGostra(options *Options) *Gostra {
	if options == nil {
		options = DefaultOptions()
	}

	return &Gostra{
		agents:        make(map[string]Agent),
		options:       options,
		tools:         make(map[string]tools.Tool),
		modelRegistry: models.NewModelRegistry(),
	}
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

// RegisterAgent registers an agent with Gostra
func (g *Gostra) RegisterAgent(agent Agent) (string, error) {
	if agent == nil {
		return "", errors.New("agent cannot be nil")
	}

	g.agentsMu.Lock()
	defer g.agentsMu.Unlock()

	info := agent.GetInfo()
	if info.ID == "" {
		info.ID = uuid.New().String()
	}

	g.agents[info.ID] = agent
	return info.ID, nil
}

// GetAgent returns an agent by ID
func (g *Gostra) GetAgent(id string) (Agent, error) {
	g.agentsMu.RLock()
	defer g.agentsMu.RUnlock()

	agent, exists := g.agents[id]
	if !exists {
		return nil, errors.New("agent not found")
	}

	return agent, nil
}

// ListAgents returns a list of all registered agents
func (g *Gostra) ListAgents() []AgentInfo {
	g.agentsMu.RLock()
	defer g.agentsMu.RUnlock()

	agents := make([]AgentInfo, 0, len(g.agents))
	for _, agent := range g.agents {
		agents = append(agents, agent.GetInfo())
	}

	return agents
}

// RemoveAgent removes an agent by ID
func (g *Gostra) RemoveAgent(id string) error {
	g.agentsMu.Lock()
	defer g.agentsMu.Unlock()

	if _, exists := g.agents[id]; !exists {
		return errors.New("agent not found")
	}

	delete(g.agents, id)
	return nil
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
