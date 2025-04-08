package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourusername/gostra/pkg/agent"
	"github.com/yourusername/gostra/pkg/memory"
	"github.com/yourusername/gostra/pkg/models/openai"
	"github.com/yourusername/gostra/pkg/tools"
)

// 天气工具的Schema
type WeatherSchema struct{}

func (s *WeatherSchema) Validate(params map[string]interface{}) error {
	if _, ok := params["location"]; !ok {
		return tools.NewValidationError("缺少必需参数: location")
	}
	return nil
}

func (s *WeatherSchema) JSONSchema() (map[string]interface{}, error) {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"location": map[string]interface{}{
				"type":        "string",
				"description": "城市名称，例如 '北京'、'上海'、'广州'",
			},
		},
		"required": []string{"location"},
	}, nil
}

// 创建天气工具
func createWeatherTool() *tools.BasicTool {
	return tools.NewBasicTool(
		"get_weather",
		"获取指定城市的天气信息",
		&WeatherSchema{},
		func(params map[string]interface{}, options *tools.ExecuteOptions) (interface{}, error) {
			location, _ := params["location"].(string)
			// 这里实际应用中应该调用真实的天气API
			// 这里只是模拟返回数据
			weather := map[string]interface{}{
				"location":    location,
				"temperature": 23.5,
				"condition":   "晴朗",
				"humidity":    45,
				"wind":        "东北风3-4级",
				"updated_at":  time.Now().Format(time.RFC3339),
			}
			return weather, nil
		},
	)
}

// 计算工具的Schema
type CalculatorSchema struct{}

func (s *CalculatorSchema) Validate(params map[string]interface{}) error {
	if _, ok := params["expression"]; !ok {
		return tools.NewValidationError("缺少必需参数: expression")
	}
	return nil
}

func (s *CalculatorSchema) JSONSchema() (map[string]interface{}, error) {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"expression": map[string]interface{}{
				"type":        "string",
				"description": "数学表达式，例如 '1 + 2' 或 '5 * 10'",
			},
		},
		"required": []string{"expression"},
	}, nil
}

// 创建计算工具
func createCalculatorTool() *tools.BasicTool {
	return tools.NewBasicTool(
		"calculate",
		"计算简单的数学表达式",
		&CalculatorSchema{},
		func(params map[string]interface{}, options *tools.ExecuteOptions) (interface{}, error) {
			expression, _ := params["expression"].(string)
			// 这里简化处理，实际应用中应该使用表达式解析库
			// 仅支持简单的加减乘除
			var result float64
			var operation string
			var operands []float64

			if expression == "1 + 2" {
				result = 3
				operation = "+"
				operands = []float64{1, 2}
			} else if expression == "5 * 10" {
				result = 50
				operation = "*"
				operands = []float64{5, 10}
			} else {
				result = 42 // 默认答案 :)
				operation = "默认"
				operands = []float64{42}
			}

			return map[string]interface{}{
				"expression": expression,
				"result":     result,
				"operation":  operation,
				"operands":   operands,
			}, nil
		},
	)
}

