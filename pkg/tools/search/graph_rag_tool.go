package search

import (
	"errors"
	"fmt"
	"sort"

	"github.com/yourusername/gostra/pkg/tools"
)

// GraphRAGTool Graph RAG工具
type GraphRAGTool struct {
	options GraphRAGOptions
	id      string
	desc    string
	schema  tools.Schema
}

// NewGraphRAGTool 创建Graph RAG工具
func NewGraphRAGTool(options GraphRAGOptions) *GraphRAGTool {
	// 设置默认值
	if options.Threshold == 0 {
		options.Threshold = 0.7
	}
	if options.MaxNodes == 0 {
		options.MaxNodes = 10
	}
	if options.Dimension == 0 {
		options.Dimension = 1536
	}

	// 创建输入模式
	schema := tools.NewSimpleSchema(tools.TypeObject, "Graph RAG参数")

	// 查询参数
	querySchema := tools.NewSimpleSchema(tools.TypeString, "搜索查询文本")
	schema.AddProperty("query", querySchema, true)

	// 文档内容参数
	contentSchema := tools.NewSimpleSchema(tools.TypeString, "要搜索的文档内容")
	schema.AddProperty("content", contentSchema, false)

	// 结果数量参数
	topKSchema := tools.NewSimpleSchema(tools.TypeInteger, "返回结果数量")
	schema.AddProperty("top_k", topKSchema, false)

	// 相似度阈值参数
	thresholdSchema := tools.NewSimpleSchema(tools.TypeNumber, "相似度阈值（0-1）")
	schema.AddProperty("threshold", thresholdSchema, false)

	// 过滤器参数
	filtersSchema := tools.NewSimpleSchema(tools.TypeObject, "元数据过滤条件")
	schema.AddProperty("filters", filtersSchema, false)

	// 构建方法参数
	methodSchema := tools.NewSimpleSchema(tools.TypeString, "构建方法（simple, community）")
	schema.AddProperty("method", methodSchema, false)

	return &GraphRAGTool{
		options: options,
		id:      "graph_rag",
		desc:    "通过构建知识图谱实现检索增强生成",
		schema:  schema,
	}
}

// GetID 返回工具ID
func (t *GraphRAGTool) GetID() string {
	return t.id
}

// GetDescription 返回工具描述
func (t *GraphRAGTool) GetDescription() string {
	return t.desc
}

// GetInputSchema 返回输入架构
func (t *GraphRAGTool) GetInputSchema() tools.Schema {
	return t.schema
}

// Execute 执行工具
func (t *GraphRAGTool) Execute(params map[string]interface{}, options *tools.ExecuteOptions) (interface{}, error) {
	// 验证参数
	query, ok := params["query"].(string)
	if !ok || query == "" {
		return nil, errors.New("必须提供非空的查询文本")
	}

	// 获取可选参数
	topK := 5 // 默认返回5个结果
	if topKVal, ok := params["top_k"].(float64); ok && topKVal > 0 {
		topK = int(topKVal)
	}

	threshold := t.options.Threshold
	if thresholdVal, ok := params["threshold"].(float64); ok && thresholdVal > 0 {
		threshold = float32(thresholdVal)
	}

	var filters map[string]interface{}
	if filtersVal, ok := params["filters"].(map[string]interface{}); ok {
		filters = filtersVal
	}

	method := "simple" // 默认构建方法
	if methodVal, ok := params["method"].(string); ok && methodVal != "" {
		method = methodVal
	}

	// 处理新文档内容（如果提供）
	if content, ok := params["content"].(string); ok && content != "" && t.options.VectorSearchTool != nil {
		documentID := "temp-graphrag-doc"
		addContentParams := map[string]interface{}{
			"content":     content,
			"document_id": documentID,
		}
		_, err := t.options.VectorSearchTool.Execute(addContentParams, options)
		if err != nil {
			return nil, fmt.Errorf("添加内容失败: %w", err)
		}
	}

	// 执行向量搜索
	searchParams := map[string]interface{}{
		"query":     query,
		"top_k":     float64(topK * 2), // 获取更多结果用于构建图
		"threshold": float64(threshold),
		"filters":   filters,
	}

	var searchResults []*VectorSearchResult
	if t.options.VectorSearchTool != nil {
		result, err := t.options.VectorSearchTool.Execute(searchParams, options)
		if err != nil {
			return nil, fmt.Errorf("向量搜索失败: %w", err)
		}
		var ok bool
		searchResults, ok = result.([]*VectorSearchResult)
		if !ok {
			return nil, errors.New("搜索结果类型错误")
		}
	} else {
		return nil, errors.New("未配置向量搜索工具")
	}

	// 构建知识图谱
	graph, err := t.buildGraph(query, searchResults, threshold, method)
	if err != nil {
		return nil, fmt.Errorf("构建知识图谱失败: %w", err)
	}

	// 执行图遍历
	traversalResults, err := t.traverseGraph(graph, query, topK)
	if err != nil {
		return nil, fmt.Errorf("图遍历失败: %w", err)
	}

	// 返回最终结果
	return map[string]interface{}{
		"graph":   graph,
		"results": traversalResults,
	}, nil
}

