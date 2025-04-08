package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/google/uuid"
	"github.com/yourusername/gostra/pkg/memory"
	"github.com/yourusername/gostra/pkg/models"
	"github.com/yourusername/gostra/pkg/tools"
)

// Message 定义消息的基本结构
type Message struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt int64  `json:"created_at,omitempty"`
	Type      string `json:"type,omitempty"`
}

// GenerateOptions 定义生成文本的选项
type GenerateOptions struct {
	MaxSteps     int               `json:"max_steps"`
	Temperature  float64           `json:"temperature"`
	Output       interface{}       `json:"output,omitempty"`
	ThreadID     string            `json:"thread_id,omitempty"`
	ResourceID   string            `json:"resource_id,omitempty"`
	Instructions string            `json:"instructions,omitempty"`
	AbortSignal  context.Context   `json:"-"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// StreamOptions 定义流式生成文本的选项
type StreamOptions struct {
	MaxSteps     int               `json:"max_steps"`
	Temperature  float64           `json:"temperature"`
	Output       interface{}       `json:"output,omitempty"`
	ThreadID     string            `json:"thread_id,omitempty"`
	ResourceID   string            `json:"resource_id,omitempty"`
	Instructions string            `json:"instructions,omitempty"`
	AbortSignal  context.Context   `json:"-"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// GenerateResponse 定义生成响应的结构
type GenerateResponse struct {
	Text       string      `json:"text"`
	Object     interface{} `json:"object,omitempty"`
	Messages   []Message   `json:"messages"`
	Steps      []Step      `json:"steps,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	FinishInfo FinishInfo  `json:"finish_info,omitempty"`
}

// StreamResponse 定义流式响应的结构
type StreamResponse struct {
	TextStream  <-chan string      `json:"-"`
	ObjectChan  <-chan interface{} `json:"-"`
	MessageChan <-chan Message     `json:"-"`
	FinishChan  <-chan FinishInfo  `json:"-"`
}

// Step 定义执行步骤
type Step struct {
	Text        string        `json:"text"`
	ToolCalls   []ToolCall    `json:"tool_calls,omitempty"`
	ToolResults []interface{} `json:"tool_results,omitempty"`
}

// ToolCall 定义工具调用
type ToolCall struct {
	ID        string      `json:"id"`
	ToolID    string      `json:"tool_id"`
	Arguments interface{} `json:"arguments"`
}

// FinishInfo 定义完成信息
type FinishInfo struct {
	FinishReason string `json:"finish_reason"`
	Usage        Usage  `json:"usage"`
}

// Usage 定义资源使用情况
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Config 定义Agent的配置
type Config struct {
	ID           string               `json:"id"`
	Name         string               `json:"name"`
	Instructions string               `json:"instructions"`
	Model        models.ModelProvider `json:"model"`
	Tools        map[string]Tool      `json:"tools,omitempty"`
	Memory       interface{}          `json:"memory,omitempty"`
}

// Tool 定义工具接口
type Tool interface {
	GetID() string
	GetDescription() string
	GetInputSchema() interface{}
	Execute(params map[string]interface{}, options interface{}) (interface{}, error)
}

// Agent 表示一个AI Agent
type Agent struct {
	ID             string
	Name           string
	SystemPrompt   string
	ModelProvider  models.ModelProvider
	MemoryProvider memory.MemoryProvider
	Tools          map[string]tools.Tool
	MaxTokens      int
	StateManager   *StateManager
}

// Options 是创建Agent的配置选项
type Options struct {
	ID             string
	Name           string
	SystemPrompt   string
	ModelProvider  models.ModelProvider
	MemoryProvider memory.MemoryProvider
	Tools          []tools.Tool
	MaxTokens      int
}

// NewAgent 创建一个新的Agent
func NewAgent(opts *Options) (*Agent, error) {
	if opts == nil {
		return nil, errors.New("options cannot be nil")
	}

	if opts.ModelProvider == nil {
		return nil, errors.New("model provider is required")
	}

	if opts.MemoryProvider == nil {
		return nil, errors.New("memory provider is required")
	}

	id := opts.ID
	if id == "" {
		id = uuid.New().String()
	}

	name := opts.Name
	if name == "" {
		name = "Agent-" + id[:8]
	}

	agent := &Agent{
		ID:             id,
		Name:           name,
		SystemPrompt:   opts.SystemPrompt,
		ModelProvider:  opts.ModelProvider,
		MemoryProvider: opts.MemoryProvider,
		Tools:          make(map[string]tools.Tool),
		MaxTokens:      opts.MaxTokens,
		StateManager:   NewStateManager(id),
	}

	// 如果MaxTokens未设置，使用默认值
	if agent.MaxTokens <= 0 {
		agent.MaxTokens = 4000
	}

	// 注册工具
	for _, tool := range opts.Tools {
		if tool == nil {
			continue
		}
		agent.Tools[tool.GetID()] = tool
	}

	return agent, nil
}

// Run 运行Agent进行推理
func (a *Agent) Run(ctx context.Context, opts *RunOptions) (string, error) {
	if opts == nil {
		return "", errors.New("options cannot be nil")
	}

	if opts.ThreadID == "" {
		return "", errors.New("threadID is required")
	}

	// 检查Agent是否已经在运行
	if a.StateManager.IsRunning() {
		return "", errors.New("agent is already running")
	}

	// 更新状态为运行中
	taskID := "task_" + uuid.New().String()
	a.StateManager.SetRunning(opts.ThreadID, taskID)

	// 确保在函数返回时更新状态
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("panic occurred: %v", r)
			a.StateManager.SetError(errMsg)
			log.Printf("[AGENT] %s", errMsg)
			panic(r) // 重新抛出panic
		}
	}()

	// 获取可用工具
	availableTools := opts.AvailableTools
	if availableTools == nil {
		// 如果未指定，使用所有注册的工具
		availableTools = a.getToolsList()
	}

	// 如果有用户输入，添加到线程
	if opts.Input != "" {
		_, err := a.MemoryProvider.AddMessage(ctx, opts.ThreadID, "user", opts.Input, nil)
		if err != nil {
			a.StateManager.SetError(fmt.Sprintf("failed to add user message: %v", err))
			return "", fmt.Errorf("failed to add user message: %w", err)
		}
	}

	// 设置最大连续调用次数，防止无限循环
	maxConsecutiveCalls := opts.MaxConsecutiveCalls
	if maxConsecutiveCalls <= 0 {
		maxConsecutiveCalls = 10
	}

	// 准备工具定义
	toolDefs := make([]models.ToolDefinition, 0, len(availableTools))
	for _, tool := range availableTools {
		schema, err := tool.GetInputSchema().JSONSchema()
		if err != nil {
			a.StateManager.SetError(fmt.Sprintf("failed to get tool schema: %v", err))
			return "", fmt.Errorf("failed to get tool schema: %w", err)
		}

		toolDefs = append(toolDefs, models.ToolDefinition{
			Type: "function",
			Function: models.FunctionDefinition{
				Name:        tool.GetID(),
				Description: tool.GetDescription(),
				Parameters:  schema,
			},
		})
	}

	var finalResponse string

	// 思考循环
	for callCount := 0; callCount < maxConsecutiveCalls; callCount++ {
		// 构建消息历史
		messages, err := a.buildMessages(ctx, opts.ThreadID)
		if err != nil {
			a.StateManager.SetError(fmt.Sprintf("failed to build messages: %v", err))
			return "", fmt.Errorf("failed to build messages: %w", err)
		}

		// 准备生成选项
		genOpts := &models.GenerateOptions{
			MaxTokens: a.MaxTokens,
			Tools:     toolDefs,
			// 可选：设置工具选择策略
			// ToolChoice: "auto",
		}

		// 调用模型生成响应
		response, err := a.ModelProvider.Generate(ctx, messages, genOpts)
		if err != nil {
			a.StateManager.SetError(fmt.Sprintf("model generation failed: %v", err))
			return "", fmt.Errorf("model generation failed: %w", err)
		}

		// 检查是否有工具调用
		toolCalls, hasToolCalls := a.parseToolCalls(response)
		if !hasToolCalls {
			// 如果没有工具调用，保存最终响应并返回
			if _, err := a.MemoryProvider.AddMessage(ctx, opts.ThreadID, "assistant", response, nil); err != nil {
				a.StateManager.SetError(fmt.Sprintf("failed to add assistant message: %v", err))
				return "", fmt.Errorf("failed to add assistant message: %w", err)
			}

			finalResponse = response
			break
		}

		// 有工具调用，保存助手消息
		assistantMetadata := map[string]interface{}{
			"has_tool_calls": true,
			"tool_calls":     toolCalls,
		}

		if _, err := a.MemoryProvider.AddMessage(ctx, opts.ThreadID, "assistant", response, assistantMetadata); err != nil {
			a.StateManager.SetError(fmt.Sprintf("failed to add assistant message with tool calls: %v", err))
			return "", fmt.Errorf("failed to add assistant message with tool calls: %w", err)
		}

		// 执行工具调用
		for _, toolCall := range toolCalls {
			toolID := toolCall.ID
			toolName := toolCall.Function.Name
			argsStr := toolCall.Function.Arguments

			// 记录当前工具调用
			a.StateManager.SetData("current_tool", toolName)

			// 解析参数
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(argsStr), &args); err != nil {
				log.Printf("Failed to parse tool arguments: %v", err)
				toolResult := fmt.Sprintf("Error: Failed to parse tool arguments - %v", err)

				if _, err := a.MemoryProvider.AddMessage(ctx, opts.ThreadID, "tool", toolResult, map[string]interface{}{
					"tool_call_id": toolID,
					"tool_name":    toolName,
				}); err != nil {
					a.StateManager.SetError(fmt.Sprintf("failed to add tool error message: %v", err))
					return "", fmt.Errorf("failed to add tool error message: %w", err)
				}

				continue
			}

			// 查找工具
			tool := a.findTool(availableTools, toolName)
			if tool == nil {
				toolResult := fmt.Sprintf("Error: Tool not found - %s", toolName)

				if _, err := a.MemoryProvider.AddMessage(ctx, opts.ThreadID, "tool", toolResult, map[string]interface{}{
					"tool_call_id": toolID,
					"tool_name":    toolName,
				}); err != nil {
					a.StateManager.SetError(fmt.Sprintf("failed to add tool error message: %v", err))
					return "", fmt.Errorf("failed to add tool error message: %w", err)
				}

				continue
			}

			// 执行工具
			execOpts := &tools.ExecuteOptions{
				ThreadID: opts.ThreadID,
				CallID:   toolID,
			}

			a.StateManager.SetData("tool_execution_started", time.Now())
			result, err := tool.Execute(args, execOpts)
			a.StateManager.DeleteData("tool_execution_started") // 清除开始执行时间

			var resultStr string
			if err != nil {
				a.StateManager.SetData("last_tool_error", err.Error())
				resultStr = fmt.Sprintf("Error: %v", err)
			} else {
				// 将结果转换为字符串
				switch r := result.(type) {
				case string:
					resultStr = r
				default:
					resultBytes, err := json.Marshal(result)
					if err != nil {
						resultStr = fmt.Sprintf("%v", result)
					} else {
						resultStr = string(resultBytes)
					}
				}
				a.StateManager.SetData("last_tool_result", resultStr)
			}

			// 保存工具结果
			if _, err := a.MemoryProvider.AddMessage(ctx, opts.ThreadID, "tool", resultStr, map[string]interface{}{
				"tool_call_id": toolID,
				"tool_name":    toolName,
			}); err != nil {
				a.StateManager.SetError(fmt.Sprintf("failed to add tool result message: %v", err))
				return "", fmt.Errorf("failed to add tool result message: %w", err)
			}

			// 清除当前工具
			a.StateManager.DeleteData("current_tool")
		}
	}

	// 任务完成，更新状态
	a.StateManager.CompleteTask(taskID)
	a.StateManager.SetIdle()

	return finalResponse, nil
}

// parseToolCalls 从模型响应中解析工具调用
func (a *Agent) parseToolCalls(response string) ([]models.ToolCall, bool) {
	// 尝试查找JSON格式的工具调用
	toolCalls := make([]models.ToolCall, 0)

	// 简单解析示例，实际应根据具体的模型输出格式调整
	// 在内容中查找工具调用标记
	startMarker := "```json"
	endMarker := "```"

	start := strings.Index(response, startMarker)
	if start == -1 {
		return toolCalls, false
	}

	// 找到结束标记
	start += len(startMarker)
	end := strings.Index(response[start:], endMarker)
	if end == -1 {
		return toolCalls, false
	}

	jsonStr := strings.TrimSpace(response[start : start+end])

	// 尝试解析为单个工具调用
	var singleCall models.ToolCall
	err := json.Unmarshal([]byte(jsonStr), &singleCall)
	if err == nil && singleCall.Type == "function" && singleCall.Function.Name != "" {
		// 有效的单个工具调用
		if singleCall.ID == "" {
			singleCall.ID = uuid.New().String()
		}
		toolCalls = append(toolCalls, singleCall)
		return toolCalls, true
	}

	// 尝试解析为工具调用数组
	var multipleCalls []models.ToolCall
	err = json.Unmarshal([]byte(jsonStr), &multipleCalls)
	if err == nil && len(multipleCalls) > 0 {
		// 有效的多个工具调用
		for i := range multipleCalls {
			if multipleCalls[i].ID == "" {
				multipleCalls[i].ID = uuid.New().String()
			}
		}
		return multipleCalls, true
	}

	// 如果没有找到有效的工具调用
	return toolCalls, false
}

// buildMessages 从内存线程构建消息历史
func (a *Agent) buildMessages(ctx context.Context, threadID string) ([]models.Message, error) {
	// 获取线程中的消息
	memoryMessages, err := a.MemoryProvider.GetMessages(ctx, threadID, 100, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	// 构建模型消息
	modelMessages := make([]models.Message, 0, len(memoryMessages)+1)

	// 添加系统消息
	if a.SystemPrompt != "" {
		modelMessages = append(modelMessages, models.Message{
			Role:    "system",
			Content: a.SystemPrompt,
		})
	}

	// 添加历史消息
	for _, msg := range memoryMessages {
		modelMessage := models.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}

		// 处理工具消息，添加name字段
		if msg.Role == "tool" && msg.Metadata != nil {
			if toolCallID, ok := msg.Metadata["tool_call_id"].(string); ok {
				modelMessage.Name = toolCallID
			}
		}

		modelMessages = append(modelMessages, modelMessage)
	}

	return modelMessages, nil
}

// getToolsList 获取所有注册的工具列表
func (a *Agent) getToolsList() []tools.Tool {
	tools := make([]tools.Tool, 0, len(a.Tools))
	for _, tool := range a.Tools {
		tools = append(tools, tool)
	}
	return tools
}

// findTool 根据名称查找工具
func (a *Agent) findTool(toolsList []tools.Tool, name string) tools.Tool {
	for _, tool := range toolsList {
		if tool.GetID() == name {
			return tool
		}
	}
	return nil
}

// RegisterTool 注册一个工具
func (a *Agent) RegisterTool(tool tools.Tool) error {
	if tool == nil {
		return errors.New("tool cannot be nil")
	}

	toolID := tool.GetID()
	if toolID == "" {
		return errors.New("tool ID cannot be empty")
	}

	if _, exists := a.Tools[toolID]; exists {
		return fmt.Errorf("tool with ID '%s' already registered", toolID)
	}

	a.Tools[toolID] = tool
	return nil
}

// GetTool 获取工具
func (a *Agent) GetTool(toolID string) (tools.Tool, error) {
	tool, exists := a.Tools[toolID]
	if !exists {
		return nil, fmt.Errorf("tool with ID '%s' not found", toolID)
	}
	return tool, nil
}

// GetState 获取代理的当前状态
func (a *Agent) GetState() *State {
	return a.StateManager.GetState()
}

// SaveState 保存代理的状态为JSON字符串
func (a *Agent) SaveState() (string, error) {
	return a.StateManager.SaveState()
}

// LoadState 从JSON字符串加载代理状态
func (a *Agent) LoadState(stateJSON string) error {
	return a.StateManager.LoadState(stateJSON)
}

// Stop 停止代理
func (a *Agent) Stop() {
	a.StateManager.SetStopped()
}

// Pause 暂停代理
func (a *Agent) Pause() {
	a.StateManager.SetPaused()
}

// Resume 恢复代理
func (a *Agent) Resume() {
	a.StateManager.SetIdle()
}

// Generate 生成文本响应
func (a *Agent) Generate(messages []Message, options *GenerateOptions) (*GenerateResponse, error) {
	if len(messages) == 0 {
		return nil, errors.New("empty messages")
	}

	if options == nil {
		options = &GenerateOptions{
			MaxSteps:    1,
			Temperature: 0.7,
		}
	}

	// 这里应该调用Agent Actor来处理请求
	// 在完整实现中，这里会发送消息给Agent Actor并等待响应
	// 现在只返回一个模拟的响应

	return &GenerateResponse{
		Text: fmt.Sprintf("Response to: %s", messages[len(messages)-1].Content),
		Messages: []Message{
			{
				Role:    "assistant",
				Content: fmt.Sprintf("Response to: %s", messages[len(messages)-1].Content),
			},
		},
		Steps: []Step{
			{
				Text: fmt.Sprintf("Response to: %s", messages[len(messages)-1].Content),
			},
		},
		FinishInfo: FinishInfo{
			FinishReason: "stop",
			Usage: Usage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
			},
		},
	}, nil
}

