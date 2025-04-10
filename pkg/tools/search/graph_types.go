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

// GraphEdge 图边
type GraphEdge struct {
	Source       string  `json:"source"`
	Target       string  `json:"target"`
	Weight       float32 `json:"weight"`
	Relationship string  `json:"relationship,omitempty"`
}

// KnowledgeGraph 知识图谱
type KnowledgeGraph struct {
	Nodes     []*GraphNode `json:"nodes"`
	Edges     []*GraphEdge `json:"edges"`
	QueryNode *GraphNode   `json:"query_node,omitempty"`
}

// GraphRAGOptions Graph RAG选项
type GraphRAGOptions struct {
	// 嵌入向量提供者
	EmbeddingProvider EmbeddingProvider
	// 向量搜索工具
	VectorSearchTool *VectorSearchTool
	// 图构建阈值
	Threshold float32
	// 向量维度
	Dimension int
	// 最大节点数
	MaxNodes int
	// 获取相关度函数
	GetRelatedness func(text1, text2 string) (float32, error)
}