// traverseGraph 遍历知识图谱
func (t *GraphRAGTool) traverseGraph(graph *KnowledgeGraph, query string, topK int) ([]*GraphNode, error) {
	if len(graph.Nodes) <= 1 || topK <= 0 {
		return []*GraphNode{}, nil
	}

	// 使用PageRank算法计算节点重要性
	nodeScores := t.calculatePageRank(graph)

	// 基于PageRank得分和相似度排序节点
	resultNodes := make([]*GraphNode, 0, len(graph.Nodes)-1)
	for i := 1; i < len(graph.Nodes); i++ { // 跳过查询节点
		node := graph.Nodes[i]

		// 结合PageRank和相似度得分
		if pageRankScore, ok := nodeScores[node.ID]; ok {
			// 更新节点得分，结合原始相似度和PageRank
			node.Score = node.Score*0.7 + float32(pageRankScore)*0.3
		}

		resultNodes = append(resultNodes, node)
	}

	// 排序结果
	sort.Slice(resultNodes, func(i, j int) bool {
		return resultNodes[i].Score > resultNodes[j].Score
	})

	// 返回前topK个结果
	if len(resultNodes) > topK {
		resultNodes = resultNodes[:topK]
	}

	return resultNodes, nil
}

// calculatePageRank 计算PageRank
func (t *GraphRAGTool) calculatePageRank(graph *KnowledgeGraph) map[string]float64 {
	// 初始化得分
	scores := make(map[string]float64)
	nodeCount := len(graph.Nodes)

	// 初始得分均匀分布
	initialScore := 1.0 / float64(nodeCount)
	for _, node := range graph.Nodes {
		scores[node.ID] = initialScore
	}

	// 构建邻接表
	outLinks := make(map[string][]string)
	inLinks := make(map[string]map[string]float32)

	for _, edge := range graph.Edges {
		if _, ok := outLinks[edge.Source]; !ok {
			outLinks[edge.Source] = make([]string, 0)
		}
		outLinks[edge.Source] = append(outLinks[edge.Source], edge.Target)

		if _, ok := inLinks[edge.Target]; !ok {
			inLinks[edge.Target] = make(map[string]float32)
		}
		inLinks[edge.Target][edge.Source] = edge.Weight
	}

	// PageRank迭代
	dampingFactor := 0.85
	iterations := 20

	for iter := 0; iter < iterations; iter++ {
		newScores := make(map[string]float64)

		// 每个节点的基础得分
		randomJumpScore := (1.0 - dampingFactor) / float64(nodeCount)

		for _, node := range graph.Nodes {
			nodeID := node.ID
			newScores[nodeID] = randomJumpScore

			// 来自入链的得分
			if sourceNodes, ok := inLinks[nodeID]; ok {
				for sourceID, weight := range sourceNodes {
					// 计算源节点的出链得分
					sourceScore := scores[sourceID]
					outLinkCount := float64(len(outLinks[sourceID]))
					if outLinkCount > 0 {
						weightFactor := float64(weight) // 考虑边权重
						newScores[nodeID] += dampingFactor * sourceScore * weightFactor / outLinkCount
					}
				}
			}
		}

		// 更新得分
		scoreSum := 0.0
		for id, score := range newScores {
			scoreSum += score
		}

		// 归一化
		if scoreSum > 0 {
			for id, score := range newScores {
				scores[id] = score / scoreSum
			}
		} else {
			// 如果总分为0，则均匀分布
			for id := range newScores {
				scores[id] = 1.0 / float64(nodeCount)
			}
		}
	}

	return scores
}
