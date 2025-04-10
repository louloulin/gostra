# Gostra 项目完整修复指南

经过检查和修复，我们解决了项目中多个结构性问题，并提供了简化示例。以下是修复过程的总结和未解决问题的解决方案。

## 已修复的问题

### 1. 项目结构问题

- **混合包声明**：将不同包的文件移动到了单独的目录中
  ```
  examples/workflows/          -> examples/workflows/main_examples/
                                 examples/workflows/workflow_examples/
                                 examples/workflows/parallel_workflow_examples/
  pkg/tools/search/            -> 测试文件移至 pkg/tools/search/test/
  ```

- **主函数重复声明**：将包含多个main函数的文件分离到不同目录，避免冲突
  ```
  examples/agent_network/      -> 将文件移动到子目录
  ```

### 2. 导入路径问题

- **错误的导入前缀**：统一修正了不正确的导入路径前缀
  ```
  github.com/louloulin/agent/gastra/gostra/pkg -> github.com/louloulin/gostra/pkg
  github.com/louloulin/agent/gastra/pkg -> github.com/louloulin/gostra/pkg
  ```

- **相对路径导入**：将不支持的相对路径导入（`"./xxx"`）改为绝对路径
  ```
  "./context_sharing" -> "github.com/louloulin/gostra/examples/agent_network/context_sharing"
  ```

- **缺失包**：创建了缺失的pkg/schema包以满足导入需求

### 3. API兼容性问题

- **Actor API**：修正了ActorSystem创建和使用
  ```go
  // 旧代码
  actorSystem := actor.NewActorSystem(actor.ActorSystemOptions{})
  // 修正后
  actorSystem := actor.NewActorSystem(&actor.Configuration{})
  ```

- **Workflow API**：简化了workflow创建示例，使用符合当前API的接口

## 仍需修复的问题

### 1. API 包中的重复声明

在pkg/api目录中发现多个重复声明：
```
ServerOptions redeclared
DefaultServerOptions redeclared
Server redeclared
NewServer redeclared
Server.AddMiddleware already declared
Server.RegisterPlugin already declared
Server.Start already declared
Server.Stop already declared
```

**解决方案**：
- 检查 `pkg/api/api.go` 和 `pkg/api/server.go` 文件
- 合并这两个文件，或者将它们分离为不同的包
- 可能的方法：将 `api.go` 重命名为 `client.go`，专注于API客户端功能

### 2. API 类型不匹配

发现API调用中的类型不匹配问题：
```
cannot use req.Messages (variable of type []agent.Message) as string value
cannot use req.Options (variable of type *agent.StreamOptions) as pkg.StreamOptions
```

**解决方案**：
- 检查 `pkg/api/api.go` 中的 Stream 方法调用
- 更新方法签名以匹配预期类型，或创建适配器转换类型

### 3. Schema包未被正确识别

尽管创建了schema包，但模块系统仍未正确识别：
```
github.com/louloulin/gostra/pkg/schema: no matching versions for query "latest"
```

**解决方案**：
- 在go.mod中明确添加replace指令：
  ```
  replace github.com/louloulin/gostra/pkg/schema => ./pkg/schema
  ```
- 或考虑将schema包内容合并到主模块中

## 推荐的修复步骤

1. **解决API包中的重复声明**：
   ```bash
   # 创建备份
   cp pkg/api/api.go pkg/api/api.go.bak
   cp pkg/api/server.go pkg/api/server.go.bak
   
   # 将server.go中的内容移至单独的包
   mkdir -p pkg/server
   cp pkg/api/server.go pkg/server/server.go
   
   # 修改server.go中的包声明为server
   sed -i '' 's/package api/package server/' pkg/server/server.go
   
   # 或者合并这两个文件
   # 这需要仔细手动编辑，确保没有重复声明
   ```

2. **修复API类型不匹配**：
   ```go
   // 在pkg/api/api.go中找到Stream方法调用
   // 修改为：
   response, err := agentInstance.Stream(ctx, req.UserMessage, req.Options)
   // 或创建适配器转换类型
   ```

3. **修复Schema包导入**：
   ```bash
   # 在go.mod中添加或更新replace指令
   go mod edit -replace github.com/louloulin/gostra/pkg/schema=./pkg/schema
   
   # 更新go.mod
   go mod tidy
   ```

4. **运行构建测试**：
   ```bash
   # 只构建核心包
   go build ./pkg/...
   
   # 构建简化示例
   go build ./examples/workflow_demo/simple_workflow.go
   go build ./examples/agent_demo/simple_agent.go
   
   # 尝试构建整个项目
   go build ./...
   ```

## 简化示例

我们创建了两个简化示例，展示如何正确使用当前实现的API：

1. **工作流示例**：`examples/workflow_demo/simple_workflow.go`
2. **代理示例**：`examples/agent_demo/simple_agent.go`

这些示例避免了使用不确定或未实现的API，可以作为项目未来开发的参考。

## 总结

Gostra项目中的大多数结构性问题已经得到修复，但仍有一些API兼容性问题需要解决。主要的剩余问题集中在API包的重复声明和类型不匹配上。通过遵循本指南中的步骤，应该能够解决这些问题并使项目能够正常构建和运行。 