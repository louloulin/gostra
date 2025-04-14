package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/louloulin/gostra/pkg"
	"github.com/louloulin/gostra/pkg/agent"
	"github.com/louloulin/gostra/pkg/memory"
	"github.com/louloulin/gostra/pkg/models/openai"
	"github.com/louloulin/gostra/pkg/tools"
)

// RunOptions is a local implementation of the agent's run options
type RunOptions struct {
	ThreadID            string
	Input               string
	AvailableTools      []tools.Tool
	MaxConsecutiveCalls int
	MaxTokens           int
}

// Convert to agent.RunOptions
func (r *RunOptions) toAgentRunOptions() interface{} {
	// This is basically a cast to agent.RunOptions
	return &struct {
		ThreadID            string
		Input               string
		AvailableTools      []tools.Tool
		MaxConsecutiveCalls int
		MaxTokens           int
	}{
		ThreadID:            r.ThreadID,
		Input:               r.Input,
		AvailableTools:      r.AvailableTools,
		MaxConsecutiveCalls: r.MaxConsecutiveCalls,
		MaxTokens:           r.MaxTokens,
	}
}

// AgentAdapter adapts agent.Agent to pkg.Agent interface
type AgentAdapter struct {
	agent *agent.Agent
}

// Generate adapts the agent.Agent.Generate method to pkg.Agent interface
func (a *AgentAdapter) Generate(message string, options pkg.GenerateOptions) (*pkg.GenerateResponse, error) {
	// Convert message to agent.Message format
	agentMessages := []agent.Message{
		{
			Role:    "user",
			Content: message,
		},
	}

	// Convert options
	agentOptions := &agent.GenerateOptions{
		MaxSteps:    options.MaxTokens,
		Temperature: options.Temperature,
	}

	// Call the underlying agent
	res, err := a.agent.Generate(agentMessages, agentOptions)
	if err != nil {
		return nil, err
	}

	// Convert response to pkg.GenerateResponse
	return &pkg.GenerateResponse{
		Message: pkg.Message{
			Role:    res.Messages[0].Role,
			Content: res.Messages[0].Content,
		},
		Usage: pkg.Usage{
			PromptTokens:     res.FinishInfo.Usage.PromptTokens,
			CompletionTokens: res.FinishInfo.Usage.CompletionTokens,
			TotalTokens:      res.FinishInfo.Usage.TotalTokens,
		},
	}, nil
}

// Stream adapts the agent.Agent.Stream method to pkg.Agent interface
func (a *AgentAdapter) Stream(message string, options pkg.StreamOptions) (*pkg.StreamResponse, error) {
	// Convert message to agent.Message format
	agentMessages := []agent.Message{
		{
			Role:    "user",
			Content: message,
		},
	}

	// Convert options
	agentOptions := &agent.StreamOptions{
		MaxSteps:    options.MaxTokens,
		Temperature: options.Temperature,
		AbortSignal: context.Background(),
	}

	// This is a simplified implementation
	// In a real implementation, you would handle streaming properly
	_, err := a.agent.Stream(agentMessages, agentOptions)
	if err != nil {
		return nil, err
	}

	// Return a mock response
	return &pkg.StreamResponse{
		Message: pkg.Message{
			Role:    "assistant",
			Content: "Stream response",
		},
		Usage: pkg.Usage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	}, nil
}

// GetInfo returns information about the agent
func (a *AgentAdapter) GetInfo() pkg.AgentInfo {
	return pkg.AgentInfo{
		ID:          a.agent.ID,
		Name:        a.agent.ID,
		Description: "Agent adapter",
		ModelName:   "unknown",
	}
}

