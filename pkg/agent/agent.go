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
	"github.com/louloulin/gostra/pkg/memory"
	"github.com/louloulin/gostra/pkg/models"
	"github.com/louloulin/gostra/pkg/tools"
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
	var steps []Step
	toolCallCount := 0
	var lastAssistantResponseText string // Store the last assistant response text

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

		// 调用模型生成响应 (Using GenerateWithFunctionCalls)
		response, err := a.ModelProvider.GenerateWithFunctionCalls(ctx, messages, genOpts)
		if err != nil {
			a.StateManager.SetError(fmt.Sprintf("model generation failed: %v", err))
			return "", fmt.Errorf("model generation failed: %w", err)
		}

		lastAssistantResponseText = response.Text // Capture the latest response text from the struct

		// 检查是否有工具调用 (Check FinishReason and ToolCalls from struct)
		hasToolCalls := len(response.ToolCalls) > 0
		isFinished := response.FinishReason != "tool_calls" && response.FinishReason != "function_call"

		if !hasToolCalls && isFinished {
			// 如果没有工具调用且模型完成，保存最终响应并返回
			if _, err := a.MemoryProvider.AddMessage(ctx, opts.ThreadID, "assistant", response.Text, nil); err != nil {
				a.StateManager.SetError(fmt.Sprintf("failed to add assistant message: %v", err))
				return "", fmt.Errorf("failed to add assistant message: %w", err)
			}
			finalResponse = response.Text
			break // Exit loop, normal finish
		}

		// 有工具调用或模型未完成，保存助手消息
		assistantMetadata := map[string]interface{}{
			"has_tool_calls": hasToolCalls,
			"tool_calls":     response.ToolCalls,
			"finish_reason":  response.FinishReason,
		}
		if _, err := a.MemoryProvider.AddMessage(ctx, opts.ThreadID, "assistant", response.Text, assistantMetadata); err != nil {
			a.StateManager.SetError(fmt.Sprintf("failed to add assistant message with tool calls: %v", err))
			return "", fmt.Errorf("failed to add assistant message with tool calls: %w", err)
		}

		// 如果没有工具调用但模型要求继续 (e.g., maybe length limit), continue loop
		if !hasToolCalls {
			continue
		}

		// 执行工具调用
		// Declare slices to hold results for this step
		toolResultsData := make([]interface{}, len(response.ToolCalls))
		toolResultsMsgs := make([]models.Message, 0, len(response.ToolCalls))

		for i, toolCall := range response.ToolCalls { // Use ToolCalls from struct
			toolID := toolCall.ID
			toolName := toolCall.Function.Name
			argsStr := toolCall.Function.Arguments
			toolCallCount++

			// 记录当前工具调用
			a.StateManager.SetData("current_tool", toolName)

			// 解析参数
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(argsStr), &args); err != nil {
				log.Printf("Failed to parse tool arguments: %v", err)
				toolResultStr := fmt.Sprintf("Error: Failed to parse tool arguments - %v", err)
				toolResultsData[i] = toolResultStr // Store error string in data slice
				toolResultsMsgs = append(toolResultsMsgs, models.Message{
					Role:    "tool",
					Content: toolResultStr,
					Name:    toolID,
				})
				if _, err := a.MemoryProvider.AddMessage(ctx, opts.ThreadID, "tool", toolResultStr, map[string]interface{}{
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
				toolResultStr := fmt.Sprintf("Error: Tool not found - %s", toolName)
				toolResultsData[i] = toolResultStr // Store error string in data slice
				toolResultsMsgs = append(toolResultsMsgs, models.Message{
					Role:    "tool",
					Content: toolResultStr,
					Name:    toolID,
				})
				if _, err := a.MemoryProvider.AddMessage(ctx, opts.ThreadID, "tool", toolResultStr, map[string]interface{}{
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

			toolResultsData[i] = result // Store raw result

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
					resultBytes, marshalErr := json.Marshal(result)
					if marshalErr != nil {
						resultStr = fmt.Sprintf("%+v", result)
					} else {
						resultStr = string(resultBytes)
					}
				}
				a.StateManager.SetData("last_tool_result", resultStr)
			}

			toolResultsMsgs = append(toolResultsMsgs, models.Message{
				Role:    "tool",
				Content: resultStr,
				Name:    toolID,
			})
			if _, err := a.MemoryProvider.AddMessage(ctx, opts.ThreadID, "tool", resultStr, map[string]interface{}{
				"tool_call_id": toolID,
				"tool_name":    toolName,
			}); err != nil {
				a.StateManager.SetError(fmt.Sprintf("failed to add tool result message: %v", err))
				return "", fmt.Errorf("failed to add tool result message: %w", err)
			}
			a.StateManager.DeleteData("current_tool")
		} // End tool execution loop

		// Prepare data for step and step callback
		localToolCalls := make([]ToolCall, len(response.ToolCalls))
		for idx, tc := range response.ToolCalls {
			localToolCalls[idx] = ToolCall{
				ID:        tc.ID,
				ToolID:    tc.Function.Name,
				Arguments: tc.Function.Arguments,
			}
		}

		currentStep := Step{
			Text:        response.Text,   // The raw string response from the model for this step
			ToolCalls:   localToolCalls,  // Use converted local type
			ToolResults: toolResultsData, // Use the collected raw results/errors
		}
		steps = append(steps, currentStep)

		// Check if max consecutive calls reached *after* processing tool calls for this iteration
		if callCount == maxConsecutiveCalls-1 {
			log.Printf("Reached maximum function call attempts: %d", maxConsecutiveCalls)
			// Loop will terminate, finalResponse might still be empty
			break
		}
	}

	// If loop finished without a final response text (e.g., hit max calls),
	// return the last assistant response text we captured.
	if finalResponse == "" {
		finalResponse = lastAssistantResponseText
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

	// 将Agent消息转换为模型消息
	modelMessages := make([]models.Message, len(messages))
	for i, msg := range messages {
		modelMessages[i] = models.Message{
			Role:    msg.Role,
			Content: msg.Content,
			Name:    msg.Type, // 使用Type作为Name字段
		}
	}

	// 创建模型选项
	modelOptions := &models.GenerateOptions{
		Temperature: options.Temperature,
		MaxTokens:   a.MaxTokens,
	}

	// 创建通道
	textChan := make(chan string, 100)
	msgChan := make(chan Message, 5)
	finishChan := make(chan FinishInfo, 1)
	objectChan := make(chan interface{}, 5)

	// 启动新的goroutine处理流式响应
	go func() {
		defer close(textChan)
		defer close(msgChan)
		defer close(finishChan)
		defer close(objectChan)

		// 设置状态为运行中
		a.StateManager.SetRunning("", "")

		// 调用模型提供者的流式API
		stream, err := a.ModelProvider.Stream(options.AbortSignal, modelMessages, modelOptions)
		if err != nil {
			// 发送错误信息
			a.StateManager.SetError(fmt.Sprintf("streaming error: %v", err))
			return
		}

		// 收集完整响应
		fullResponse := strings.Builder{}

		// 处理流式响应
		for {
			select {
			case <-options.AbortSignal.Done():
				a.StateManager.SetIdle()
				return
			case chunk, ok := <-stream:
				if !ok {
					// 流已关闭，发送完整消息和完成信息
					responseText := fullResponse.String()

					msgChan <- Message{
						ID:        uuid.New().String(),
						Role:      "assistant",
						Content:   responseText,
						CreatedAt: time.Now().Unix(),
					}

					finishChan <- FinishInfo{
						FinishReason: "stop",
						Usage: Usage{
							PromptTokens:     100, // 实际应用中应从模型获取
							CompletionTokens: 50,  // 实际应用中应从模型获取
							TotalTokens:      150, // 实际应用中应从模型获取
						},
					}

					// 设置状态为空闲
					a.StateManager.SetIdle()
					return
				}

				// 发送文本块
				textChan <- chunk

				// 追加到完整响应
				fullResponse.WriteString(chunk)
			}
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

// 处理函数调用的方法
func (a *Agent) handleFunctionCall(ctx context.Context, functionCall *models.FunctionCall) (string, error) {
	// 解析函数调用参数
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(functionCall.Arguments), &args); err != nil {
		return "", fmt.Errorf("解析函数参数失败: %w", err)
	}

	// 查找工具
	tool, ok := a.Tools[functionCall.Name]
	if !ok {
		return "", fmt.Errorf("找不到工具: %s", functionCall.Name)
	}

	// 执行工具
	result, err := tool.Execute(args, nil)
	if err != nil {
		return fmt.Sprintf("执行失败: %s", err.Error()), nil
	}

	// 将结果转换为字符串
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("序列化结果失败: %w", err)
	}

	return string(resultJSON), nil
}

// 处理工具调用的方法
func (a *Agent) handleToolCall(ctx context.Context, toolCall models.ToolCall) (string, error) {
	return a.handleFunctionCall(ctx, &toolCall.Function)
}

// 生成带函数调用支持的响应
func (a *Agent) GenerateWithFunctionCalls(ctx context.Context, prompt string) (string, error) {
	// 准备历史消息
	messages := []models.Message{
		{
			Role:    "system",
			Content: a.SystemPrompt,
		},
	}

	// 添加历史消息
	if a.MemoryProvider != nil && a.ID != "" {
		historyMessages, err := a.MemoryProvider.GetMessages(ctx, a.ID, 10, 0)
		if err != nil {
			log.Printf("获取历史消息失败: %v", err)
		} else {
			for _, msg := range historyMessages {
				messages = append(messages, models.Message{
					Role:    msg.Role,
					Content: msg.Content,
				})
			}
		}
	}

	// 添加当前提示
	messages = append(messages, models.Message{
		Role:    "user",
		Content: prompt,
	})

	// 准备工具定义
	var tools []models.ToolDefinition
	if len(a.Tools) > 0 {
		tools = make([]models.ToolDefinition, 0, len(a.Tools))
		for name, tool := range a.Tools {
			schema, err := tool.GetInputSchema().JSONSchema()
			if err != nil {
				a.StateManager.SetError(fmt.Sprintf("failed to get tool schema: %v", err))
				return "", fmt.Errorf("failed to get tool schema: %w", err)
			}

			tools = append(tools, models.ToolDefinition{
				Type: "function",
				Function: models.FunctionDefinition{
					Name:        name,
					Description: tool.GetDescription(),
					Parameters:  schema,
				},
			})
		}
	}

	// 设置生成选项
	options := models.DefaultGenerateOptions()
	options.Tools = tools

	// 最大执行函数调用的次数
	maxFunctionCalls := 5
	functionCallCount := 0

	var finalResponse string

	for {
		// 防止无限循环
		if functionCallCount >= maxFunctionCalls {
			log.Printf("达到最大函数调用次数: %d", functionCallCount)
			return finalResponse, nil
		}

		// 生成响应
		response, err := a.ModelProvider.GenerateWithFunctionCalls(ctx, messages, options)
		if err != nil {
			return "", fmt.Errorf("生成响应失败: %w", err)
		}

		// 保存AI响应到消息历史
		messages = append(messages, models.Message{
			Role:         "assistant",
			Content:      response.Text,
			ToolCalls:    response.ToolCalls,
			FunctionCall: response.FunctionCall,
		})

		// 如果没有函数调用，直接返回文本响应
		if response.FunctionCall == nil && len(response.ToolCalls) == 0 {
			finalResponse = response.Text
			break
		}

		// 处理单个函数调用（旧版API兼容）
		if response.FunctionCall != nil {
			functionCallCount++
			log.Printf("执行函数调用: %s", response.FunctionCall.Name)

			// 执行函数并获取结果
			result, err := a.handleFunctionCall(ctx, response.FunctionCall)
			if err != nil {
				log.Printf("函数执行失败: %v", err)
				result = fmt.Sprintf("Error: %s", err.Error())
			}

			// 添加函数结果到消息历史
			messages = append(messages, models.Message{
				Role:    "function",
				Name:    response.FunctionCall.Name,
				Content: result,
			})

			// 继续生成循环
			continue
		}

		// 处理多个工具调用（新版API）
		if len(response.ToolCalls) > 0 {
			for _, toolCall := range response.ToolCalls {
				functionCallCount++
				log.Printf("执行工具调用: ID=%s, Name=%s", toolCall.ID, toolCall.Function.Name)

				// 执行工具并获取结果
				result, err := a.handleToolCall(ctx, toolCall)
				if err != nil {
					log.Printf("工具执行失败: %v", err)
					result = fmt.Sprintf("Error: %s", err.Error())
				}

				// 添加工具结果到消息历史
				messages = append(messages, models.Message{
					Role:    "tool",
					Content: result,
					Name:    toolCall.ID,
				})
			}

			// 继续生成循环
			continue
		}

		// 如果到达这里，说明没有函数调用了，使用最后的响应
		finalResponse = response.Text
		break
	}

	// 保存对话到内存（如果启用）
	if a.MemoryProvider != nil && a.ID != "" {
		// 保存用户消息
		_, err := a.MemoryProvider.AddMessage(ctx, a.ID, "user", prompt, nil)
		if err != nil {
			log.Printf("保存用户消息失败: %v", err)
		}

		// 保存AI响应
		_, err = a.MemoryProvider.AddMessage(ctx, a.ID, "assistant", finalResponse, nil)
		if err != nil {
			log.Printf("保存AI响应失败: %v", err)
		}
	}

	return finalResponse, nil
}

// StreamWithFunctionCalls 使用函数调用流式生成响应
func (a *Agent) StreamWithFunctionCalls(ctx context.Context, prompt string) (<-chan *models.ResponseChunk, error) {
	// 创建输出通道
	outputChan := make(chan *models.ResponseChunk, 100)

	// 启动goroutine处理流式生成
	go func() {
		defer close(outputChan)

		// 准备历史消息
		messages := []models.Message{
			{
				Role:    "system",
				Content: a.SystemPrompt,
			},
		}

		// 添加历史消息
		if a.MemoryProvider != nil && a.ID != "" {
			historyMessages, err := a.MemoryProvider.GetMessages(ctx, a.ID, 10, 0)
			if err != nil {
				log.Printf("获取历史消息失败: %v", err)
			} else {
				for _, msg := range historyMessages {
					messages = append(messages, models.Message{
						Role:    msg.Role,
						Content: msg.Content,
					})
				}
			}
		}

		// 添加当前提示
		messages = append(messages, models.Message{
			Role:    "user",
			Content: prompt,
		})

		// 准备工具定义
		var tools []models.ToolDefinition
		if len(a.Tools) > 0 {
			tools = make([]models.ToolDefinition, 0, len(a.Tools))
			for name, tool := range a.Tools {
				schema, err := tool.GetInputSchema().JSONSchema()
				if err != nil {
					log.Printf("获取工具参数架构失败: %v", err)
					continue
				}

				tools = append(tools, models.ToolDefinition{
					Type: "function",
					Function: models.FunctionDefinition{
						Name:        name,
						Description: tool.GetDescription(),
						Parameters:  schema,
					},
				})
			}
		}

		// 设置生成选项
		options := models.DefaultGenerateOptions()
		options.Tools = tools

		// 最大执行函数调用的次数
		maxFunctionCalls := 5
		functionCallCount := 0

		// 存储完整响应文本
		var fullResponse strings.Builder

		for {
			// 防止无限循环
			if functionCallCount >= maxFunctionCalls {
				log.Printf("达到最大函数调用次数: %d", functionCallCount)
				// 发送完成信号
				outputChan <- &models.ResponseChunk{
					IsFinished:   true,
					FinishReason: "max_function_calls",
				}

				// 保存对话到内存（如果启用）
				if a.MemoryProvider != nil && a.ID != "" {
					// 保存用户消息
					_, err := a.MemoryProvider.AddMessage(ctx, a.ID, "user", prompt, nil)
					if err != nil {
						log.Printf("保存用户消息失败: %v", err)
					}

					// 保存AI响应
					_, err = a.MemoryProvider.AddMessage(ctx, a.ID, "assistant", fullResponse.String(), nil)
					if err != nil {
						log.Printf("保存AI响应失败: %v", err)
					}
				}

				return
			}

			// 启动流式生成
			stream, err := a.ModelProvider.StreamWithFunctionCalls(ctx, messages, options)
			if err != nil {
				// 发送错误并结束
				outputChan <- &models.ResponseChunk{
					Text:         fmt.Sprintf("Error: %s", err.Error()),
					IsFinished:   true,
					FinishReason: "error",
				}
				return
			}

			// 用于构建当前消息
			var currentMessage strings.Builder
			var currentToolCalls []models.ToolCall
			var currentFunctionCall *models.FunctionCall

			// 处理流式响应
			for chunk := range stream {
				// 检查上下文是否已取消
				select {
				case <-ctx.Done():
					// 发送取消信号并结束
					outputChan <- &models.ResponseChunk{
						IsFinished:   true,
						FinishReason: "canceled",
					}
					return
				default:
					// 继续处理
				}

				// 累积文本响应
				if chunk.Text != "" {
					currentMessage.WriteString(chunk.Text)
					fullResponse.WriteString(chunk.Text)
				}

				// 处理工具调用块
				if chunk.ToolCallChunk != nil {
					// 查找现有工具调用或创建新的
					found := false
					for i, tc := range currentToolCalls {
						if tc.ID == chunk.ToolCallChunk.ID {
							// 更新现有工具调用
							if chunk.ToolCallChunk.Function != nil {
								if chunk.ToolCallChunk.Function.Name != "" {
									currentToolCalls[i].Function.Name = chunk.ToolCallChunk.Function.Name
								}
								if chunk.ToolCallChunk.Function.Arguments != "" {
									currentToolCalls[i].Function.Arguments += chunk.ToolCallChunk.Function.Arguments
								}
							}
							found = true
							break
						}
					}

					if !found && chunk.ToolCallChunk.ID != "" {
						// 创建新的工具调用
						newToolCall := models.ToolCall{
							ID:   chunk.ToolCallChunk.ID,
							Type: chunk.ToolCallChunk.Type,
							Function: models.FunctionCall{
								Name:      "",
								Arguments: "",
							},
						}

						if chunk.ToolCallChunk.Function != nil {
							if chunk.ToolCallChunk.Function.Name != "" {
								newToolCall.Function.Name = chunk.ToolCallChunk.Function.Name
							}
							if chunk.ToolCallChunk.Function.Arguments != "" {
								newToolCall.Function.Arguments = chunk.ToolCallChunk.Function.Arguments
							}
						}

						currentToolCalls = append(currentToolCalls, newToolCall)
					}
				}

				// 处理函数调用块（旧版API）
				if chunk.FunctionCallChunk != nil {
					if currentFunctionCall == nil {
						currentFunctionCall = &models.FunctionCall{
							Name:      chunk.FunctionCallChunk.Name,
							Arguments: chunk.FunctionCallChunk.Arguments,
						}
					} else {
						if chunk.FunctionCallChunk.Name != "" {
							currentFunctionCall.Name = chunk.FunctionCallChunk.Name
						}
						if chunk.FunctionCallChunk.Arguments != "" {
							currentFunctionCall.Arguments += chunk.FunctionCallChunk.Arguments
						}
					}
				}

				// 转发块到输出通道
				outputChan <- chunk

				// 检查是否完成
				if chunk.IsFinished {
					break
				}
			}

			// 保存助手消息到历史
			assistantMessage := models.Message{
				Role:         "assistant",
				Content:      currentMessage.String(),
				ToolCalls:    currentToolCalls,
				FunctionCall: currentFunctionCall,
			}
			messages = append(messages, assistantMessage)

			// 如果没有函数调用或工具调用，则结束
			if currentFunctionCall == nil && len(currentToolCalls) == 0 {
				// 保存对话到内存（如果启用）
				if a.MemoryProvider != nil && a.ID != "" {
					// 保存用户消息
					_, err := a.MemoryProvider.AddMessage(ctx, a.ID, "user", prompt, nil)
					if err != nil {
						log.Printf("保存用户消息失败: %v", err)
					}

					// 保存AI响应
					_, err = a.MemoryProvider.AddMessage(ctx, a.ID, "assistant", fullResponse.String(), nil)
					if err != nil {
						log.Printf("保存AI响应失败: %v", err)
					}
				}

				// 完成
				return
			}

			// 处理函数调用（旧版API）
			if currentFunctionCall != nil {
				functionCallCount++
				log.Printf("执行函数调用: %s", currentFunctionCall.Name)

				// 发送函数调用开始事件
				outputChan <- &models.ResponseChunk{
					Text: fmt.Sprintf("\n[执行函数: %s]\n", currentFunctionCall.Name),
				}

				// 执行函数并获取结果
				result, err := a.handleFunctionCall(ctx, currentFunctionCall)
				if err != nil {
					log.Printf("函数执行失败: %v", err)
					result = fmt.Sprintf("Error: %s", err.Error())

					// 发送错误消息
					outputChan <- &models.ResponseChunk{
						Text: fmt.Sprintf("[函数执行失败: %s]\n", err.Error()),
					}
				} else {
					// 发送函数结果
					outputChan <- &models.ResponseChunk{
						Text: fmt.Sprintf("[函数结果: %s]\n", result),
					}
				}

				// 添加函数结果到消息历史
				messages = append(messages, models.Message{
					Role:    "function",
					Name:    currentFunctionCall.Name,
					Content: result,
				})

				// 继续生成循环
				continue
			}

			// 处理工具调用（新版API）
			if len(currentToolCalls) > 0 {
				for _, toolCall := range currentToolCalls {
					functionCallCount++
					log.Printf("执行工具调用: ID=%s, Name=%s", toolCall.ID, toolCall.Function.Name)

					// 发送工具调用开始事件
					outputChan <- &models.ResponseChunk{
						Text: fmt.Sprintf("\n[执行工具: %s (ID: %s)]\n", toolCall.Function.Name, toolCall.ID),
					}

					// 执行工具并获取结果
					result, err := a.handleToolCall(ctx, toolCall)
					if err != nil {
						log.Printf("工具执行失败: %v", err)
						result = fmt.Sprintf("Error: %s", err.Error())

						// 发送错误消息
						outputChan <- &models.ResponseChunk{
							Text: fmt.Sprintf("[工具执行失败: %s]\n", err.Error()),
						}
					} else {
						// 发送工具结果
						outputChan <- &models.ResponseChunk{
							Text: fmt.Sprintf("[工具结果: %s]\n", result),
						}
					}

					// 添加工具结果到消息历史
					messages = append(messages, models.Message{
						Role:    "tool",
						Content: result,
						Name:    toolCall.ID,
					})
				}

				// 继续生成循环
				continue
			}
		}
	}()

	return outputChan, nil
}

// StepFinishData 包含步骤完成时的数据
type StepFinishData struct {
	Text        string        `json:"text"`
	ToolCalls   []ToolCall    `json:"tool_calls,omitempty"`
	ToolResults []interface{} `json:"tool_results,omitempty"`
	StepIndex   int           `json:"step_index"`
	TotalSteps  int           `json:"total_steps"`
}

// FinishData 包含执行完成时的数据
type FinishData struct {
	Text          string `json:"text"`
	Steps         []Step `json:"steps,omitempty"`
	FinishReason  string `json:"finish_reason"`
	Usage         Usage  `json:"usage"`
	ToolCallCount int    `json:"tool_call_count"`
}

// 在Run方法中添加回调函数支持
func (a *Agent) RunWithCallbacks(ctx context.Context, opts *RunOptions,
	onStepFinish func(*StepFinishData),
	onFinish func(*FinishData)) (string, error) {
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
	var steps []Step
	toolCallCount := 0
	var lastAssistantResponseText string // Store the last assistant response text

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

		// 调用模型生成响应 (Using GenerateWithFunctionCalls)
		response, err := a.ModelProvider.GenerateWithFunctionCalls(ctx, messages, genOpts)
		if err != nil {
			a.StateManager.SetError(fmt.Sprintf("model generation failed: %v", err))
			return "", fmt.Errorf("model generation failed: %w", err)
		}

		lastAssistantResponseText = response.Text // Capture the latest response text from the struct

		// 检查是否有工具调用 (Check FinishReason and ToolCalls from struct)
		hasToolCalls := len(response.ToolCalls) > 0
		isFinished := response.FinishReason != "tool_calls" && response.FinishReason != "function_call"

		if !hasToolCalls && isFinished {
			// 如果没有工具调用且模型完成，保存最终响应并返回
			if _, err := a.MemoryProvider.AddMessage(ctx, opts.ThreadID, "assistant", response.Text, nil); err != nil {
				a.StateManager.SetError(fmt.Sprintf("failed to add assistant message: %v", err))
				return "", fmt.Errorf("failed to add assistant message: %w", err)
			}
			finalResponse = response.Text
			break // Exit loop, normal finish
		}

		// 有工具调用或模型未完成，保存助手消息
		assistantMetadata := map[string]interface{}{
			"has_tool_calls": hasToolCalls,
			"tool_calls":     response.ToolCalls,
			"finish_reason":  response.FinishReason,
		}
		if _, err := a.MemoryProvider.AddMessage(ctx, opts.ThreadID, "assistant", response.Text, assistantMetadata); err != nil {
			a.StateManager.SetError(fmt.Sprintf("failed to add assistant message with tool calls: %v", err))
			return "", fmt.Errorf("failed to add assistant message with tool calls: %w", err)
		}

		// 如果没有工具调用但模型要求继续 (e.g., maybe length limit), continue loop
		if !hasToolCalls {
			continue
		}

		// 执行工具调用
		// Declare slices to hold results for this step
		toolResultsData := make([]interface{}, len(response.ToolCalls))
		toolResultsMsgs := make([]models.Message, 0, len(response.ToolCalls))

		for i, toolCall := range response.ToolCalls { // Use ToolCalls from struct
			toolID := toolCall.ID
			toolName := toolCall.Function.Name
			argsStr := toolCall.Function.Arguments
			toolCallCount++

			// 记录当前工具调用
			a.StateManager.SetData("current_tool", toolName)

			// 解析参数
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(argsStr), &args); err != nil {
				log.Printf("Failed to parse tool arguments: %v", err)
				toolResultStr := fmt.Sprintf("Error: Failed to parse tool arguments - %v", err)
				toolResultsData[i] = toolResultStr // Store error string in data slice
				toolResultsMsgs = append(toolResultsMsgs, models.Message{
					Role:    "tool",
					Content: toolResultStr,
					Name:    toolID,
				})
				if _, err := a.MemoryProvider.AddMessage(ctx, opts.ThreadID, "tool", toolResultStr, map[string]interface{}{
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
				toolResultStr := fmt.Sprintf("Error: Tool not found - %s", toolName)
				toolResultsData[i] = toolResultStr // Store error string in data slice
				toolResultsMsgs = append(toolResultsMsgs, models.Message{
					Role:    "tool",
					Content: toolResultStr,
					Name:    toolID,
				})
				if _, err := a.MemoryProvider.AddMessage(ctx, opts.ThreadID, "tool", toolResultStr, map[string]interface{}{
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

			toolResultsData[i] = result // Store raw result

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
					resultBytes, marshalErr := json.Marshal(result)
					if marshalErr != nil {
						resultStr = fmt.Sprintf("%+v", result)
					} else {
						resultStr = string(resultBytes)
					}
				}
				a.StateManager.SetData("last_tool_result", resultStr)
			}

			toolResultsMsgs = append(toolResultsMsgs, models.Message{
				Role:    "tool",
				Content: resultStr,
				Name:    toolID,
			})
			if _, err := a.MemoryProvider.AddMessage(ctx, opts.ThreadID, "tool", resultStr, map[string]interface{}{
				"tool_call_id": toolID,
				"tool_name":    toolName,
			}); err != nil {
				a.StateManager.SetError(fmt.Sprintf("failed to add tool result message: %v", err))
				return "", fmt.Errorf("failed to add tool result message: %w", err)
			}
			a.StateManager.DeleteData("current_tool")
		} // End tool execution loop

		// Prepare data for step and step callback
		localToolCalls := make([]ToolCall, len(response.ToolCalls))
		for idx, tc := range response.ToolCalls {
			localToolCalls[idx] = ToolCall{
				ID:        tc.ID,
				ToolID:    tc.Function.Name,
				Arguments: tc.Function.Arguments,
			}
		}

		currentStep := Step{
			Text:        response.Text,   // The raw string response from the model for this step
			ToolCalls:   localToolCalls,  // Use converted local type
			ToolResults: toolResultsData, // Use the collected raw results/errors
		}
		steps = append(steps, currentStep)

		if onStepFinish != nil {
			stepData := &StepFinishData{
				Text:        response.Text,   // Pass the raw string response
				ToolCalls:   localToolCalls,  // Pass the converted local type
				ToolResults: toolResultsData, // Pass the collected raw results/errors from this step
				StepIndex:   callCount,
				TotalSteps:  -1,
			}
			onStepFinish(stepData)
		}

		// Check if max consecutive calls reached
		if callCount == maxConsecutiveCalls-1 {
			log.Printf("Reached maximum function call attempts: %d", maxConsecutiveCalls)
			// Loop will terminate, finalResponse might still be empty
			break
		}
	}

	// If loop finished without a final response text (e.g., hit max calls),
	// return the last assistant response text we captured.
	if finalResponse == "" {
		finalResponse = lastAssistantResponseText
	}

	// 任务完成，更新状态
	a.StateManager.CompleteTask(taskID)
	a.StateManager.SetIdle()

	// 调用完成回调
	if onFinish != nil {
		finishData := &FinishData{
			Text:          finalResponse,
			Steps:         steps,
			FinishReason:  "stop",
			ToolCallCount: toolCallCount,
			Usage: Usage{
				PromptTokens:     100, // 这里应该从模型获取实际使用情况
				CompletionTokens: 50,  // 这里应该从模型获取实际使用情况
				TotalTokens:      150, // 这里应该从模型获取实际使用情况
			},
		}
		onFinish(finishData)
	}

	return finalResponse, nil
}

// StreamWithCallbacks 实现带回调的流式响应
func (a *Agent) StreamWithCallbacks(messages []Message, options *StreamOptions,
	onStepFinish func(*StepFinishData),
	onFinish func(*FinishData)) (*StreamResponse, error) {
	if len(messages) == 0 {
		return nil, errors.New("empty messages")
	}

	if options == nil {
		options = &StreamOptions{
			MaxSteps:    1,
			Temperature: 0.7,
		}
	}

	// 将Agent消息转换为模型消息
	modelMessages := make([]models.Message, len(messages))
	for i, msg := range messages {
		modelMessages[i] = models.Message{
			Role:    msg.Role,
			Content: msg.Content,
			Name:    msg.Type, // 使用Type作为Name字段
		}
	}

	// 创建模型选项
	modelOptions := &models.GenerateOptions{
		Temperature: options.Temperature,
		MaxTokens:   a.MaxTokens,
	}

	// 创建通道
	textChan := make(chan string, 100)
	msgChan := make(chan Message, 5)
	finishChan := make(chan FinishInfo, 1)
	objectChan := make(chan interface{}, 5)

	// 启动新的goroutine处理流式响应
	go func() {
		defer close(textChan)
		defer close(msgChan)
		defer close(finishChan)
		defer close(objectChan)

		// 设置状态为运行中
		a.StateManager.SetRunning("", "")

		// 调用模型提供者的流式API
		stream, err := a.ModelProvider.Stream(options.AbortSignal, modelMessages, modelOptions)
		if err != nil {
			// 发送错误信息
			a.StateManager.SetError(fmt.Sprintf("streaming error: %v", err))

			// 调用完成回调
			if onFinish != nil {
				finishData := &FinishData{
					Text:         fmt.Sprintf("Error: %v", err),
					FinishReason: "error",
					Usage: Usage{
						PromptTokens:     0,
						CompletionTokens: 0,
						TotalTokens:      0,
					},
				}
				onFinish(finishData)
			}
			return
		}

		// 收集完整响应
		fullResponse := strings.Builder{}
		stepIndex := 0

		// 处理流式响应
		for {
			select {
			case <-options.AbortSignal.Done():
				a.StateManager.SetIdle()

				// 调用完成回调
				if onFinish != nil {
					finishData := &FinishData{
						Text:         fullResponse.String(),
						FinishReason: "canceled",
						Usage: Usage{
							PromptTokens:     100,
							CompletionTokens: stepIndex * 10,
							TotalTokens:      100 + stepIndex*10,
						},
					}
					onFinish(finishData)
				}
				return
			case chunk, ok := <-stream:
				if !ok {
					// 流已关闭，发送完整消息和完成信息
					responseText := fullResponse.String()

					msgChan <- Message{
						ID:        uuid.New().String(),
						Role:      "assistant",
						Content:   responseText,
						CreatedAt: time.Now().Unix(),
					}

					// 创建完成信息
					finishInfo := FinishInfo{
						FinishReason: "stop",
						Usage: Usage{
							PromptTokens:     100, // 实际应用中应从模型获取
							CompletionTokens: 50,  // 实际应用中应从模型获取
							TotalTokens:      150, // 实际应用中应从模型获取
						},
					}

					finishChan <- finishInfo

					// 调用完成回调
					if onFinish != nil {
						finishData := &FinishData{
							Text:         responseText,
							FinishReason: "stop",
							Usage:        finishInfo.Usage,
						}
						onFinish(finishData)
					}

					// 设置状态为空闲
					a.StateManager.SetIdle()
					return
				}

				// 发送文本块
				textChan <- chunk

				// 追加到完整响应
				fullResponse.WriteString(chunk)

				// 调用步骤完成回调
				if onStepFinish != nil {
					stepData := &StepFinishData{
						Text:       chunk,
						StepIndex:  stepIndex,
						TotalSteps: -1, // 流式响应无法预知总步骤数
					}
					onStepFinish(stepData)
					stepIndex++
				}
			}
		}
	}()

	return &StreamResponse{
		TextStream:  textChan,
		MessageChan: msgChan,
		FinishChan:  finishChan,
		ObjectChan:  objectChan,
	}, nil
}
