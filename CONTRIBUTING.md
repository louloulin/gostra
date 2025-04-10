# Gostra 贡献指南

感谢您对 Gostra 项目的关注！我们欢迎各种形式的贡献，包括代码提交、文档改进、错误报告和功能建议。本指南将帮助您了解如何参与贡献。

## 行为准则

参与 Gostra 项目的所有贡献者都应遵循我们的行为准则。我们希望所有贡献者创造一个积极、包容和尊重的环境。

## 如何贡献

### 报告问题

如果您发现了问题或有功能请求，请先检查[现有 issues](https://github.com/louloulin/gostra/issues)，看是否已经有人报告过相同的问题。如果没有，请创建一个新的 issue，并提供以下信息：

1. 清晰描述问题或功能请求
2. 如何重现问题（对于错误报告）
3. 预期行为和实际行为
4. 系统环境（Go 版本、操作系统等）
5. 相关日志或截图

### 提交代码

1. Fork 本仓库
2. 创建功能分支 (`git checkout -b feature/your-feature-name`)
3. 提交更改 (`git commit -am 'Add some feature'`)
4. 推送到分支 (`git push origin feature/your-feature-name`)
5. 创建新的 Pull Request

### Pull Request 准则

- 每个 PR 应该专注于一个特定的功能或修复
- 代码应遵循项目的代码风格（使用 `go fmt` 和 `go vet`）
- 所有测试必须通过
- 更新相关文档
- PR 描述应清晰地说明更改内容和原因

### 开发流程

1. **设置开发环境**

```bash
git clone https://github.com/louloulin/gostra.git
cd gostra
go mod tidy
```

2. **运行测试**

```bash
go test ./...
```

3. **代码风格检查**

```bash
go fmt ./...
go vet ./...
```

## 项目结构

了解项目结构有助于您进行贡献：

```
gostra/
├── agent/          # Agent 相关代码
├── memory/         # 内存和向量存储
├── tools/          # 工具定义和管理
├── workflow/       # 工作流定义和执行
├── actor/          # Actor 系统实现
├── server/         # HTTP 服务器实现
├── examples/       # 示例代码
└── docs/           # 文档
```

## 开发指南

### Actor 模型基础

如果您不熟悉 Actor 模型，我们建议先阅读以下资源：

- [The Actor Model](https://en.wikipedia.org/wiki/Actor_model)
- [ProtoActor Go 文档](https://proto.actor/docs/golang/)

### 编码规范

- 遵循 [Effective Go](https://golang.org/doc/effective_go.html) 中的建议
- 使用 [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments) 中的最佳实践
- 为所有导出函数和类型编写文档注释
- 编写单元测试，目标是达到至少 80% 的代码覆盖率

### 文档贡献

除了代码贡献外，我们也非常欢迎文档改进：

- 修正文档中的错误或不清晰之处
- 添加示例或教程
- 改进 API 文档

## 版本控制

我们使用[语义化版本控制](https://semver.org/)。版本号格式为 X.Y.Z：

- X = 主版本号：不兼容的 API 变更
- Y = 次版本号：向后兼容的功能新增
- Z = 修订号：向后兼容的问题修复

## 许可证

通过贡献您的代码，您同意将其授权给项目，并根据项目的许可证（Apache 2.0）发布。

## 联系我们

如果您有任何问题或需要帮助，可以通过以下方式联系我们：

- GitHub Issues
- 项目讨论区
- 电子邮件：your.email@example.com

感谢您对 Gostra 的贡献！ 