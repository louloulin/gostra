package agent

import (
	"context"
	"testing"
	"time"

	"github.com/yourusername/gostra/pkg/memory"
	"github.com/yourusername/gostra/pkg/models"
)

// 创建一个模拟的模型提供者用于测试
type MockStreamModelProvider struct {
	ID       string
	Provider string
}

func (m *MockStreamModelProvider) GetID() string {
	return m.ID
}

func (m *MockStreamModelProvider) GetProvider() string {
	return m.Provider
}

func (m *MockStreamModelProvider) Generate(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (string, error) {
	return "This is a mock response", nil
}

func (m *MockStreamModelProvider) Stream(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (<-chan string, error) {
	// 创建一个通道来传递流式响应
	outChan := make(chan string, 10)

	// 启动一个goroutine来模拟流式输出
	go func() {
		defer close(outChan)

		// 模拟多个消息块
		chunks := []string{
			"This ", "is ", "a ", "test ", "of ", "streaming ", "response ",
			"with ", "multiple ", "chunks.",
		}

		for _, chunk := range chunks {
			select {
			case <-ctx.Done():
				// 上下文取消，停止发送
				return
			case outChan <- chunk:
				// 成功发送块
				time.Sleep(50 * time.Millisecond) // 添加延迟模拟网络延迟
			}
		}
	}()

	return outChan, nil
}

func TestAgentStreaming(t *testing.T) {
	// 创建模拟的模型提供者
	mockModel := &MockStreamModelProvider{
		ID:       "mock-model",
		Provider: "mock-provider",
	}

	// 创建内存提供者
	memProvider := memory.NewInMemoryProvider()

	// 创建代理
	agent, err := NewAgent(&Options{
		ID:             "test-agent",
		Name:           "Test Agent",
		SystemPrompt:   "You are a test agent.",
		ModelProvider:  mockModel,
		MemoryProvider: memProvider,
	})

	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	// 创建取消上下文
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 准备测试消息
	messages := []Message{
		{
			Role:    "user",
			Content: "Hello, this is a streaming test.",
		},
	}

	// 准备流选项
	options := &StreamOptions{
		MaxSteps:    1,
		Temperature: 0.7,
		AbortSignal: ctx,
	}

	// 调用流式API
	streamResp, err := agent.Stream(messages, options)
	if err != nil {
		t.Fatalf("Stream API failed: %v", err)
	}

	// 验证结果
	var textChunks []string
	var allMessages []Message
	var finishInfo *FinishInfo

	// 收集文本块
	textDone := make(chan bool)
	go func() {
		defer close(textDone)
		for text := range streamResp.TextStream {
			textChunks = append(textChunks, text)
		}
	}()

	// 收集消息
	msgDone := make(chan bool)
	go func() {
		defer close(msgDone)
		for msg := range streamResp.MessageChan {
			allMessages = append(allMessages, msg)
		}
	}()

	// 获取完成信息
	finishDone := make(chan bool)
	go func() {
		defer close(finishDone)
		finish, ok := <-streamResp.FinishChan
		if ok {
			finishInfo = &finish
		}
	}()

	// 等待所有通道关闭或超时
	select {
	case <-textDone:
		t.Log("Text stream completed")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for text stream")
	}

	select {
	case <-msgDone:
		t.Log("Message stream completed")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for message stream")
	}

	select {
	case <-finishDone:
		t.Log("Finish info received")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for finish info")
	}

	// 验证结果
	if len(textChunks) == 0 {
		t.Error("No text chunks received")
	} else {
		t.Logf("Received %d text chunks", len(textChunks))
	}

	if len(allMessages) == 0 {
		t.Error("No messages received")
	} else {
		t.Logf("Received %d messages", len(allMessages))
	}

	if finishInfo == nil {
		t.Error("No finish info received")
	} else {
		t.Logf("Finish reason: %s", finishInfo.FinishReason)
	}
}
