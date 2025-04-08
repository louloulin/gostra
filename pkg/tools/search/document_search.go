package search

import (
	"errors"
	"fmt"

	"github.com/yourusername/gostra/pkg/tools"
	"github.com/yourusername/gostra/pkg/tools/document"
)

// DocumentSearchOptions 文档搜索工具选项
type DocumentSearchOptions struct {
	// 向量搜索工具
	VectorSearchTool *VectorSearchTool
	// 文档分块工具
	DocumentChunker *document.DocumentChunker
	// 嵌入向量提供者
	EmbeddingProvider EmbeddingProvider
	// 块大小
	ChunkSize int
	// 块重叠
	ChunkOverlap int
	// 分块策略
	ChunkStrategy document.ChunkStrategy
}

// DocumentSearchTool 文档搜索工具
type DocumentSearchTool struct {
	options DocumentSearchOptions
	id      string
	desc    string
	schema  tools.Schema
}

// NewDocumentSearchTool 创建新的文档搜索工具
func NewDocumentSearchTool(options DocumentSearchOptions) *DocumentSearchTool {
	// 设置默认值
	if options.ChunkSize == 0 {
		options.ChunkSize = 1000
	}
	if options.ChunkStrategy == "" {
		options.ChunkStrategy = document.StrategyRecursive
	}

	// 创建输入模式
	schema := tools.NewSimpleSchema(tools.TypeObject, "文档搜索参数")

	// 查询参数
	querySchema := tools.NewSimpleSchema(tools.TypeString, "搜索查询文本")
	schema.AddProperty("query", querySchema, true)

	// 文档内容参数
	contentSchema := tools.NewSimpleSchema(tools.TypeString, "要搜索的文档内容")
	schema.AddProperty("content", contentSchema, false)

	// 文档ID参数
	documentIDSchema := tools.NewSimpleSchema(tools.TypeString, "文档ID（如果已添加文档）")
	schema.AddProperty("document_id", documentIDSchema, false)

	// 结果数量参数
	topKSchema := tools.NewSimpleSchema(tools.TypeInteger, "返回结果数量")
	schema.AddProperty("top_k", topKSchema, false)

	// 相似度阈值参数
	thresholdSchema := tools.NewSimpleSchema(tools.TypeNumber, "相似度阈值（0-1），低于此值的结果将被过滤")
	schema.AddProperty("threshold", thresholdSchema, false)

	// 过滤器参数
	filtersSchema := tools.NewSimpleSchema(tools.TypeObject, "元数据过滤条件")
	schema.AddProperty("filters", filtersSchema, false)

	return &DocumentSearchTool{
		options: options,
		id:      "document_search",
		desc:    "对文档内容进行语义搜索，返回相关文档块",
		schema:  schema,
	}
}

// GetID 返回工具ID
func (t *DocumentSearchTool) GetID() string {
	return t.id
}

// GetDescription 返回工具描述
func (t *DocumentSearchTool) GetDescription() string {
	return t.desc
}

// GetInputSchema 返回输入架构
func (t *DocumentSearchTool) GetInputSchema() tools.Schema {
	return t.schema
}

// AddDocument 添加文档到搜索索引
func (t *DocumentSearchTool) AddDocument(content string, documentID string, metadata map[string]interface{}) error {
	if t.options.VectorSearchTool == nil {
		return errors.New("未设置向量搜索工具")
	}
	if t.options.DocumentChunker == nil {
		return errors.New("未设置文档分块工具")
	}

	// 准备分块参数
	chunkParams := document.ChunkParams{
		Strategy: t.options.ChunkStrategy,
		Size:     t.options.ChunkSize,
		Overlap:  t.options.ChunkOverlap,
	}

	// 构建Execute参数
	executeParams := map[string]interface{}{
		"content":  content,
		"strategy": string(chunkParams.Strategy),
		"size":     float64(chunkParams.Size),
		"overlap":  float64(chunkParams.Overlap),
	}

	// 分块文档
	chunker := t.options.DocumentChunker
	result, err := chunker.Execute(executeParams, nil)
	if err != nil {
		return fmt.Errorf("分块文档失败: %w", err)
	}

	// 转换结果为DocumentChunk切片
	chunks, ok := result.([]*document.DocumentChunk)
	if !ok {
		return fmt.Errorf("分块结果类型错误")
	}

	// 设置文档ID和元数据
	for _, chunk := range chunks {
		chunk.DocumentID = documentID
		chunk.Metadata = metadata
	}

	// 提取文本内容，用于生成嵌入向量
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = chunk.Content
	}

	// 获取嵌入向量
	if t.options.EmbeddingProvider == nil {
		return errors.New("未设置嵌入向量提供者")
	}
	embeddings, err := t.options.EmbeddingProvider.GetEmbeddings(texts)
	if err != nil {
		return fmt.Errorf("获取嵌入向量失败: %w", err)
	}

	// 添加到索引
	for i, chunk := range chunks {
		id := fmt.Sprintf("%s-%d", chunk.DocumentID, i)
		if err := t.options.VectorSearchTool.AddItem(id, chunk, embeddings[i], chunk.Metadata); err != nil {
			return err
		}
	}

	return nil
}

// Execute 执行工具
func (t *DocumentSearchTool) Execute(params map[string]interface{}, options *tools.ExecuteOptions) (interface{}, error) {
	if t.options.VectorSearchTool == nil {
		return nil, errors.New("未设置向量搜索工具")
	}

	// 获取查询文本
	query, ok := params["query"].(string)
	if !ok || query == "" {
		return nil, errors.New("必须提供非空的查询文本")
	}

	// 处理新文档内容（如果提供）
	if content, ok := params["content"].(string); ok && content != "" {
		documentID := "temp-doc"
		if docID, ok := params["document_id"].(string); ok && docID != "" {
			documentID = docID
		}

		// 添加文档到索引
		var metadata map[string]interface{}
		if err := t.AddDocument(content, documentID, metadata); err != nil {
			return nil, fmt.Errorf("添加文档失败: %w", err)
		}
	}

	// 获取可选参数
	topK := 5 // 默认返回5个结果
	if topKVal, ok := params["top_k"].(float64); ok && topKVal > 0 {
		topK = int(topKVal)
	}

	threshold := float32(0.0) // 默认不设置阈值
	if thresholdVal, ok := params["threshold"].(float64); ok && thresholdVal > 0 {
		threshold = float32(thresholdVal)
	}

	var filters SearchFilters
	if filtersVal, ok := params["filters"].(map[string]interface{}); ok {
		filters = filtersVal
	}

	// 执行向量搜索
	results, err := t.options.VectorSearchTool.searchByText(query, topK, threshold, filters)
	if err != nil {
		return nil, fmt.Errorf("搜索失败: %w", err)
	}

	return results, nil
}

// ClearDocuments 清空所有文档
func (t *DocumentSearchTool) ClearDocuments() {
	if t.options.VectorSearchTool != nil {
		t.options.VectorSearchTool.Clear()
	}
}

// DeleteDocument 删除指定ID的文档
func (t *DocumentSearchTool) DeleteDocument(documentID string) bool {
	if t.options.VectorSearchTool == nil {
		return false
	}

	// 删除包含此文档ID的所有向量项
	deleted := false
	for id, chunk := range t.options.VectorSearchTool.chunks {
		if chunk.DocumentID == documentID {
			if t.options.VectorSearchTool.DeleteItem(id) {
				deleted = true
			}
		}
	}

	return deleted
}