func main() {
	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 设置信号处理
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChan
		log.Println("接收到终止信号，正在关闭...")
		cancel()
		os.Exit(0)
	}()

	// 获取OpenAI API密钥
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatalln("请设置OPENAI_API_KEY环境变量")
	}

	// 创建OpenAI模型提供者
	openaiProvider, err := openai.NewOpenAIProvider(&openai.Options{
		APIKey: apiKey,
		Model:  "gpt-3.5-turbo",
	})
	if err != nil {
		log.Fatalf("创建OpenAI提供者失败: %v", err)
	}

	// 创建内存提供者
	memoryProvider := memory.NewInMemoryProvider()

	// 创建工具
	weatherTool := createWeatherTool()
	calculatorTool := createCalculatorTool()

	// 创建Agent
	agent, err := agent.NewAgent(&agent.Options{
		Name:           "函数调用助手",
		SystemPrompt:   "你是一个有用的助手，可以使用工具来回答用户的问题。当需要获取天气信息或计算数学表达式时，请使用相应的工具。",
		ModelProvider:  openaiProvider,
		MemoryProvider: memoryProvider,
		Tools:          []tools.Tool{weatherTool, calculatorTool},
	})
	if err != nil {
		log.Fatalf("创建Agent失败: %v", err)
	}

	// 创建线程
	thread, err := memoryProvider.CreateThread(ctx, nil)
	if err != nil {
		log.Fatalf("创建线程失败: %v", err)
	}

	fmt.Println("=== 函数调用示例 ===")
	fmt.Println("1. 简单对话")

	// 运行简单对话
	response, err := agent.Run(ctx, &struct {
		ThreadID            string
		Input               string
		AvailableTools      []tools.Tool
		MaxConsecutiveCalls int
	}{
		ThreadID:            thread.ID,
		Input:               "你好！",
		MaxConsecutiveCalls: 10,
	})
	if err != nil {
		log.Fatalf("执行失败: %v", err)
	}
	fmt.Printf("助手: %s\n\n", response)

	fmt.Println("2. 触发天气工具")
	response, err = agent.Run(ctx, &struct {
		ThreadID            string
		Input               string
		AvailableTools      []tools.Tool
		MaxConsecutiveCalls int
	}{
		ThreadID:            thread.ID,
		Input:               "北京今天天气怎么样？",
		MaxConsecutiveCalls: 10,
	})
	if err != nil {
		log.Fatalf("执行失败: %v", err)
	}
	fmt.Printf("助手: %s\n\n", response)

	fmt.Println("3. 触发计算工具")
	response, err = agent.Run(ctx, &struct {
		ThreadID            string
		Input               string
		AvailableTools      []tools.Tool
		MaxConsecutiveCalls int
	}{
		ThreadID:            thread.ID,
		Input:               "计算 5 * 10 是多少？",
		MaxConsecutiveCalls: 10,
	})
	if err != nil {
		log.Fatalf("执行失败: %v", err)
	}
	fmt.Printf("助手: %s\n\n", response)

	fmt.Println("4. 同时使用多个工具")
	response, err = agent.Run(ctx, &struct {
		ThreadID            string
		Input               string
		AvailableTools      []tools.Tool
		MaxConsecutiveCalls int
	}{
		ThreadID:            thread.ID,
		Input:               "上海的天气如何？另外，1 + 2 等于多少？",
		MaxConsecutiveCalls: 10,
	})
	if err != nil {
		log.Fatalf("执行失败: %v", err)
	}
	fmt.Printf("助手: %s\n\n", response)

	// 展示历史消息
	fmt.Println("=== 对话历史 ===")
	messages, err := memoryProvider.GetMessages(ctx, thread.ID, 100, 0)
	if err != nil {
		log.Fatalf("获取历史消息失败: %v", err)
	}

	for i, msg := range messages {
		switch msg.Role {
		case "user":
			fmt.Printf("用户: %s\n", msg.Content)
		case "assistant":
			fmt.Printf("助手: %s\n", msg.Content)
		case "tool":
			toolName := "未知工具"
			toolCallID := "未知ID"
			if msg.Metadata != nil {
				if name, ok := msg.Metadata["tool_name"].(string); ok {
					toolName = name
				}
				if id, ok := msg.Metadata["tool_call_id"].(string); ok {
					toolCallID = id
				}
			}
			fmt.Printf("工具(%s, ID: %s): %s\n", toolName, toolCallID, msg.Content)

			// 如果是JSON格式，美化输出
			var jsonData map[string]interface{}
			if err := json.Unmarshal([]byte(msg.Content), &jsonData); err == nil {
				jsonBytes, _ := json.MarshalIndent(jsonData, "", "  ")
				fmt.Printf("格式化结果:\n%s\n", string(jsonBytes))
			}
		}
		if i < len(messages)-1 {
			fmt.Println("---")
		}
	}

	fmt.Println("\n=== 流式函数调用示例 ===")

	// 创建流式函数调用上下文
	streamCtx, streamCancel := context.WithTimeout(ctx, 30*time.Second)
	defer streamCancel()

	// 获取函数调用流
	fmt.Println("问题: 广州的天气如何？请同时计算 5 * 10 的结果。")
	stream, err := agent.StreamWithFunctionCalls(streamCtx, "广州的天气如何？请同时计算 5 * 10 的结果。")
	if err != nil {
		log.Fatalf("创建流式调用失败: %v", err)
	}

	fmt.Println("助手: ")
	for chunk := range stream {
		if chunk.Text != "" {
			fmt.Print(chunk.Text)
		}

		if chunk.ToolCallChunk != nil {
			if chunk.ToolCallChunk.Function != nil && chunk.ToolCallChunk.Function.Name != "" {
				fmt.Printf("\n[调用工具: %s]\n", chunk.ToolCallChunk.Function.Name)
			}
		}

		if chunk.IsFinished {
			fmt.Printf("\n[完成，原因: %s]\n", chunk.FinishReason)
		}
	}

	fmt.Println("\n=== 示例结束 ===")
}
