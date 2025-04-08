package models

import (
	"context"
	"encoding/json"
)

// ModelProvider 定义模型提供者接口
type ModelProvider interface {
	// GetID 返回模型ID
	GetID() string

	// GetProvider 返回提供者名称（如openai, anthropic, gemini等）
	GetProvider() string

	// Generate 生成文本，不使用流式响应
	Generate(ctx context.Context, messages []Message, options *GenerateOptions) (string, error)

	// GenerateWithFunctionCalls 生成文本，并支持函数调用
	GenerateWithFunctionCalls(ctx context.Context, messages []Message, options *GenerateOptions) (*ResponseWithFunctionCalls, error)

	// Stream 生成文本，使用流式响应
	Stream(ctx context.Context, messages []Message, options *GenerateOptions) (<-chan string, error)

	// StreamWithFunctionCalls 流式生成文本，并支持函数调用
	StreamWithFunctionCalls(ctx context.Context, messages []Message, options *GenerateOptions) (<-chan *ResponseChunk, error)
}

// Message 表示发送到模型的消息
type Message struct {
	Role         string        `json:"role"`                    // 角色: user, assistant, system, function等
	Content      string        `json:"content"`                 // 消息内容
	Name         string        `json:"name,omitempty"`          // 可选的名称，用于function调用
	FunctionCall *FunctionCall `json:"function_call,omitempty"` // 函数调用信息，适用于assistant角色
	ToolCalls    []ToolCall    `json:"tool_calls,omitempty"`    // 工具调用列表，适用于assistant角色
}

// ResponseWithFunctionCalls 包含文本响应和函数调用
type ResponseWithFunctionCalls struct {
	Text         string        `json:"text"`                    // 模型生成的文本
	FunctionCall *FunctionCall `json:"function_call,omitempty"` // 单个函数调用（旧版API）
	ToolCalls    []ToolCall    `json:"tool_calls,omitempty"`    // 多个工具调用（新版API）
	FinishReason string        `json:"finish_reason"`           // 结束原因: stop, length, function_call等
}

// ResponseChunk 表示流式响应的一块内容
type ResponseChunk struct {
	Text              string             `json:"text,omitempty"`                // 文本内容
	FunctionCallChunk *FunctionCallChunk `json:"function_call_chunk,omitempty"` // 函数调用块
	ToolCallChunk     *ToolCallChunk     `json:"tool_call_chunk,omitempty"`     // 工具调用块
	IsFinished        bool               `json:"is_finished"`                   // 是否是最后一块
	FinishReason      string             `json:"finish_reason,omitempty"`       // 结束原因
}

// FunctionCallChunk 表示流式函数调用的一部分
type FunctionCallChunk struct {
	Name      string `json:"name"`      // 函数名称
	Arguments string `json:"arguments"` // 函数参数的一部分
	Index     int    `json:"index"`     // 当前块的索引
}

// ToolCallChunk 表示流式工具调用的一部分
type ToolCallChunk struct {
	ID       string             `json:"id"`       // 工具调用ID
	Type     string             `json:"type"`     // 工具类型
	Function *FunctionCallChunk `json:"function"` // 函数调用块
	Index    int                `json:"index"`    // 当前工具在工具列表中的索引
}

// GenerateOptions 包含生成文本的选项
type GenerateOptions struct {
	Temperature      float64                `json:"temperature,omitempty"`       // 温度参数，控制随机性
	MaxTokens        int                    `json:"max_tokens,omitempty"`        // 最大生成的token数量
	TopP             float64                `json:"top_p,omitempty"`             // 核取样参数
	FrequencyPenalty float64                `json:"frequency_penalty,omitempty"` // 频率惩罚
	PresencePenalty  float64                `json:"presence_penalty,omitempty"`  // 存在惩罚
	Stop             []string               `json:"stop,omitempty"`              // 停止序列
	Tools            []ToolDefinition       `json:"tools,omitempty"`             // 可用工具定义
	ToolChoice       interface{}            `json:"tool_choice,omitempty"`       // 工具选择策略
	LogitBias        map[string]interface{} `json:"logit_bias,omitempty"`        // 逻辑偏差
	FunctionCall     interface{}            `json:"function_call,omitempty"`     // 函数调用策略（兼容旧版API）
}

// ToolDefinition 表示可以被模型调用的工具定义
type ToolDefinition struct {
	Type     string             `json:"type"`     // 工具类型，通常是"function"
	Function FunctionDefinition `json:"function"` // 函数定义
}

// FunctionDefinition 表示函数定义
type FunctionDefinition struct {
	Name        string                 `json:"name"`               // 函数名称
	Description string                 `json:"description"`        // 函数描述
	Parameters  map[string]interface{} `json:"parameters"`         // 参数架构
	Required    []string               `json:"required,omitempty"` // 必需参数列表
}

// ToolCall 表示模型的工具调用请求
type ToolCall struct {
	ID       string       `json:"id"`       // 工具调用的唯一ID
	Type     string       `json:"type"`     // 工具类型，通常是"function"
	Function FunctionCall `json:"function"` // 函数调用
}

// FunctionCall 表示函数调用
type FunctionCall struct {
	Name      string `json:"name"`      // 函数名称
	Arguments string `json:"arguments"` // 函数参数，JSON字符串
}

// ToolResult 表示工具调用的结果
type ToolResult struct {
	ToolCallID string      `json:"tool_call_id"` // 对应的工具调用ID
	Output     interface{} `json:"output"`       // 工具输出结果
}

// ConvertToMessages 将字符串数组转换为消息数组
func ConvertToMessages(inputs []string) []Message {
	if len(inputs) == 0 {
		return []Message{}
	}

	messages := make([]Message, len(inputs))
	for i, content := range inputs {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}

		messages[i] = Message{
			Role:    role,
			Content: content,
		}
	}

	return messages
}

// DefaultGenerateOptions 返回默认的生成选项
func DefaultGenerateOptions() *GenerateOptions {
	return &GenerateOptions{
		Temperature: 0.7,
		MaxTokens:   1000,
		TopP:        1.0,
	}
}

// BuildToolMessages constructs messages for tool response
func BuildToolMessages(messages []Message, toolResults []ToolResult) []Message {
	newMessages := make([]Message, 0, len(messages)+len(toolResults))

	// Copy existing messages
	newMessages = append(newMessages, messages...)

	// Add tool messages
	for _, result := range toolResults {
		var content string
		switch output := result.Output.(type) {
		case string:
			content = output
		default:
			jsonBytes, _ := json.Marshal(output)
			content = string(jsonBytes)
		}

		newMessages = append(newMessages, Message{
			Role:    "tool",
			Content: content,
			Name:    result.ToolCallID,
		})
	}

	return newMessages
}
