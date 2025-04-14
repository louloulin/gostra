package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/louloulin/gostra/pkg/actor"
	"github.com/louloulin/gostra/pkg/agent"
	"github.com/louloulin/gostra/pkg/memory"
	"github.com/louloulin/gostra/pkg/models/openai"
	"github.com/louloulin/gostra/pkg/tools/document"
	"github.com/louloulin/gostra/pkg/tools/search"
)

// 计算余弦相似度
func cosineSimilarity(vec1, vec2 []float32) float32 {
	if len(vec1) != len(vec2) {
		return 0
	}

	var dotProduct, norm1, norm2 float32
	for i := 0; i < len(vec1); i++ {
		dotProduct += vec1[i] * vec2[i]
		norm1 += vec1[i] * vec1[i]
		norm2 += vec2[i] * vec2[i]
	}

	if norm1 == 0 || norm2 == 0 {
		return 0
	}

	return dotProduct / (float32(sqrt(float64(norm1))) * float32(sqrt(float64(norm2))))
}

// 计算平方根
func sqrt(x float64) float64 {
	return float64(x)
}

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
	system := actor.NewActorSystem(actor.ActorSystemConfig{
		Name: "graph-rag-example",
	})

	// 创建OpenAI提供者
	openaiProvider, err := openai.NewOpenAIProvider(&openai.Options{
		APIKey: openaiKey,
		Model:  "gpt-4o",
	})
	if err != nil {
		log.Fatalf("Failed to create OpenAI provider: %v", err)
	}

	// 创建嵌入向量提供者
	embeddingProvider := &openai.OpenAIEmbedder{
		APIKey: openaiKey,
		Model:  "text-embedding-3-small",
	}

	// 创建PostgreSQL向量存储
	pgVectorStore, err := memory.NewPostgresVectorStorage(memory.PostgresVectorOptions{
		ConnectionString: pgConnStr,
		TableName:        "graph_rag_example",
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

	// 创建Graph RAG工具
	graphRAGTool := search.NewGraphRAGTool(search.GraphRAGOptions{
		EmbeddingProvider: embeddingProvider,
		VectorSearchTool:  vectorSearchTool,
		Threshold:         0.7,
		Dimension:         1536,
		MaxNodes:          15,
		GetRelatedness: func(text1, text2 string) (float32, error) {
			// 使用OpenAI提供者计算文本相关度
			emb1, err := embeddingProvider.GetEmbedding(text1)
			if err != nil {
				return 0, err
			}
			emb2, err := embeddingProvider.GetEmbedding(text2)
			if err != nil {
				return 0, err
			}
			// Ensure the return type is float32
			return float32(cosineSimilarity(emb1, emb2)), nil
		},
	})

	// 创建文档搜索工具
	documentSearchTool := search.NewDocumentSearchTool(search.DocumentSearchOptions{
		VectorSearchTool:  vectorSearchTool,
		DocumentChunker:   documentChunker,
		EmbeddingProvider: embeddingProvider,
		ChunkSize:         500,
		ChunkOverlap:      100,
		ChunkStrategy:     document.StrategyRecursive,
	})

	// 创建存储文档的函数，使用PostgreSQL
	storeDocumentFunction := func(chunks []*document.DocumentChunk) error {
		// 提取文本内容
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

	// 创建RAG (Retrieval Augmented Generation) Agent
	graphRAGAgent := agent.NewAgent(&agent.AgentConfig{
		Name:          "graph-rag-agent",
		ModelProvider: openaiProvider,
		Tools:         []agent.Tool{graphRAGTool},
		Instructions: `你是一个擅长使用Graph RAG技术回答问题的助手。
Graph RAG能够构建文档块之间的知识图谱，并使用PageRank等算法找到最相关的信息。
请按以下格式回答:

1. 直接事实：列出从文本中直接提取的与问题相关的事实（2-3个要点）
2. 关联发现：列出你通过知识图谱发现的不同部分文本之间的关系（2-3个要点）
3. 结论：一句话总结，将所有内容联系起来

请保持简洁，专注于最重要的信息。`,
	})

	// 创建文档处理Agent
	documentAgent := agent.NewAgent(&agent.AgentConfig{
		Name:          "document-agent",
		ModelProvider: openaiProvider,
		Tools:         []agent.Tool{documentSearchTool},
		Instructions: `你是一个擅长处理和分析文档的助手。
你可以使用文档搜索工具在已添加的文档中查找相关信息。
请简洁明了地回答用户的问题，只基于文档内容，不要添加额外信息。`,
	})

	// 注册Agent
	system.RegisterAgent("graph-rag-agent", graphRAGAgent)
	system.RegisterAgent("document-agent", documentAgent)

	// 启动Actor系统
	system.Start()
	defer system.Stop()

	// 样本文档内容
	sampleDocument := `# 城市发展历史：河谷镇案例研究

## 1. 早期定居点 (1800-1850)
河谷镇最初由欧洲移民在1823年建立。这个地区被选中是因为它靠近河流提供了良好的交通和灌溉条件。最初的定居者主要是农民和工匠，他们建立了一个自给自足的小社区。到1840年，这个定居点已经发展到约300人。

## 2. 工业化起步 (1850-1900)
1853年，第一个水力磨坊在河谷建立，标志着工业化的开始。这一发展吸引了更多的工人和商人迁入该地区。1870年，铁路连接到河谷镇，极大地促进了当地经济的发展。到19世纪末，河谷镇已经成为该地区的一个重要工业中心，人口增长到约3,000人。

以下是这一时期的关键发展：
- 1853年：第一个水力磨坊建立
- 1862年：河谷银行成立，为当地企业提供资金
- 1870年：铁路到达，连接河谷与主要城市
- 1878年：第一所公立学校建成
- 1885年：煤矿开始运营，为工业提供能源
- 1891年：人口突破2,500人

## 3. 工业繁荣期 (1900-1950)
20世纪上半叶是河谷镇的工业繁荣期。钢铁厂、纺织厂和机械制造厂在这一时期建立，为数千人提供就业机会。两次世界大战期间，当地工业为战争提供了大量物资，进一步推动了经济发展。

工业繁荣带来的重要变化：
- 城市基础设施：新建了道路、电力系统和自来水系统
- 住房发展：为工人家庭建造了大量住宅区
- 公共设施：新建了医院、图书馆和休闲设施
- 人口构成：来自不同国家的移民使城市人口更加多样化
- 社会问题：工业污染和劳工条件开始引起关注

到1950年，河谷镇的人口达到约25,000人，成为该地区的一个重要城市。

## 4. 经济转型期 (1950-2000)
从20世纪50年代开始，全球工业格局的变化逐渐影响到河谷镇。传统工业开始衰退，许多工厂关闭或迁移到劳动力成本更低的地区。这一时期的主要挑战和应对策略包括：

### 挑战
- 工业衰退：主要工厂关闭，导致失业率上升
- 人口流失：许多年轻人离开寻找更好的机会
- 基础设施老化：工业时代建设的基础设施需要更新
- 环境问题：工业污染留下的遗留问题需要解决

### 应对策略
- 经济多元化：吸引新型企业，特别是服务业和轻工业
- 教育投资：扩建社区学院，提供再培训项目
- 城市更新：重新开发废弃的工业区
- 旅游发展：利用工业遗产发展文化旅游

到2000年，河谷镇已经部分实现了经济转型，人口稳定在约20,000人。

## 5. 现代发展 (2000至今)
21世纪的河谷镇继续适应全球化和数字化的挑战。主要发展包括：

- 技术创新：建立小型科技园区，吸引创新企业
- 可持续发展：实施绿色基础设施和可再生能源项目
- 文化复兴：利用历史建筑发展艺术和文化场所
- 社区参与：增加公民参与城市规划和决策过程

今天的河谷镇是一个人口多样、经济相对稳定的小城市，正努力在保持其历史特色的同时适应21世纪的挑战。

## 6. 城市发展的关键影响因素
纵观河谷镇的历史，几个关键因素持续影响了其发展轨迹：

### 地理位置
河谷镇的位置靠近水源和自然资源，最初吸引了定居者，并持续影响其发展可能性。

### 交通连接
从早期的河流，到铁路，再到现代公路网络，交通连接一直是城市发展的关键因素。

### 经济基础
经济基础的变化——从农业到工业再到服务业和知识经济——塑造了城市的物理形态和社会结构。

### 领导和规划
不同时期的城市领导和规划决策，无论好坏，都对城市的长期发展产生了深远影响。

### 外部事件
战争、经济衰退、技术变革等外部事件经常触发城市发展的转折点。

## 7. 未来展望
展望未来，河谷镇面临着一系列挑战和机遇：

### 挑战
- 人口老龄化
- 全球经济竞争
- 气候变化影响
- 基础设施更新需求

### 机遇
- 发展知识经济和创意产业
- 利用历史和文化吸引游客和新居民
- 实施智慧城市技术提高生活质量
- 发展绿色产业和可持续实践

河谷镇的故事表明，成功的城市能够适应变化、利用其独特资产并持续投资于居民的福祉。`

	// 添加示例文档
	err = documentSearchTool.AddDocument(sampleDocument, "riverdale-history", map[string]interface{}{
		"title":    "城市发展历史：河谷镇案例研究",
		"type":     "case-study",
		"period":   "1800-present",
		"theme":    "urban development",
		"keywords": []string{"urban", "industrial", "economic transition", "community"},
		"time_periods": map[string]interface{}{
			"early_settlement": map[string]interface{}{
				"start_year": "1800",
				"end_year":   "1850",
				"population": "300",
			},
			"industrialization": map[string]interface{}{
				"start_year": "1850",
				"end_year":   "1900",
				"population": "3000",
			},
			"industrial_boom": map[string]interface{}{
				"start_year": "1900",
				"end_year":   "1950",
				"population": "25000",
			},
			"transition": map[string]interface{}{
				"start_year": "1950",
				"end_year":   "2000",
				"population": "20000",
			},
			"modern": map[string]interface{}{
				"start_year": "2000",
				"end_year":   "present",
				"population": "20000",
			},
		},
	})
	if err != nil {
		log.Fatalf("Failed to add document: %v", err)
	}

	// 准备演示查询
	demoQueries := []string{
		"河谷镇的历史发展经历了哪些主要阶段？",
		"铁路对河谷镇的发展有什么影响？请通过知识图谱分析直接和间接影响。",
		"河谷镇如何应对工业衰退？这些策略和现代发展有什么关联？",
		"地理位置和交通因素如何塑造了河谷镇的发展历史？",
	}

	// 使用Graph RAG执行查询
	fmt.Println("===== Graph RAG查询示例 =====")
	for _, query := range demoQueries {
		fmt.Printf("\n问题: %s\n", query)

		// 使用Graph RAG获取知识图谱和查询结果
		params := map[string]interface{}{
			"query":     query,
			"top_k":     5,
			"threshold": 0.6,
			"method":    "simple",
		}

		// 发送GraphRAG查询消息到Agent
		actorContext := actor.NewContext(ctx)
		response, err := system.SendAndReceive(actorContext, "graph-rag-agent", "Execute", "graph_rag", params)
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}

		// 整理查询结果
		graphResponse, ok := response.(map[string]interface{})
		if !ok {
			log.Printf("Error: unexpected response type")
			continue
		}

		// 使用图结果生成最终回答
		graphStr := formatGraphResults(graphResponse)

		// 构建带有图结果的提示
		prompt := fmt.Sprintf(`请基于以下知识图谱结果回答问题：
问题: %s

知识图谱结果:
%s

请按要求的格式回答：
1. 直接事实：
2. 关联发现：
3. 结论：`, query, graphStr)

		// 获取最终回答
		finalResponse, err := system.SendAndReceive(actorContext, "graph-rag-agent", "Generate", prompt, nil)
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}

		// 打印回答
		finalText, ok := finalResponse.(string)
		if !ok {
			log.Printf("Error: unexpected final response type")
			continue
		}

		fmt.Printf("回答:\n%s\n", finalText)
		fmt.Println(strings.Repeat("-", 80))
	}
}

// 格式化图查询结果
func formatGraphResults(graphResponse map[string]interface{}) string {
	var result strings.Builder

	// 获取结果节点
	resultList, ok := graphResponse["results"].([]interface{})
	if !ok || len(resultList) == 0 {
		return "没有找到相关结果。"
	}

	result.WriteString("找到的相关内容：\n\n")

	// 处理每个结果节点
	for i, nodeInterface := range resultList {
		node, ok := nodeInterface.(map[string]interface{})
		if !ok {
			continue
		}

		content, _ := node["content"].(string)
		score, _ := node["score"].(float64)

		result.WriteString(fmt.Sprintf("%d. 相关性: %.2f\n   内容: %s\n\n", i+1, score, content))
	}

	return result.String()
}
