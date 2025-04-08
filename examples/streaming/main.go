package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/yourusername/gostra/pkg/agent"
	"github.com/yourusername/gostra/pkg/memory"
	"github.com/yourusername/gostra/pkg/models/openai"
)

func main() {
	// 读取API密钥
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("请设置OPENAI_API_KEY环境变量")
	}

	// 创建OpenAI模型提供者
	modelProvider, err := openai.NewOpenAIProvider(&openai.Options{
		APIKey: apiKey,
		Model:  "gpt-3.5-turbo",
	})
	if err != nil {
		log.Fatalf("创建模型提供者失败: %v", err)
	}

	// 创建内存提供者
	memoryProvider := memory.NewInMemoryProvider()

	// 创建Agent
	myAgent, err := agent.NewAgent(&agent.Options{
		ID:             "streaming-agent",
		Name:           "Streaming Demo Agent",
		SystemPrompt:   "你是一个有用的助手，会详细解释概念。",
		ModelProvider:  modelProvider,
		MemoryProvider: memoryProvider,
	})
	if err != nil {
		log.Fatalf("创建Agent失败: %v", err)
	}

	// 创建可取消的上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 捕获信号以取消上下文
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChan
		fmt.Println("\n收到中断信号，取消流式响应...")
		cancel()
	}()

	// 创建消息
	messages := []agent.Message{
		{
			Role:    "user",
			Content: "请详细解释流式响应（Streaming Response）的技术优势，以及如何在网络应用中实现它。",
		},
	}

	// 设置流选项
	options := &agent.StreamOptions{
		MaxSteps:    1,
		Temperature: 0.7,
		AbortSignal: ctx,
	}

	fmt.Println("开始流式生成，按Ctrl+C可中断...")
	fmt.Println("------------------------------------")

	// 调用流式API
	streamResp, err := myAgent.Stream(messages, options)
	if err != nil {
		log.Fatalf("流式API调用失败: %v", err)
	}

	// 处理流式响应
	go func() {
		// 打印文本流
		for text := range streamResp.TextStream {
			fmt.Print(text)
		}
		fmt.Println("\n------------------------------------")
	}()

	// 等待消息完成
	var responseMessage agent.Message
	select {
	case msg := <-streamResp.MessageChan:
		responseMessage = msg
	case <-ctx.Done():
		fmt.Println("\n生成已取消")
		return
	}

	// 等待完成信息
	select {
	case finish := <-streamResp.FinishChan:
		fmt.Printf("\n完成原因: %s\n", finish.FinishReason)
		fmt.Printf("Token使用: 提示=%d, 完成=%d, 总计=%d\n",
			finish.Usage.PromptTokens,
			finish.Usage.CompletionTokens,
			finish.Usage.TotalTokens,
		)
	case <-ctx.Done():
		fmt.Println("\n生成已取消")
		return
	}

	// 保存对话到内存
	threadID := "example-thread"
	threadMetadata := map[string]interface{}{
		"name":        "流式响应示例",
		"description": "测试流式响应功能的演示线程",
		"created_by":  "streaming-example",
	}
	thread, err := memoryProvider.CreateThread(context.Background(), threadMetadata)
	if err != nil {
		log.Printf("创建对话线程失败: %v", err)
	} else {
		threadID = thread.ID
		log.Printf("创建线程成功，ID: %s", threadID)
	}

	// 保存用户消息
	_, err = memoryProvider.AddMessage(context.Background(), threadID, "user", messages[0].Content, nil)
	if err != nil {
		log.Printf("保存用户消息失败: %v", err)
	}

	// 保存助手回复
	_, err = memoryProvider.AddMessage(context.Background(), threadID, "assistant", responseMessage.Content, nil)
	if err != nil {
		log.Printf("保存助手回复失败: %v", err)
	}

	fmt.Println("流式生成示例完成，对话已保存")
}
