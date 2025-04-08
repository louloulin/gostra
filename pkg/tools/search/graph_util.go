package search

import (
	"fmt"
)

// buildGraph 构建知识图谱
func (t *GraphRAGTool) buildGraph(query string, results []*VectorSearchResult, threshold float32, method string) (*KnowledgeGraph, error) {
	if len(results) == 0 {
		return &KnowledgeGraph{
			Nodes: []*GraphNode{},
			Edges: []*GraphEdge{},
		}, nil
	}

	// 创建查询节点
	queryNode := &GraphNode{
		ID:      "query",
		Content: query,
		Score:   1.0,
	}

	// 创建节点列表
	nodes := make([]*GraphNode, 0, len(results)+1)
	nodes = append(nodes, queryNode)

	for i, result := range results {
		nodeID := fmt.Sprintf("node-%d", i)
		if result.Chunk != nil && result.Chunk.DocumentID != "" {
			nodeID = fmt.Sprintf("%s-%d", result.Chunk.DocumentID, i)
		}

		content := ""
		if result.Chunk != nil {
			content = result.Chunk.Content
		}

		node := &GraphNode{
			ID:       nodeID,
			Content:  content,
			Score:    result.Score,
			Chunk:    result.Chunk,
			Metadata: result.Metadata,
		}
		nodes = append(nodes, node)
	}

	// 根据方法构建边
	var edges []*GraphEdge
	switch method {
	case "community":
		edges = t.buildCommunityEdges(nodes, threshold)
	default:
		edges = t.buildSimpleEdges(nodes, threshold)
	}

	graph := &KnowledgeGraph{
		Nodes:     nodes,
		Edges:     edges,
		QueryNode: queryNode,
	}

	return graph, nil
}

// buildSimpleEdges 构建简单连接边
func (t *GraphRAGTool) buildSimpleEdges(nodes []*GraphNode, threshold float32) []*GraphEdge {
	if len(nodes) <= 1 {
		return []*GraphEdge{}
	}

	edges := make([]*GraphEdge, 0)
	queryNode := nodes[0] // 假设查询节点是第一个

	// 连接查询节点和所有其他节点
	for i := 1; i < len(nodes); i++ {
		resultNode := nodes[i]
		if resultNode.Score < threshold {
			continue
		}

		edge := &GraphEdge{
			Source: queryNode.ID,
			Target: resultNode.ID,
			Weight: resultNode.Score,
		}
		edges = append(edges, edge)
	}

	// 计算结果节点之间的相似度
	for i := 1; i < len(nodes); i++ {
		for j := i + 1; j < len(nodes); j++ {
			node1 := nodes[i]
			node2 := nodes[j]

			if t.options.GetRelatedness != nil {
				relatedness, err := t.options.GetRelatedness(node1.Content, node2.Content)
				if err == nil && relatedness >= threshold {
					edge := &GraphEdge{
						Source: node1.ID,
						Target: node2.ID,
						Weight: relatedness,
					}
					edges = append(edges, edge)
				}
			} else {
				// 简单估计相似度
				combinedScore := (node1.Score + node2.Score) / 2
				if combinedScore >= threshold {
					edge := &GraphEdge{
						Source: node1.ID,
						Target: node2.ID,
						Weight: combinedScore,
					}
					edges = append(edges, edge)
				}
			}
		}
	}

	return edges
}

// buildCommunityEdges 构建社区连接边
func (t *GraphRAGTool) buildCommunityEdges(nodes []*GraphNode, threshold float32) []*GraphEdge {
	// 基础连接与简单边相同
	edges := t.buildSimpleEdges(nodes, threshold)

	// 基于相似度计算社区结构
	// 这里简化实现，真实社区检测可能需要更复杂的算法

	// 返回构建的边
	return edges
}