// Stream 流式生成文本响应
func (a *Agent) Stream(messages []Message, options *StreamOptions) (*StreamResponse, error) {
	if len(messages) == 0 {
		return nil, errors.New("empty messages")
	}

	if options == nil {
		options = &StreamOptions{
			MaxSteps:    1,
			Temperature: 0.7,
		}
	}

	// 创建通道
	textChan := make(chan string)
	msgChan := make(chan Message)
	finishChan := make(chan FinishInfo)
	objectChan := make(chan interface{})

	// 异步处理流式响应
	go func() {
		defer close(textChan)
		defer close(msgChan)
		defer close(finishChan)
		defer close(objectChan)

		// 模拟流式响应
		responseText := fmt.Sprintf("Response to: %s", messages[len(messages)-1].Content)

		// 分块发送
		for _, word := range []string{"Response ", "to: ", messages[len(messages)-1].Content} {
			select {
			case <-options.AbortSignal.Done():
				return
			case textChan <- word:
				// 发送成功
			}
		}

		// 发送消息
		msgChan <- Message{
			Role:    "assistant",
			Content: responseText,
		}

		// 发送完成信息
		finishChan <- FinishInfo{
			FinishReason: "stop",
			Usage: Usage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
			},
		}
	}()

	return &StreamResponse{
		TextStream:  textChan,
		MessageChan: msgChan,
		FinishChan:  finishChan,
		ObjectChan:  objectChan,
	}, nil
}

