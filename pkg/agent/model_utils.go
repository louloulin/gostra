package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/louloulin/gostra/pkg/memory"
	"github.com/louloulin/gostra/pkg/models"
	"github.com/louloulin/gostra/pkg/tools"
)

// RunOptions 是运行Agent的选项
type RunOptions struct {
	ThreadID            string
	Input               string
	AvailableTools      []tools.Tool
	MaxConsecutiveCalls int
	MaxTokens           int
}

// FunctionDefinition 表示工具的函数定义
type FunctionDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// 获取工具的函数定义
func convertToolsToFunctionDefinitions(toolsList []tools.Tool) []FunctionDefinition {
	if len(toolsList) == 0 {
		return nil
	}

	defs := make([]FunctionDefinition, 0, len(toolsList))

	for _, tool := range toolsList {
		if tool == nil {
			continue
		}

		// 获取JSON模式并处理错误
		schema, err := tool.GetInputSchema().JSONSchema()
		if err != nil {
			// 如果无法获取模式，则使用空模式
			schema = map[string]interface{}{}
		}

		def := FunctionDefinition{
			Name:        tool.GetID(),
			Description: tool.GetDescription(),
			Parameters:  schema,
		}

		defs = append(defs, def)
	}

	return defs
}

// 构建模型消息
func buildModelMessages(ctx context.Context, memProvider memory.MemoryProvider, threadID string, systemPrompt string) ([]models.Message, error) {
	if threadID == "" {
		return nil, errors.New("threadID is required")
	}

	// 获取线程中的消息
	msgs, err := memProvider.GetMessages(ctx, threadID, 100, 0) // 获取最近100条消息
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	// 创建模型消息
	modelMsgs := make([]models.Message, 0, len(msgs)+1)

	// 添加系统消息
	if systemPrompt != "" {
		modelMsgs = append(modelMsgs, models.Message{
			Role:    "system",
			Content: systemPrompt,
		})
	}

	// 添加线程消息
	for _, msg := range msgs {
		modelMsgs = append(modelMsgs, models.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	return modelMsgs, nil
}

// 解析工具调用
func parseToolCall(response string) (toolName string, params map[string]interface{}, err error) {
	// 简单实现，根据实际模型输出格式可能需要调整
	toolCallStart := strings.Index(response, "```tool_call")
	if toolCallStart == -1 {
		return "", nil, nil // 没有工具调用
	}

	toolCallEnd := strings.Index(response[toolCallStart:], "```")
	if toolCallEnd == -1 {
		return "", nil, errors.New("invalid tool call format: no closing ```")
	}

	toolCallEnd = toolCallStart + toolCallEnd + 3
	toolCallContent := response[toolCallStart+13 : toolCallEnd-3]

	// 解析工具名称和参数
	lines := strings.Split(toolCallContent, "\n")
	if len(lines) < 2 {
		return "", nil, errors.New("invalid tool call format: insufficient content")
	}

	toolName = strings.TrimSpace(lines[0])

	// 简单解析参数，实际应用中可能需要更复杂的JSON解析
	params = make(map[string]interface{})
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			params[key] = value
		}
	}

	return toolName, params, nil
}

// 执行工具调用
func executeToolCall(ctx context.Context, toolName string, params map[string]interface{}, availableTools []tools.Tool) (interface{}, error) {
	for _, tool := range availableTools {
		if tool.GetID() == toolName {
			// 执行工具，使用正确的参数格式
			options := &tools.ExecuteOptions{
				ThreadID:   "", // 可以在这里添加线程ID
				ResourceID: "", // 可以在这里添加资源ID
				CallID:     "", // 可以在这里添加调用ID
				Context:    ctx,
			}
			return tool.Execute(params, options)
		}
	}

	return nil, fmt.Errorf("tool not found: %s", toolName)
}

// 将工具结果格式化为消息
func formatToolResultAsMessage(toolName string, result interface{}, err error) string {
	var resultStr string

	if err != nil {
		resultStr = fmt.Sprintf("Error: %s", err.Error())
	} else {
		// 将结果转换为字符串，根据实际情况可能需要调整
		resultStr = fmt.Sprintf("%v", result)
	}

	return fmt.Sprintf(
		"Tool: %s\nResult: %s",
		toolName,
		resultStr,
	)
}
