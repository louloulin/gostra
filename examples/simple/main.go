package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourusername/gostra/pkg"
	"github.com/yourusername/gostra/pkg/agent"
	"github.com/yourusername/gostra/pkg/memory"
	"github.com/yourusername/gostra/pkg/models/openai"
	"github.com/yourusername/gostra/pkg/tools"
)

func main() {
	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 创建Gostra实例
	log.Println("初始化Gostra...")
	g := pkg.New(nil)

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
		func(ctx context.Context, params map[string]interface{}, options tools.ExecuteOptions) (interface{}, error) {
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

	// 注册Agent
	if err := g.RegisterAgent("simple-agent", agent); err != nil {
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
	response, err := agent.Run(ctx, &agent.RunOptions{
		ThreadID: thread.ID,
	})
	if err != nil {
		log.Fatalf("运行Agent失败: %v", err)
	}

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

func (s *EchoSchema) JSONSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"message": map[string]interface{}{
				"type":        "string",
				"description": "要回显的消息",
			},
		},
		"required": []string{"message"},
	}
}
