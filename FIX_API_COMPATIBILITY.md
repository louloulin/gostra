# API兼容性问题修复指南

在修复包结构和导入路径后，我们发现代码中仍存在API兼容性问题。下面是详细的兼容性问题清单和建议的修复方法。

## 1. 主要API不匹配问题

### `pkg/actor` 包

**问题**:
- 示例中使用了 `actor.ActorSystemOptions{}` 但实际API使用的是 `actor.Configuration{}`
- 示例中假设ActorSystem可以直接传递给AgentNetwork创建函数

**修复建议**:
```go
// 旧代码
actorSystem := actor.NewActorSystem(actor.ActorSystemOptions{})

// 修复后
actorSystem := actor.NewActorSystem(&actor.Configuration{})

// 然后在AgentNetworkOptions中使用
networkOptions := &agent.AgentNetworkOptions{
    ActorSystem: actorSystem,
    // 其他选项...
}
```

### `pkg/models` 包

**问题**:
- 示例中使用了 `models.NewOpenAIModelProvider` 和 `models.OpenAIModelProviderOptions` 
- 实际API可能使用不同的函数名和结构体

**修复建议**:
```go
// 旧代码
modelProvider := models.NewOpenAIModelProvider(&models.OpenAIModelProviderOptions{
    ApiKey:   "YOUR_API_KEY",
    Model:    "gpt-4",
    MaxRetry: 3,
    Timeout:  time.Second * 60,
})

// 修复后 (查看pkg/models实际定义)
modelProvider, err := models.NewOpenAIProvider(&models.OpenAIOptions{
    APIKey: "YOUR_API_KEY",
    Model:  "gpt-4",
})
if err != nil {
    log.Fatalf("Failed to create model provider: %v", err)
}
```

### `pkg/workflow` 包

**问题**:
- `WorkflowOptions` 结构体缺少示例中使用的 `InputSchema` 和 `OutputSchema` 字段
- 示例使用了 `BuildParallelCOTWorkflow` 函数，但实际可能不存在

**修复建议**:
```go
// 添加必要的步骤定义
steps := []*workflow.Step{
    {
        ID:          "context_analysis",
        Name:        "Context Analysis",
        Description: "Analyze the context of the query",
        Execute: func(ctx context.Context, input map[string]interface{}, w *workflow.Workflow) (map[string]interface{}, error) {
            // 实现步骤逻辑
            return map[string]interface{}{
                "context_analysis": "Context analysis result",
            }, nil
        },
    },
    // 添加其他步骤...
}

// 正确创建WorkflowOptions
workflowOptions := &workflow.WorkflowOptions{
    Name:        "Parallel Critical Analysis Workflow",
    Description: "A workflow that performs critical analysis with parallel steps",
    Steps:       steps,
    StartStep:   "context_analysis", // 指定起始步骤
    Metadata: map[string]interface{}{
        "version": "1.0.0",
        "type":    "analysis",
    },
}

// 使用正确的函数创建Workflow
cotWorkflow, err := workflow.NewWorkflow(workflowOptions)
if err != nil {
    log.Fatalf("Failed to create workflow: %v", err)
}

// 使用正确的方法运行Workflow
result, err := cotWorkflow.Run(context.Background(), inputData)
```

### `pkg/agent` 包

**问题**:
- 示例假设 `agent.NewAgentNetwork` 只接受两个参数，但实际函数签名不同
- 示例中使用了 `network.Start` 和 `network.Shutdown` 方法，但实际API可能不支持

**修复建议**:
```go
// 创建正确的网络选项
networkOptions := &agent.AgentNetworkOptions{
    ID:          "network-id",
    Name:        "Network Name",
    ActorSystem: actorSystem,
    // 其他必要选项...
}

// 创建网络
network, err := agent.NewAgentNetwork(networkOptions, modelProvider)
if err != nil {
    log.Fatalf("Failed to create network: %v", err)
}

// 注意：检查network是否有Start和Shutdown方法，如果没有则删除相关代码
```

## 2. 其他API差异

根据APIs的实际定义，还需要解决以下问题：

1. 内存提供者初始化
```go
memoryProvider, err := memory.NewInMemoryProvider()
if err != nil {
    log.Fatalf("Failed to create memory provider: %v", err)
}
```

2. Schema操作
```go
// 确保schema包在项目中正确配置并能被导入
```

## 如何进行修复

1. **对照实际代码检查API**：
   - 查看每个包中的实际实现
   - 按照实际接口修改示例代码

2. **渐进式修复**：
   - 先修复actor系统和基本组件
   - 然后处理Agent和Workflow的创建
   - 最后修复具体的执行代码

3. **可选：简化示例**：
   - 如果完整的示例难以修复，考虑创建一个简化的示例
   - 专注于展示核心功能而不是完整的工作流程

## 修复后检查

在完成修复后，使用以下命令验证：

```bash
# 进入项目目录
cd gostra

# 运行go mod tidy更新依赖
go mod tidy

# 尝试构建项目
go build ./...

# 如果特定示例修复后，可以尝试运行它
# go run examples/workflows/main_examples/parallel_cot_example.go
``` 