func main() {
	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 创建Gostra实例
	log.Println("初始化Gostra...")
	g := pkg.NewGostra(pkg.DefaultOptions())

	// 启动Gostra
	if err := g.Start(ctx); err != nil {
		log.Fatalf("启动Gostra失败: %v", err)
	}
	defer g.Stop()

	// 注册内存提供者
	memoryProvider := memory.NewInMemoryProvider()

	// 注册一个简单工具
	echoTool := tools.NewBasicTool(
		"echo",
		"回显输入的消息",
		&EchoSchema{},
		func(params map[string]interface{}, options *tools.ExecuteOptions) (interface{}, error) {
			message, _ := params["message"].(string)
			return map[string]interface{}{
				"message": message,
				"time":    time.Now().Format(time.RFC3339),
			}, nil
		},
	)

	if err := g.RegisterTool("echo", echoTool); err != nil {
		log.Fatalf("注册工具失败: %v", err)
	}

	// 注册OpenAI模型提供者
	openaiKey := os.Getenv("OPENAI_API_KEY")
	if openaiKey == "" {
		log.Println("警告: 未设置OPENAI_API_KEY环境变量")
		openaiKey = "your-api-key-here" // 示例值，实际使用时需要替换
	}

	openaiProvider, err := openai.NewOpenAIProvider(&openai.Options{
		APIKey:  openaiKey,
		BaseURL: "",
		Model:   "gpt-3.5-turbo",
	})

	if err != nil {
		log.Fatalf("创建OpenAI提供者失败: %v", err)
	}

	if err := g.RegisterModel("openai", openaiProvider); err != nil {
		log.Fatalf("注册模型提供者失败: %v", err)
	}

	// 创建一个Agent
	agent, err := agent.NewAgent(&agent.Options{
		ID:             "simple-agent",
		ModelProvider:  openaiProvider,
		MemoryProvider: memoryProvider,
		SystemPrompt:   "你是一个助手，可以回答问题并执行简单的任务。",
	})

	if err != nil {
		log.Fatalf("创建Agent失败: %v", err)
	}

	// 创建适配器并注册Agent
	adapter := &AgentAdapter{agent: agent}
	if _, err := g.RegisterAgent(adapter); err != nil {
		log.Fatalf("注册Agent失败: %v", err)
	}

	// 创建一个内存线程
	thread, err := memoryProvider.CreateThread(ctx, map[string]interface{}{
		"name": "示例对话",
	})
	if err != nil {
		log.Fatalf("创建内存线程失败: %v", err)
	}

	log.Printf("创建内存线程成功，ID: %s", thread.ID)

	// 添加一条用户消息
	_, err = memoryProvider.AddMessage(ctx, thread.ID, "user", "你好，请介绍一下自己，然后使用echo工具回应我的问候。", nil)
	if err != nil {
		log.Fatalf("添加消息失败: %v", err)
	}

	// 运行Agent
	log.Println("运行Agent...")
	// Simulate a response instead of calling agent.Run
	response := "这是一个模拟的回应。在实际的系统中，我会使用Echo工具回应你的问候。"
	log.Printf("Agent响应: %s", response)

	// 获取线程中的所有消息
	messages, err := memoryProvider.GetMessages(ctx, thread.ID, 10, 0)
	if err != nil {
		log.Fatalf("获取消息失败: %v", err)
	}

	log.Println("对话历史:")
	for _, msg := range messages {
		log.Printf("[%s] %s: %s", msg.CreatedAt.Format(time.RFC3339), msg.Role, msg.Content)
	}

	// 等待中断信号以优雅地关闭服务器
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("收到关闭信号，程序退出...")
}

// EchoSchema 是echo工具的参数schema
type EchoSchema struct{}

func (s *EchoSchema) Validate(params map[string]interface{}) error {
	if _, ok := params["message"]; !ok {
		return tools.NewValidationError("message 参数是必须的")
	}
	return nil
}

func (s *EchoSchema) JSONSchema() (map[string]interface{}, error) {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"message": map[string]interface{}{
				"type":        "string",
				"description": "要回显的消息",
			},
		},
		"required": []string{"message"},
	}, nil
}
