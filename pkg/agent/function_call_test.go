package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/louloulin/gostra/pkg/memory"
	"github.com/louloulin/gostra/pkg/models"
	"github.com/louloulin/gostra/pkg/tools"
	"github.com/stretchr/testify/assert"
)

// MockFunctionCallModelProvider 实现了具有函数调用功能的模型提供者
type MockFunctionCallModelProvider struct {
	ID                 string
	Provider           string
	ShouldCallFunction bool
	FunctionToCall     string
	ResponseText       string
}

func (m *MockFunctionCallModelProvider) GetID() string {
	return m.ID
}

func (m *MockFunctionCallModelProvider) GetProvider() string {
	return m.Provider
}

// Generate 生成文本，不使用流式响应
func (m *MockFunctionCallModelProvider) Generate(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (string, error) {
	return m.ResponseText, nil
}

// GenerateWithFunctionCalls 生成文本，并支持函数调用
func (m *MockFunctionCallModelProvider) GenerateWithFunctionCalls(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (*models.ResponseWithFunctionCalls, error) {
	response := &models.ResponseWithFunctionCalls{
		Text:         m.ResponseText,
		FinishReason: "stop",
	}

	if m.ShouldCallFunction && len(options.Tools) > 0 {
		// 如果设置为需要调用函数，并且提供了工具定义
		for _, tool := range options.Tools {
			if tool.Function.Name == m.FunctionToCall || m.FunctionToCall == "" {
				// 使用第一个找到的工具或指定的工具
				response.FinishReason = "function_call"
				response.ToolCalls = []models.ToolCall{
					{
						ID:   "call_" + time.Now().Format("20060102150405"),
						Type: "function",
						Function: models.FunctionCall{
							Name:      tool.Function.Name,
							Arguments: `{"text": "test argument"}`,
						},
					},
				}
				break
			}
		}
	}

	return response, nil
}

// Stream 生成文本，使用流式响应
func (m *MockFunctionCallModelProvider) Stream(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (<-chan string, error) {
	// 创建一个通道来传递流式响应
	outChan := make(chan string, 10)

	// 启动一个goroutine来模拟流式输出
	go func() {
		defer close(outChan)

		// 将回复拆分为多个块
		chunks := strings.Split(m.ResponseText, " ")

		for _, chunk := range chunks {
			select {
			case <-ctx.Done():
				// 上下文取消，停止发送
				return
			case outChan <- chunk + " ":
				// 成功发送块
				time.Sleep(20 * time.Millisecond) // 添加延迟模拟网络延迟
			}
		}
	}()

	return outChan, nil
}

// StreamWithFunctionCalls 流式生成文本，并支持函数调用
func (m *MockFunctionCallModelProvider) StreamWithFunctionCalls(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (<-chan *models.ResponseChunk, error) {
	// 创建一个通道来传递流式响应
	outChan := make(chan *models.ResponseChunk, 10)

	// 启动一个goroutine来模拟流式输出
	go func() {
		defer close(outChan)

		// 将回复拆分为多个块
		chunks := strings.Split(m.ResponseText, " ")

		for i, chunk := range chunks {
			select {
			case <-ctx.Done():
				// 上下文取消，停止发送
				return
			case outChan <- &models.ResponseChunk{
				Text: chunk + " ",
			}:
				// 成功发送块
				time.Sleep(20 * time.Millisecond) // 添加延迟模拟网络延迟
			}

			// 发送最后一个块时，如果需要调用函数，添加函数调用
			if i == len(chunks)-1 && m.ShouldCallFunction && len(options.Tools) > 0 {
				var toolName string

				// 寻找要调用的工具
				for _, tool := range options.Tools {
					if tool.Function.Name == m.FunctionToCall || m.FunctionToCall == "" {
						toolName = tool.Function.Name
						break
					}
				}

				if toolName != "" {
					callID := "call_" + time.Now().Format("20060102150405")

					// 发送工具调用块
					outChan <- &models.ResponseChunk{
						ToolCallChunk: &models.ToolCallChunk{
							ID:   callID,
							Type: "function",
							Function: &models.FunctionCallChunk{
								Name:      toolName,
								Arguments: `{"text": "`,
							},
						},
					}

					outChan <- &models.ResponseChunk{
						ToolCallChunk: &models.ToolCallChunk{
							ID: callID,
							Function: &models.FunctionCallChunk{
								Arguments: `test argument"}`,
							},
						},
					}

					// 发送完成信号
					outChan <- &models.ResponseChunk{
						IsFinished:   true,
						FinishReason: "tool_calls",
					}
					return
				}
			}
		}

		// 发送完成信号
		outChan <- &models.ResponseChunk{
			IsFinished:   true,
			FinishReason: "stop",
		}
	}()

	return outChan, nil
}

// MockEchoTool 实现一个简单的回显工具
type MockEchoTool struct {
	tools.BasicTool
}

// EchoSchema 定义回显工具的输入Schema
type EchoSchema struct{}

func (s *EchoSchema) Validate(params map[string]interface{}) error {
	if _, ok := params["text"]; !ok {
		return tools.NewValidationError("missing required parameter: text")
	}
	return nil
}

func (s *EchoSchema) JSONSchema() (map[string]interface{}, error) {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"text": map[string]interface{}{
				"type":        "string",
				"description": "The text to echo back",
			},
		},
		"required": []string{"text"},
	}, nil
}

// NewMockEchoTool 创建一个新的模拟回显工具
func NewMockEchoTool() *MockEchoTool {
	tool := &MockEchoTool{}
	tool.ID = "echo"
	tool.Description = "Echo back the input text"
	tool.InputSchema = &EchoSchema{}
	tool.ExecuteFunc = func(params map[string]interface{}, options *tools.ExecuteOptions) (interface{}, error) {
		text, _ := params["text"].(string)
		return map[string]interface{}{
			"echoed": text,
			"time":   time.Now().Format(time.RFC3339),
		}, nil
	}
	return tool
}

// TestAgentFunctionCall 测试代理的函数调用功能
func TestAgentFunctionCall(t *testing.T) {
	// 创建模拟的模型提供者
	mockModel := &MockFunctionCallModelProvider{
		ID:                 "mock-model",
		Provider:           "mock",
		ShouldCallFunction: true,
		ResponseText:       "I'll help you with that request.",
	}

	// 创建内存提供者
	memProvider := memory.NewInMemoryProvider()

	// 创建回显工具
	echoTool := NewMockEchoTool()

	// 创建代理
	agent, err := NewAgent(&Options{
		ID:             "test-agent",
		Name:           "Test Agent",
		SystemPrompt:   "You are a test agent that uses functions.",
		ModelProvider:  mockModel,
		MemoryProvider: memProvider,
		Tools:          []tools.Tool{echoTool},
	})

	assert.NoError(t, err)
	assert.NotNil(t, agent)

	// 创建线程
	thread, err := memProvider.CreateThread(context.Background(), nil)
	assert.NoError(t, err)
	assert.NotNil(t, thread)

	// 测试GenerateWithFunctionCalls方法
	response, err := agent.GenerateWithFunctionCalls(context.Background(), "Can you echo this for me?")
	assert.NoError(t, err)
	assert.Contains(t, response, "echoed")

	// 测试解析工具调用
	testResponse := "```json\n{\"id\":\"call_123\",\"type\":\"function\",\"function\":{\"name\":\"echo\",\"arguments\":\"{\\\"text\\\":\\\"test\\\"}\"}}```"
	toolCalls, hasToolCalls := agent.parseToolCalls(testResponse)
	assert.True(t, hasToolCalls)
	assert.Equal(t, 1, len(toolCalls))
	assert.Equal(t, "echo", toolCalls[0].Function.Name)

	// 测试多个工具调用
	testMultiResponse := "```json\n[{\"id\":\"call_123\",\"type\":\"function\",\"function\":{\"name\":\"echo\",\"arguments\":\"{\\\"text\\\":\\\"test1\\\"}\"}},{\"id\":\"call_456\",\"type\":\"function\",\"function\":{\"name\":\"echo\",\"arguments\":\"{\\\"text\\\":\\\"test2\\\"}\"}}]```"
	toolCalls, hasToolCalls = agent.parseToolCalls(testMultiResponse)
	assert.True(t, hasToolCalls)
	assert.Equal(t, 2, len(toolCalls))
	assert.Equal(t, "echo", toolCalls[0].Function.Name)
	assert.Equal(t, "echo", toolCalls[1].Function.Name)

	// 测试无工具调用
	testNoCallResponse := "This is a normal response without any function calls."
	toolCalls, hasToolCalls = agent.parseToolCalls(testNoCallResponse)
	assert.False(t, hasToolCalls)
	assert.Equal(t, 0, len(toolCalls))

	// 测试处理函数调用
	functionCall := &models.FunctionCall{
		Name:      "echo",
		Arguments: `{"text": "function call test"}`,
	}

	result, err := agent.handleFunctionCall(context.Background(), functionCall)
	assert.NoError(t, err)

	var resultMap map[string]interface{}
	err = json.Unmarshal([]byte(result), &resultMap)
	assert.NoError(t, err)
	assert.Equal(t, "function call test", resultMap["echoed"])
}

// TestAgentStreamWithFunctionCalls 测试代理的流式函数调用功能
func TestAgentStreamWithFunctionCalls(t *testing.T) {
	// 创建模拟的模型提供者
	mockModel := &MockFunctionCallModelProvider{
		ID:                 "mock-model",
		Provider:           "mock",
		ShouldCallFunction: true,
		FunctionToCall:     "echo",
		ResponseText:       "I am streaming a response with function call",
	}

	// 创建内存提供者
	memProvider := memory.NewInMemoryProvider()

	// 创建回显工具
	echoTool := NewMockEchoTool()

	// 创建代理
	agent, err := NewAgent(&Options{
		ID:             "test-agent",
		Name:           "Test Agent",
		SystemPrompt:   "You are a test agent that uses functions.",
		ModelProvider:  mockModel,
		MemoryProvider: memProvider,
		Tools:          []tools.Tool{echoTool},
	})

	assert.NoError(t, err)
	assert.NotNil(t, agent)

	// 创建上下文
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 测试StreamWithFunctionCalls方法
	stream, err := agent.StreamWithFunctionCalls(ctx, "Can you echo this for me in a stream?")
	assert.NoError(t, err)
	assert.NotNil(t, stream)

	// 从流中收集响应
	var texts []string
	var functionCalls []*models.FunctionCallChunk
	var toolCalls []*models.ToolCallChunk
	var isFinished bool

	for chunk := range stream {
		if chunk.Text != "" {
			texts = append(texts, chunk.Text)
		}

		if chunk.FunctionCallChunk != nil {
			functionCalls = append(functionCalls, chunk.FunctionCallChunk)
		}

		if chunk.ToolCallChunk != nil {
			toolCalls = append(toolCalls, chunk.ToolCallChunk)
		}

		if chunk.IsFinished {
			isFinished = true
		}
	}

	// 验证流式响应
	assert.True(t, len(texts) > 0 || len(toolCalls) > 0)
	assert.True(t, isFinished)
}

// TestRunWithFunctionCalls 测试Agent.Run方法处理函数调用
func TestRunWithFunctionCalls(t *testing.T) {
	// 创建模拟的模型提供者，配置为返回工具调用
	mockModel := &MockFunctionCallModelProvider{
		ID:                 "mock-model",
		Provider:           "mock",
		ShouldCallFunction: true,
		FunctionToCall:     "echo",
		ResponseText:       "```json\n{\"id\":\"call_123\",\"type\":\"function\",\"function\":{\"name\":\"echo\",\"arguments\":\"{\\\"text\\\":\\\"run test\\\"}\"}}```",
	}

	// 创建内存提供者
	memProvider := memory.NewInMemoryProvider()

	// 创建回显工具
	echoTool := NewMockEchoTool()

	// 创建代理
	agent, err := NewAgent(&Options{
		ID:             "test-agent",
		Name:           "Test Agent",
		SystemPrompt:   "You are a test agent that uses functions.",
		ModelProvider:  mockModel,
		MemoryProvider: memProvider,
		Tools:          []tools.Tool{echoTool},
	})

	assert.NoError(t, err)
	assert.NotNil(t, agent)

	// 创建线程
	thread, err := memProvider.CreateThread(context.Background(), nil)
	assert.NoError(t, err)
	assert.NotNil(t, thread)

	// 使用Run方法
	response, err := agent.Run(context.Background(), &RunOptions{
		ThreadID:            thread.ID,
		Input:               "Can you echo this for me?",
		MaxConsecutiveCalls: 5,
	})

	assert.NoError(t, err)
	assert.NotEmpty(t, response)

	// 检查消息历史，应该包含用户消息、助手消息和工具结果
	messages, err := memProvider.GetMessages(context.Background(), thread.ID, 10, 0)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(messages), 3) // 至少有用户、助手、工具三条消息

	// 验证消息类型
	assert.Equal(t, "user", messages[0].Role)
	assert.Equal(t, "assistant", messages[1].Role)

	// 如果第三条消息存在，它应该是工具响应
	if len(messages) > 2 {
		assert.Equal(t, "tool", messages[2].Role)
	}
}
