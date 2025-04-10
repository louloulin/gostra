# Go Vet 问题修复计划

经过运行 `go vet ./...` 分析，我们发现了以下问题，需要进行修复：

## 1. 空文件问题

以下文件存在但内容为空，需要添加适当的代码或删除：

- `pkg/errors/supervisor_test.go` - 期望包声明，但文件为空
- `pkg/tools/search/graph_rag.go` - 期望包声明，但文件为空
- `pkg/memory/postgres_vector_integration_test.go` - 期望包声明，但文件为空

## 2. 导入循环问题

在以下包之间存在导入循环：
```
github.com/louloulin/gostra/pkg/agent
github.com/louloulin/gostra/pkg/memory
github.com/louloulin/gostra/pkg/tools/document
github.com/louloulin/gostra/pkg/tools/search
```

具体循环路径：
- `pkg/tools/search/graph_types.go` 导入了 `pkg/tools/document`
- `pkg/tools/document/document_search_adapter.go` 导入了 `pkg/tools/search`

## 3. 导入路径错误

在 `examples/search/main.go` 中的导入路径不正确：
```go
import (
    "github.com/louloulin/agent/gastra/gostra/pkg/memory"
    "github.com/louloulin/agent/gastra/gostra/pkg/tools/document"
    "github.com/louloulin/agent/gastra/gostra/pkg/tools/search"
)
```

而正确的导入路径应该是：
```go
import (
    "github.com/louloulin/gostra/pkg/memory"
    "github.com/louloulin/gostra/pkg/tools/document"
    "github.com/louloulin/gostra/pkg/tools/search"
)
```

## 4. 未使用的变量

在 `pkg/tools/search/graph_rag_tool.go` 中有未使用的变量：
```go
// Line 260
for id, score := range newScores {
    scoreSum += score
}
```
变量 `id` 被声明但未使用。

## 修复计划

### 1. 空文件处理

- [ ] **删除空文件**
  ```bash
  rm pkg/errors/supervisor_test.go
  rm pkg/tools/search/graph_rag.go
  rm pkg/memory/postgres_vector_integration_test.go
  ```
  如果这些文件是必需的，则添加适当的内容：
  ```go
  // pkg/errors/supervisor_test.go
  package errors

  import (
      "testing"
  )

  func TestSupervisor(t *testing.T) {
      // TODO: 实现测试
  }
  ```

### 2. 解决导入循环

- [ ] **创建共享类型包**
  1. 创建一个新的包 `pkg/tools/common` 来存放共享的数据类型
  2. 将 `DocumentChunk` 移到 `pkg/tools/common/types.go` 文件中
  3. 修改 `pkg/tools/search/graph_types.go`，导入共享包：
  ```go
  package search

  import (
      "github.com/louloulin/gostra/pkg/tools/common"
  )

  // GraphNode 图节点
  type GraphNode struct {
      ID       string                 `json:"id"`
      Content  string                 `json:"content"`
      Score    float32                `json:"score"`
      Chunk    *common.DocumentChunk  `json:"chunk,omitempty"`
      Metadata map[string]interface{} `json:"metadata,omitempty"`
  }
  ```
  
  4. 修改 `pkg/tools/document/document_search_adapter.go`，使用接口代替具体实现：
  ```go
  package document

  import (
      "context"
      "errors"
      "fmt"

      "github.com/louloulin/gostra/pkg/tools"
      "github.com/louloulin/gostra/pkg/tools/common"
  )

  // VectorSearchInterface 定义向量搜索工具的接口
  type VectorSearchInterface interface {
      Execute(params map[string]interface{}, options *tools.ExecuteOptions) (interface{}, error)
      GetID() string
      GetDescription() string
      GetInputSchema() tools.Schema
  }

  // DocumentSearchAdapterImpl 文档搜索适配器
  type DocumentSearchAdapterImpl struct {
      // 向量搜索工具
      VectorTool VectorSearchInterface
      // 其他字段...
  }
  ```

### 3. 修复导入路径

- [ ] **更新示例代码中的导入路径**
  编辑 `examples/search/main.go` 文件，修改导入路径：
  ```go
  import (
      "github.com/louloulin/gostra/pkg/memory"
      "github.com/louloulin/gostra/pkg/tools/document"
      "github.com/louloulin/gostra/pkg/tools/search"
  )
  ```

### 4. 修复未使用的变量

- [ ] **修复 `pkg/tools/search/graph_rag_tool.go` 中的未使用变量**
  
  方法1: 使用匿名变量
  ```go
  for _, score := range newScores {
      scoreSum += score
  }
  ```
  
  方法2: 在后续代码中使用该变量
  ```go
  for id, score := range newScores {
      scoreSum += score
      fmt.Printf("Node %s: score %f\n", id, score) // 使用id变量
  }
  ```

## 执行计划

1. 首先解决导入循环问题，因为这是最复杂的部分
2. 接着修复小问题（未使用的变量）
3. 更新示例代码中的导入路径
4. 最后删除或填充空文件

## 测试验证

每完成一步修改后，运行以下命令验证：
```bash
go vet ./...
```

完成所有修改后，运行完整测试确保代码功能正常：
```bash
go test ./...
```

## 长期改进建议

1. 添加 Go linter 到开发流程中，例如 golangci-lint
2. 实施自动化代码检查和测试
3. 审查包结构设计，避免循环依赖
4. 使用接口隔离层来减少包之间的直接依赖 