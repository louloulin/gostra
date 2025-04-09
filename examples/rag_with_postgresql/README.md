# PostgreSQL RAG 实现示例

这个示例展示了如何使用Gostra框架实现一个完整的RAG（检索增强生成）系统，该系统使用PostgreSQL向量存储来存储和检索文档嵌入。

## 功能特点

- 文档处理和分块
- 使用OpenAI嵌入API生成文本嵌入
- 将嵌入存储在PostgreSQL中（使用pgvector扩展）
- 使用Actor架构进行查询和响应
- 高级元数据过滤功能：
  - 支持精确匹配、比较运算符、正则表达式
  - 支持嵌套元数据路径
  - 支持数组内容过滤

## 前置要求

1. 安装PostgreSQL（版本14+）并启用pgvector扩展
2. 设置必要的环境变量
3. 安装Gostra依赖项

## 设置PostgreSQL与pgvector

1. 安装PostgreSQL（如果尚未安装）
   ```
   # Ubuntu/Debian
   sudo apt install postgresql postgresql-contrib
   
   # macOS (使用Homebrew)
   brew install postgresql
   ```

2. 安装pgvector扩展（确保您有编译器和PostgreSQL开发包）
   ```
   # 克隆并编译pgvector
   git clone --branch v0.5.1 https://github.com/pgvector/pgvector.git
   cd pgvector
   make
   make install
   ```

3. 创建数据库并启用扩展
   ```
   createdb ragdb
   psql -d ragdb -c "CREATE EXTENSION IF NOT EXISTS vector;"
   ```

## 运行示例

1. 设置必要的环境变量
   ```
   export OPENAI_API_KEY="your-openai-api-key"
   export POSTGRES_CONNECTION_STRING="postgresql://username:password@localhost:5432/ragdb"
   ```

2. 从Gostra根目录运行示例
   ```
   cd gostra
   go run examples/rag_with_postgresql/main.go
   ```

## 示例输出

示例将执行两类查询:

1. 基本查询：直接从文档内容中检索信息
2. 元数据过滤查询：使用高级过滤器定位特定信息

### 基本查询示例

```
问题: 什么是监督学习？
回答: [LLM生成的回答基于文档内容]

问题: 机器学习中有哪些主要的类型？
回答: [LLM生成的回答基于文档内容]
```

### 元数据过滤查询示例

```
问题: 使用正则表达式搜索标题中包含"学习"的所有内容
回答: [使用正则表达式过滤后生成的回答]

问题: 查找难度级别为"高级"的所有学习方法
回答: [使用嵌套元数据过滤后生成的回答]
```

## 代码说明

该示例演示了:

1. **Actor系统初始化**：创建并配置Gostra Actor系统
2. **工具链集成**：创建文档处理、向量搜索和文档搜索工具
3. **Agent配置**：设置RAG代理，提供清晰的指令和可用工具
4. **PostgreSQL向量存储**：配置和使用postgres_vector实现
5. **高级元数据过滤**：演示对复杂元数据结构的查询能力

## 扩展可能性

可以通过以下方式扩展此示例:

1. 添加更多文档类型和数据源
2. 实现增量更新和文档版本控制
3. 添加文档聚类和主题建模
4. 实现多Agent协作的复杂RAG系统
5. 添加Web API接口供外部应用程序使用 