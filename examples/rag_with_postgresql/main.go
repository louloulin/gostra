package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/louloulin/gostra/pkg/actor"
	"github.com/louloulin/gostra/pkg/agent"
	"github.com/louloulin/gostra/pkg/memory"
	"github.com/louloulin/gostra/pkg/models/openai"
	"github.com/louloulin/gostra/pkg/tools"
	"github.com/louloulin/gostra/pkg/tools/document"
	"github.com/louloulin/gostra/pkg/tools/search"
)

func main() {
	// 获取环境变量
	openaiKey := os.Getenv("OPENAI_API_KEY")
	if openaiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	pgConnStr := os.Getenv("POSTGRES_CONNECTION_STRING")
	if pgConnStr == "" {
		log.Fatal("POSTGRES_CONNECTION_STRING environment variable is required")
	}

	// 创建上下文
	ctx := context.Background()

	// 初始化Actor系统
	system := actor.NewActorSystem(&actor.Configuration{})

	// 创建OpenAI提供者
	openaiProvider, err := openai.NewOpenAIProvider(&openai.Options{
		APIKey: openaiKey,
		Model:  "gpt-4o",
	})
	if err != nil {
		log.Fatalf("Failed to create OpenAI provider: %v", err)
	}

	// 创建嵌入向量提供者
	embeddingProvider, err := memory.NewOpenAIEmbeddingProvider(&memory.OpenAIEmbeddingOptions{
		APIKey: openaiKey,
		Model:  "text-embedding-3-small",
	})
	if err != nil {
		log.Fatalf("Failed to create embedding provider: %v", err)
	}

	// 创建PostgreSQL向量存储
	pgVectorStore, err := memory.NewPostgresVectorStorage(memory.PostgresVectorOptions{
		ConnectionString: pgConnStr,
		TableName:        "rag_example",
		VectorDimension:  1536, // OpenAI text-embedding-3-small 维度
		BatchSize:        100,
	})
	if err != nil {
		log.Fatalf("Failed to create PostgreSQL vector storage: %v", err)
	}
	defer pgVectorStore.Close()

	// 清除之前的数据
	if err := pgVectorStore.Clear(ctx); err != nil {
		log.Printf("Warning: Failed to clear vector storage: %v", err)
	}

	// 创建文档分块工具
	documentChunker := document.NewDocumentChunker(document.DocumentChunkerOptions{})

	// 创建向量搜索工具
	vectorSearchTool := search.NewVectorSearchTool(search.VectorSearchOptions{
		EmbeddingProvider: embeddingProvider,
		Dimension:         1536,
		DistanceMetric:    "cosine",
	})

	// 创建文档搜索工具
	documentSearchTool := search.NewDocumentSearchTool(search.DocumentSearchOptions{
		VectorSearchTool:  vectorSearchTool,
		DocumentChunker:   documentChunker,
		EmbeddingProvider: embeddingProvider,
		ChunkSize:         1000,
		ChunkOverlap:      200,
		ChunkStrategy:     document.StrategyRecursive,
	})

	// 设置工具函数，将文档存储到PostgreSQL
	storeDocumentFunction := func(chunks []*document.DocumentChunk) error {
		// 提取文本内容，用于生成嵌入向量
		texts := make([]string, len(chunks))
		for i, chunk := range chunks {
			texts[i] = chunk.Content
		}

		// 获取嵌入向量
		embeddings, err := embeddingProvider.GetEmbeddings(texts)
		if err != nil {
			return fmt.Errorf("获取嵌入向量失败: %w", err)
		}

		// 转换为向量存储格式
		vectors := make([]memory.Vector, len(chunks))
		for i, chunk := range chunks {
			id := fmt.Sprintf("%s-%d", chunk.DocumentID, i)
			vectors[i] = memory.Vector{
				ID:       id,
				Values:   embeddings[i],
				Metadata: chunk.Metadata,
			}
		}

		// 存储到PostgreSQL
		return pgVectorStore.Store(ctx, vectors)
	}

	// 创建RAG (Retrieval Augmented Generation) 的代理
	ragAgent, err := agent.NewAgent(&agent.Options{
		ID:            "rag-agent",
		Name:          "rag-agent",
		ModelProvider: openaiProvider,
		SystemPrompt: `你是一个擅长基于文档回答问题的助手。
你只使用提供的文档内容回答问题，不会编造信息。
如果问题无法从文档内容中回答，请明确告知用户。

你可以使用高级元数据过滤功能来查找特定信息：

1. 精确匹配: {"field": value}
2. 比较操作: {"field": {"$gt": value}} 支持 $eq, $ne, $gt, $gte, $lt, $lte
3. 正则表达式: {"field": {"$regex": pattern}}
4. 包含操作: {"field": {"$in": [value1, value2]}}
5. 嵌套字段: {"nested.field": value}

当你需要使用高级过滤时，请构建适当的filters参数以便更精确地检索信息。`,
		Tools: []tools.Tool{documentSearchTool},
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// 注册Agent
	agentID, err := system.RegisterAgent("rag-agent", ragAgent)
	if err != nil {
		log.Fatalf("Failed to register agent: %v", err)
	}
	log.Printf("Agent registered with ID: %s", agentID)

	// 启动Actor系统
	system.Start()
	defer system.Stop()

	// 样本文档内容
	sampleDocument := `# 机器学习基础概念

## 监督学习
监督学习是机器学习的一种方法，其中算法从标记数据中学习。在监督学习中，每个训练样本都包含输入特征和所需的输出标签。目标是学习一个可以预测新输入特征的输出标签的函数。

常见的监督学习算法包括：
- 线性回归（用于回归问题）
- 逻辑回归（用于分类问题）
- 支持向量机（SVM）
- 决策树和随机森林
- 神经网络

## 无监督学习
无监督学习是机器学习的另一种方法，其中算法从未标记数据中学习。目标是发现数据中的隐藏模式或内在结构。

常见的无监督学习算法包括：
- 聚类算法（如K-均值）
- 降维技术（如主成分分析）
- 关联规则学习

## 强化学习
强化学习涉及一个代理在环境中采取行动以最大化累积奖励。与监督学习不同，强化学习不需要标记的输入/输出对，而是专注于在动态环境中找到平衡探索和利用的策略。

强化学习的关键组成部分：
- 代理（Agent）：学习和制定决策的实体
- 环境（Environment）：代理与之交互的外部系统
- 行动（Actions）：代理可以执行的一组行动
- 奖励（Rewards）：评估代理行动的信号

## 深度学习
深度学习是机器学习的一个子领域，专注于使用神经网络的算法，这些神经网络具有多个层（因此得名"深度"）。

深度学习模型包括：
- 前馈神经网络
- 卷积神经网络（CNN），主要用于图像分析
- 循环神经网络（RNN），用于序列数据
- 转换器模型，用于自然语言处理任务

## 评估指标
不同类型的机器学习问题需要不同的评估指标：

### 分类指标
- 准确率：正确预测的样本比例
- 精确率：真阳性/(真阳性+假阳性)
- 召回率：真阳性/(真阳性+假阴性)
- F1分数：精确率和召回率的调和平均值

### 回归指标
- 均方误差（MSE）
- 平均绝对误差（MAE）
- R平方（确定系数）

## 过拟合与欠拟合
- 过拟合：模型过于复杂，在训练数据上表现好，但不能很好地泛化到新数据
- 欠拟合：模型过于简单，无法捕捉训练数据中的模式

## 正则化技术
正则化技术用于防止过拟合：
- L1正则化（Lasso）
- L2正则化（Ridge）
- Dropout（随机丢弃）
- 早停（Early Stopping）
- 数据增强（Data Augmentation）

## 超参数调优
超参数是在学习过程开始之前设置的参数，而不是通过训练学习的：
- 学习率
- 批量大小
- 隐藏层数量
- 隐藏单元数量
- 正则化强度

调优技术包括：
- 网格搜索
- 随机搜索
- 贝叶斯优化`

	// 添加示例文档
	err = documentSearchTool.AddDocument(sampleDocument, "ml-basics", map[string]interface{}{
		"title":      "机器学习基础概念",
		"category":   "机器学习",
		"author":     "AI助手",
		"year":       "2024",
		"tags":       []string{"机器学习", "AI", "数据科学"},
		"difficulty": "中级",
		"sections": map[string]interface{}{
			"监督学习": map[string]interface{}{
				"algorithms": []string{"线性回归", "逻辑回归", "SVM", "决策树", "随机森林", "神经网络"},
				"difficulty": "简单",
			},
			"无监督学习": map[string]interface{}{
				"algorithms": []string{"K-均值", "PCA", "关联规则学习"},
				"difficulty": "中级",
			},
			"强化学习": map[string]interface{}{
				"components": []string{"代理", "环境", "行动", "奖励"},
				"difficulty": "高级",
			},
			"深度学习": map[string]interface{}{
				"models":     []string{"前馈神经网络", "CNN", "RNN", "转换器"},
				"difficulty": "高级",
			},
		},
	})
	if err != nil {
		log.Fatalf("Failed to add document: %v", err)
	}

	// 演示基本查询
	demoQueries := []string{
		"什么是监督学习？",
		"机器学习中有哪些主要的类型？",
		"深度学习包含哪些模型类型？",
		"过拟合是什么，如何防止？",
	}

	// 演示元数据过滤查询
	metadataQueries := []string{
		`使用正则表达式搜索标题中包含"学习"的所有内容`,
		`查找难度级别为"高级"的所有学习方法`,
		`列出所有监督学习算法`,
	}

	fmt.Println("===== 基本查询示例 =====")
	for _, query := range demoQueries {
		fmt.Printf("\n问题: %s\n", query)
		result, err := ragAgent.GenerateResponse(ctx, query, nil)
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}
		fmt.Printf("回答: %s\n", result)
	}

	fmt.Println("\n\n===== 元数据过滤查询示例 =====")
	for _, query := range metadataQueries {
		fmt.Printf("\n问题: %s\n", query)

		var result string
		if query == metadataQueries[0] {
			// 示例1：使用正则表达式过滤
			result, err = executeFilteredQuery(ctx, ragAgent, "学习", map[string]interface{}{
				"title": map[string]interface{}{
					"$regex": "学习",
				},
			})
		} else if query == metadataQueries[1] {
			// 示例2：嵌套元数据过滤
			result, err = executeFilteredQuery(ctx, ragAgent, "难度级别为高级的学习方法", map[string]interface{}{
				"sections.*.difficulty": "高级",
			})
		} else if query == metadataQueries[2] {
			// 示例3：数组内容过滤
			result, err = executeFilteredQuery(ctx, ragAgent, "监督学习算法", map[string]interface{}{
				"sections.监督学习.algorithms": map[string]interface{}{
					"$in": []string{"线性回归", "逻辑回归"},
				},
			})
		}

		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}
		fmt.Printf("回答: %s\n", result)
	}
}

// 执行带过滤的查询
func executeFilteredQuery(ctx context.Context, agent *agent.Agent, query string, filters map[string]interface{}) (string, error) {
	// 构建带有过滤器的查询
	params := map[string]interface{}{
		"query":   query,
		"filters": filters,
	}

	// 在Agent中执行查询
	return agent.ExecuteWithParams(ctx, "document_search", params)
}