// AgentActor 定义Agent的Actor
type AgentActor struct {
	agent *Agent
}

// Receive 处理接收到的消息
func (a *AgentActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *GenerateMessage:
		response, err := a.agent.Generate(msg.Messages, msg.Options)
		context.Respond(&GenerateResponse{
			Text:       response.Text,
			Messages:   response.Messages,
			Steps:      response.Steps,
			FinishInfo: response.FinishInfo,
		})
		if err != nil {
			context.Respond(err)
		}
	case *StreamMessage:
		response, err := a.agent.Stream(msg.Messages, msg.Options)
		if err != nil {
			context.Respond(err)
			return
		}
		context.Respond(response)
	}
}

// GenerateMessage 定义生成消息的请求
type GenerateMessage struct {
	Messages []Message        `json:"messages"`
	Options  *GenerateOptions `json:"options,omitempty"`
}

// StreamMessage 定义流式消息的请求
type StreamMessage struct {
	Messages []Message      `json:"messages"`
	Options  *StreamOptions `json:"options,omitempty"`
}

// NewAgentActor 创建一个新的Agent Actor Props
func NewAgentActor(agent *Agent) *actor.Props {
	return actor.PropsFromProducer(func() actor.Actor {
		return &AgentActor{
			agent: agent,
		}
	})
}
