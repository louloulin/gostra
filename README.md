# Gostra - 基于Actor模型的Go语言AI Agent框架

<p align="center">
  <strong>高性能、可扩展的AI Agent框架 · 基于Go和Actor模型构建</strong>
</p>

## 简介

Gostra是一个基于[protoactor-go](https://github.com/asynkron/protoactor-go)构建的Go语言AI Agent框架，旨在提供高性能、可扩展的AI应用开发环境。Gostra参考了[Mastra](https://mastra.ai)的设计思想，将Actor模型的并发优势与AI Agent的功能需求相结合，特别适用于需要处理大量并发请求和复杂工作流的场景。

## 核心特性

- **基于Actor模型**：利用Actor模型实现高并发、高可靠性的系统架构
- **AI Agent框架**：提供完整的Agent定义、工具调用、内存管理和工作流功能
- **高性能设计**：充分利用Go语言和Actor模型的性能优势
- **可扩展架构**：支持横向扩展和分布式部署
- **完整工具链**：包含内存、向量存储、评估和部署等完整功能模块

## 项目状态

⚠️ **注意**：Gostra目前处于早期开发阶段，API可能会有重大变更。

## 快速开始

### 安装

```bash
go get github.com/yourusername/gostra
```

### 基本示例

```go
package main

import (
    "context"
    "fmt"
    
    "github.com/yourusername/gostra"
    "github.com/yourusername/gostra/agent"
    "github.com/yourusername/gostra/models/openai"
)

func main() {
    // 初始化Gostra系统
    g := gostra.New()
    
    // 创建一个Agent
    myAgent := agent.New(&agent.Config{
        Name: "My Agent",
        Instructions: "You are a helpful assistant.",
        Model: openai.New("gpt-4"),
    })
    
    // 注册Agent
    g.RegisterAgent("myAgent", myAgent)
    
    // 启动系统
    if err := g.Start(context.Background()); err != nil {
        panic(err)
    }
    
    // 生成文本
    resp, err := myAgent.Generate([]agent.Message{
        {Role: "user", Content: "Hello, how can you assist me today?"},
    }, nil)
    
    if err != nil {
        panic(err)
    }
    
    fmt.Println("Agent:", resp.Text)
    
    // 关闭系统
    g.Stop()
}
```

## 主要组件

Gostra由以下核心组件构成：

1. **Agent**：AI Agent的基本单元，处理指令和生成响应
2. **Tool**：Agent可以调用的工具函数
3. **Memory**：管理Agent的上下文和历史记录
4. **Workflow**：定义和执行复杂的多步骤流程
5. **Actor System**：基于protoactor-go的消息传递系统
6. **Server**：提供HTTP API接口

## 与Mastra的区别

Gostra与Mastra共享相似的概念和设计理念，但有以下主要区别：

1. **语言实现**：Gostra使用Go语言，而Mastra使用TypeScript
2. **并发模型**：Gostra基于Actor模型，提供更高的并发性能
3. **部署模式**：Gostra支持单二进制部署，简化运维复杂度
4. **性能优化**：针对高吞吐量场景进行了特别优化

## 贡献指南

我们欢迎各种形式的贡献，包括但不限于：

- 提交问题和功能请求
- 提交Pull Request修复问题或添加功能
- 完善文档和示例
- 分享使用经验和最佳实践

请参阅[贡献指南](CONTRIBUTING.md)了解更多信息。

## 许可证

Gostra采用[Apache 2.0许可证](LICENSE)开源。

## 致谢

- [protoactor-go](https://github.com/asynkron/protoactor-go)：提供Actor模型实现
- [Mastra](https://mastra.ai)：提供设计灵感和参考
- 所有贡献者和使用者 