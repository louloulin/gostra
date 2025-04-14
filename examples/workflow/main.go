package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/louloulin/gostra/pkg/agent"
	"github.com/louloulin/gostra/pkg/memory"
	"github.com/louloulin/gostra/pkg/models"
	"github.com/louloulin/gostra/pkg/workflow"
)

// 简单的模型提供者
type SimpleModelProvider struct{}

func (p *SimpleModelProvider) GetID() string {
	return "simple_model"
}

func (p *SimpleModelProvider) GetProvider() string {
	return "simple"
}

func (p *SimpleModelProvider) Generate(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (string, error) {
	// 简单模拟LLM响应
	return fmt.Sprintf("Response to %d messages", len(messages)), nil
}

func (p *SimpleModelProvider) Stream(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (<-chan string, error) {
	// 简单模拟流式响应
	ch := make(chan string, 3)

	go func() {
		defer close(ch)
		ch <- "This "
		time.Sleep(100 * time.Millisecond)
		ch <- "is a "
		time.Sleep(100 * time.Millisecond)
		ch <- "streaming response"
	}()

	return ch, nil
}

// GenerateWithFunctionCalls implements the models.ModelProvider interface for function calls
func (p *SimpleModelProvider) GenerateWithFunctionCalls(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (*models.ResponseWithFunctionCalls, error) {
	// 简单模拟LLM响应，不支持函数调用
	text, err := p.Generate(ctx, messages, options)
	if err != nil {
		return nil, err
	}
	return &models.ResponseWithFunctionCalls{
		Text:         text,
		FinishReason: "stop", // 模拟停止原因
	}, nil
}

// StreamWithFunctionCalls implements the models.ModelProvider interface for streaming with function calls
func (p *SimpleModelProvider) StreamWithFunctionCalls(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (<-chan *models.ResponseChunk, error) {
	// 简单模拟流式响应，不支持函数调用
	ch := make(chan *models.ResponseChunk, 1)

	go func() {
		defer close(ch)
		text, err := p.Generate(ctx, messages, options) // 获取模拟的文本
		if err != nil {
			// 可以发送一个错误块，或者只记录错误
			log.Printf("Error generating text for stream: %v", err)
			return
		}
		ch <- &models.ResponseChunk{
			Text:       text,
			IsFinished: true,
		}
	}()

	return ch, nil
}

func main() {
	log.Println("Starting workflow example...")

	// 创建Actor系统
	system := actor.NewActorSystem()

	// 创建内存提供者
	memoryProvider := memory.NewInMemoryProvider()

	// 创建模型提供者
	modelProvider := &SimpleModelProvider{}

	// 创建两个Agent
	analysisAgent, err := agent.NewAgent(&agent.Options{
		ID:             "analysis_agent",
		Name:           "Analysis Agent",
		SystemPrompt:   "You are an analysis agent that analyzes and extracts key information.",
		ModelProvider:  modelProvider,
		MemoryProvider: memoryProvider,
	})
	if err != nil {
		log.Fatalf("Failed to create analysis agent: %v", err)
	}

	// 初始化Actor Agent，确保actors被正确启动
	analysisProps, err := agent.NewActorAgent(&agent.ActorAgentOptions{
		ID:             analysisAgent.ID,
		Name:           analysisAgent.Name,
		SystemPrompt:   analysisAgent.SystemPrompt,
		ModelProvider:  modelProvider,
		MemoryProvider: memoryProvider,
		ActorSystem:    system,
	})
	if err != nil {
		log.Fatalf("Failed to create actor agent for analysis agent: %v", err)
	}

	rootCtx := actor.NewRootContext(system, nil)
	analysisPID := rootCtx.Spawn(analysisProps)
	log.Printf("Analysis agent started with PID: %v", analysisPID)

	summaryAgent, err := agent.NewAgent(&agent.Options{
		ID:             "summary_agent",
		Name:           "Summary Agent",
		SystemPrompt:   "You are a summary agent that creates concise summaries.",
		ModelProvider:  modelProvider,
		MemoryProvider: memoryProvider,
	})
	if err != nil {
		log.Fatalf("Failed to create summary agent: %v", err)
	}

	// 初始化Actor Agent，确保actors被正确启动
	summaryProps, err := agent.NewActorAgent(&agent.ActorAgentOptions{
		ID:             summaryAgent.ID,
		Name:           summaryAgent.Name,
		SystemPrompt:   summaryAgent.SystemPrompt,
		ModelProvider:  modelProvider,
		MemoryProvider: memoryProvider,
		ActorSystem:    system,
	})
	if err != nil {
		log.Fatalf("Failed to create actor agent for summary agent: %v", err)
	}

	summaryPID := rootCtx.Spawn(summaryProps)
	log.Printf("Summary agent started with PID: %v", summaryPID)

	// 创建工作流步骤

	// 1. 数据收集步骤
	collectStep := &workflow.Step{
		ID:   "collect_data",
		Name: "Collect Data",
		Execute: func(ctx context.Context, input map[string]interface{}, w *workflow.Workflow) (map[string]interface{}, error) {
			log.Println("Collecting data...")

			// 模拟数据收集
			time.Sleep(500 * time.Millisecond)

			// 返回收集的数据
			return map[string]interface{}{
				"data": "This is some sample text data that needs to be analyzed and summarized.",
			}, nil
		},
		Next: []string{"analyze_data"},
	}

	// 2. 数据分析步骤（使用分析Agent）
	analyzeStep := &workflow.Step{
		ID:   "analyze_data",
		Name: "Analyze Data",
		Execute: func(ctx context.Context, input map[string]interface{}, w *workflow.Workflow) (map[string]interface{}, error) {
			log.Println("Analyzing data...")

			// 获取输入数据
			data, ok := input["data"].(string)
			if !ok {
				return nil, fmt.Errorf("invalid data format")
			}

			// 使用分析Agent处理数据
			resp, err := analysisAgent.Generate([]agent.Message{
				{
					Role:    "user",
					Content: fmt.Sprintf("Analyze the following text and extract key information: %s", data),
				},
			}, &agent.GenerateOptions{})

			if err != nil {
				return nil, fmt.Errorf("analysis failed: %w", err)
			}

			// 返回分析结果
			return map[string]interface{}{
				"data":     data,
				"analysis": resp.Text,
			}, nil
		},
		Next: []string{"summarize_data"},
	}

	// 3. 数据总结步骤（使用总结Agent）
	summarizeStep := &workflow.Step{
		ID:   "summarize_data",
		Name: "Summarize Data",
		Execute: func(ctx context.Context, input map[string]interface{}, w *workflow.Workflow) (map[string]interface{}, error) {
			log.Println("Summarizing data...")

			// 获取分析结果
			analysis, ok := input["analysis"].(string)
			if !ok {
				return nil, fmt.Errorf("invalid analysis format")
			}

			// 使用总结Agent创建摘要
			resp, err := summaryAgent.Generate([]agent.Message{
				{
					Role:    "user",
					Content: fmt.Sprintf("Create a concise summary of this analysis: %s", analysis),
				},
			}, &agent.GenerateOptions{})

			if err != nil {
				return nil, fmt.Errorf("summarization failed: %w", err)
			}

			// 返回最终结果
			return map[string]interface{}{
				"data":     input["data"],
				"analysis": analysis,
				"summary":  resp.Text,
			}, nil
		},
		Next: []string{"format_output"},
	}

	// 4. 输出格式化步骤
	formatStep := &workflow.Step{
		ID:   "format_output",
		Name: "Format Output",
		Execute: func(ctx context.Context, input map[string]interface{}, w *workflow.Workflow) (map[string]interface{}, error) {
			log.Println("Formatting output...")

			// 获取所需数据
			data := input["data"].(string)
			analysis := input["analysis"].(string)
			summary := input["summary"].(string)

			// 格式化最终输出
			output := fmt.Sprintf(`
===== WORKFLOW RESULTS =====

Original Data:
%s

Analysis:
%s

Summary:
%s

===========================
`, data, analysis, summary)

			return map[string]interface{}{
				"result": output,
			}, nil
		},
	}

	// 创建工作流
	wf, err := workflow.NewWorkflow(&workflow.WorkflowOptions{
		Name:        "Data Analysis Workflow",
		Description: "A workflow that collects, analyzes, and summarizes data",
		Steps:       []*workflow.Step{collectStep, analyzeStep, summarizeStep, formatStep},
		StartStep:   "collect_data",
	})
	if err != nil {
		log.Fatalf("Failed to create workflow: %v", err)
	}

	// 创建工作流管理器
	manager := workflow.NewWorkflowManager(system)

	// 注册工作流
	_, err = manager.RegisterWorkflow(wf)
	if err != nil {
		log.Fatalf("Failed to register workflow: %v", err)
	}

	// 运行工作流
	log.Println("Running workflow...")

	// 添加超时控制
	runCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 使用goroutine和通道处理工作流结果
	resultChan := make(chan map[string]interface{})
	errChan := make(chan error)

	go func() {
		result, err := manager.RunWorkflow(wf.ID, nil)
		if err != nil {
			errChan <- err
			return
		}
		resultChan <- result
	}()

	// 等待结果或超时
	select {
	case result := <-resultChan:
		// 打印结果
		fmt.Println(result["result"])
		log.Println("Workflow completed successfully!")
	case err := <-errChan:
		log.Fatalf("Workflow execution failed: %v", err)
	case <-runCtx.Done():
		log.Fatalf("Workflow execution timed out after 30 seconds")
	}
}
