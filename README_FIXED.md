# 修复包和导入问题

我们已经修复了项目中的大部分包结构和导入路径问题。以下是主要修复内容：

## 1. 包结构问题

- 修复了混合包声明问题：将不同包的文件移动到单独的子目录中
  - `examples/workflows` -> 分离为 `main_examples`、`workflow_examples` 和 `parallel_workflow_examples`
  - `pkg/tools/search` -> 分离测试包到 `test` 子目录

- 修复了main函数重复声明问题：将多个包含main函数的文件分离到不同目录
  - `examples/agent_network` -> 将 `configurable_timeout.go` 移动到 `examples` 子目录

## 2. 导入路径问题

- 修复了不正确的导入路径前缀：
  - `github.com/louloulin/agent/gastra/gostra/pkg` -> `github.com/louloulin/gostra/pkg`
  - `github.com/louloulin/agent/gastra/pkg` -> `github.com/louloulin/gostra/pkg`
  - `./context_sharing` -> `github.com/louloulin/gostra/examples/agent_network/context_sharing`

- 创建了缺失的包：
  - 添加了 `pkg/schema` 包，实现基本功能

## 剩余需要解决的问题

尽管修复了主要结构问题，但仍有一些API不匹配问题需要解决：

1. `parallel_cot_example.go` 和其他示例文件中的API调用与实际实现不匹配：
   - 函数参数不匹配
   - 结构体字段名不匹配
   - 请确保API调用与最新版本的pkg目录中的实现匹配

2. 运行测试例子前，需要确保：
   - 所有API调用正确
   - 安装了所有必要的依赖
   - 配置了正确的API密钥

## 如何运行修复后的代码

```bash
# 进入gostra目录
cd gostra

# 更新依赖
go mod tidy

# 构建项目
go build ./...

# 运行特定示例
go run examples/agent_network/context_sharing_example.go
``` 