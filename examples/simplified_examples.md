# 简化版示例代码

为了解决项目中的API兼容性问题，我们创建了两个简化版的示例代码，展示了如何正确使用当前实现的API。

## 简化版工作流示例

将以下代码保存为 `examples/workflow_demo/simple_workflow.go`：

```go
// 简化版工作流示例，兼容当前API实现
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/louloulin/gostra/pkg/workflow"
)

func main() {
	// 创建工作流步骤
	steps := []*workflow.Step{
		{
			ID:   "start",
			Name: "Starting Step",
			Description: "The first step in the workflow",
			Execute: func(ctx context.Context, input map[string]interface{}, w *workflow.Workflow) (map[string]interface{}, error) {
				fmt.Println("Executing starting step...")
				return map[string]interface{}{
					"step1_result": "Initial data processed",
				}, nil
			},
			Next: []string{"process"},
		},
		{
			ID:   "process",
			Name: "Processing Step",
			Description: "Process the data",
			Execute: func(ctx context.Context, input map[string]interface{}, w *workflow.Workflow) (map[string]interface{}, error) {
				fmt.Println("Processing data...")
				
				// 获取上一步结果
				stepResult, ok := input["step1_result"].(string)
				if !ok {
					return nil, fmt.Errorf("invalid input from previous step")
				}
				
				return map[string]interface{}{
					"step2_result": fmt.Sprintf("Processed: %s", stepResult),
				}, nil
			},
			Next: []string{"finish"},
		},
		{
			ID:   "finish",
			Name: "Final Step",
			Description: "Finalize the workflow",
			Execute: func(ctx context.Context, input map[string]interface{}, w *workflow.Workflow) (map[string]interface{}, error) {
				fmt.Println("Finalizing workflow...")
				
				// 获取上一步结果
				stepResult, ok := input["step2_result"].(string)
				if !ok {
					return nil, fmt.Errorf("invalid input from previous step")
				}
				
				return map[string]interface{}{
					"final_result": fmt.Sprintf("Final result: %s", stepResult),
				}, nil
			},
		},
	}

	// 创建工作流选项
	workflowOptions := &workflow.WorkflowOptions{
		Name:        "Simple Sequential Workflow",
		Description: "A basic workflow with sequential steps",
		Steps:       steps,
		StartStep:   "start",
		Metadata: map[string]interface{}{
			"version": "1.0.0",
			"type":    "example",
		},
	}

	// 创建工作流
	myWorkflow, err := workflow.NewWorkflow(workflowOptions)
	if err != nil {
		log.Fatalf("Failed to create workflow: %v", err)
	}

	fmt.Println("Workflow created successfully. Starting execution...")

	// 设置开始时间以测量性能
	startTime := time.Now()

	// 运行工作流
	result, err := myWorkflow.Run(context.Background(), nil)
	if err != nil {
		log.Fatalf("Error running workflow: %v", err)
	}

	// 计算执行时间
	executionTime := time.Since(startTime)

	// 打印结果
	fmt.Printf("\nWorkflow completed in %v\n", executionTime)
	fmt.Printf("Final result: %s\n", result["final_result"])

	fmt.Println("\nAll workflow results:")
	for k, v := range result {
		fmt.Printf("- %s: %v\n", k, v)
	}
}
```

## 简化版代理示例

将以下代码保存为 `examples/agent_demo/simple_agent.go`：

```go
// 简化版代理示例，使用兼容当前实现的API
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/louloulin/gostra/pkg/agent"
	"github.com/louloulin/gostra/pkg/memory"
	"github.com/louloulin/gostra/pkg/models"
)

// 简单的内存提供者
type SimpleMemoryProvider struct{}

func (s *SimpleMemoryProvider) SaveMessages(ctx context.Context, threadID string, messages []agent.Message) error {
	// 简化实现，仅用于示例
	fmt.Printf("Saving %d messages to thread %s\n", len(messages), threadID)
	return nil
}

func (s *SimpleMemoryProvider) GetMessages(ctx context.Context, threadID string) ([]agent.Message, error) {
	// 简化实现，仅用于示例
	return []agent.Message{}, nil
}

// 简单的模型提供者
type SimpleModelProvider struct{}

func (s *SimpleModelProvider) Generate(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (*models.Response, error) {
	// 简化实现，仅用于示例
	return &models.Response{
		Text: "这是一个模拟的AI响应",
	}, nil
}

func main() {
	fmt.Println("创建简化版代理示例...")

	// 创建一个简单的内存提供者
	memoryProvider := &SimpleMemoryProvider{}

	// 创建一个简单的模型提供者
	modelProvider := &SimpleModelProvider{}

	// 创建代理选项
	agentOptions := &agent.Options{
		ID:             "agent-123",
		Name:           "Simple Agent",
		SystemPrompt:   "你是一个简单的AI代理。",
		ModelProvider:  modelProvider,
		MemoryProvider: memoryProvider,
	}

	// 创建代理
	myAgent, err := agent.NewAgent(agentOptions)
	if err != nil {
		log.Fatalf("创建代理失败: %v", err)
	}

	fmt.Printf("代理创建成功: %s (ID: %s)\n", myAgent.Name, myAgent.ID)

	// 创建一个简单的运行选项
	runOptions := &agent.RunOptions{
		ThreadID:    "thread-123",
		UserMessage: "你好，请告诉我今天的日期",
	}

	// 运行代理
	fmt.Println("\n开始运行代理...")
	response, err := myAgent.Run(context.Background(), runOptions)
	if err != nil {
		log.Fatalf("运行代理失败: %v", err)
	}

	// 输出结果
	fmt.Println("\n代理响应:")
	fmt.Println(response)

	fmt.Println("\n代理示例完成!")
}
```

## 如何使用这些示例

1. 创建相应的目录：
   ```bash
   mkdir -p examples/workflow_demo
   mkdir -p examples/agent_demo
   ```

2. 将示例代码复制到相应的文件中。

3. 运行示例：
   ```bash
   go run examples/workflow_demo/simple_workflow.go
   go run examples/agent_demo/simple_agent.go
   ```

## API兼容性问题解决方案

我们已经完成了以下修复：

1. **包结构修复**：
   - 将混合包文件分离到不同目录
   - 创建了缺失的pkg/schema包

2. **导入路径修复**：
   - 修复了错误的导入路径前缀
   - 添加了模块替换配置

3. **API使用修复**：
   - 提供了简化示例，展示正确的API用法
   - 避免使用可能不存在的函数或方法

若要继续改进项目，建议：

1. 使用 `go build ./...` 命令检查编译错误
2. 对照每个包的实际实现检查API使用
3. 考虑创建更多简化示例，专注于单一功能
4. 更新文档，明确每个API的用法和期望行为 