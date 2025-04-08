package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/yourusername/gostra/pkg/tools/document"
	"github.com/yourusername/gostra/pkg/tools/search"
)

// PostgresVectorIntegration 提供PostgreSQL向量存储与搜索工具的集成
type PostgresVectorIntegration struct {
	vectorStore *PostgresVectorStore
	embedding   search.EmbeddingProvider
}

// NewPostgresVectorIntegration 创建新的PostgreSQL向量存储集成
func NewPostgresVectorIntegration(store *PostgresVectorStore, provider search.EmbeddingProvider) *PostgresVectorIntegration {
	return &PostgresVectorIntegration{
		vectorStore: store,
		embedding:   provider,
	}
}

// CreateSearchTool 创建向量搜索工具
func (p *PostgresVectorIntegration) CreateSearchTool() (*search.VectorSearchTool, error) {
	if p.vectorStore == nil {
		return nil, errors.New("未设置向量存储")
	}
	if p.embedding == nil {
		return nil, errors.New("未设置嵌入向量提供者")
	}

	// 创建向量搜索选项
	options := search.VectorSearchOptions{
		EmbeddingProvider: p.embedding,
		IndexType:         search.IndexTypeFlat,
		Dimension:         p.vectorStore.dimension,
		StoreText:         true,
		DistanceMetric:    "cosine",
	}

	// 创建向量搜索工具
	tool := search.NewVectorSearchTool(options)

	// 重新定义 ExecuteWithPostgres 方法，而不是直接替换 Execute
	// 这样可以避免尝试修改非可寻址字段的问题
	p.setupCustomExecutor(tool)

	return tool, nil
}

// setupCustomExecutor 为搜索工具设置自定义执行器
func (p *PostgresVectorIntegration) setupCustomExecutor(tool *search.VectorSearchTool) {
	// 这里不直接修改 tool.Execute，而是在特定情况下使用此函数
	// 实际应用中，需要在外部调用时使用这个自定义执行函数
}

// ExecuteWithPostgres 使用PostgreSQL执行向量搜索
func (p *PostgresVectorIntegration) ExecuteWithPostgres(params map[string]interface{}, execCtx context.Context) ([]*search.VectorSearchResult, error) {
	// 使用原始执行逻辑获取查询向量
	query, ok := params["query"].(string)
	if !ok || query == "" {
		return nil, errors.New("必须提供非空的查询文本")
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

	var filters map[string]interface{}
	if filtersVal, ok := params["filters"].(map[string]interface{}); ok {
		filters = filtersVal
	}

	// 生成查询向量
	queryVector, err := p.embedding.GetEmbedding(query)
	if err != nil {
		return nil, fmt.Errorf("获取查询嵌入向量失败: %w", err)
	}

	// 构造查询向量
	queryVec := Vector{
		Values: queryVector,
	}

	// 执行PostgreSQL搜索
	searchOpts := VectorSearchOptions{
		Limit:     topK,
		Threshold: threshold,
		Filter:    filters,
	}

	// 使用传入的上下文，如果没有则创建新的
	ctx := execCtx
	if ctx == nil {
		ctx = context.Background()
	}

	results, err := p.vectorStore.Search(ctx, queryVec, searchOpts)
	if err != nil {
		return nil, fmt.Errorf("向量搜索失败: %w", err)
	}

	// 将结果转换为VectorSearchResult格式
	searchResults := make([]*search.VectorSearchResult, len(results))
	for i, result := range results {
		// 解析元数据中的chunks
		var chunk *document.DocumentChunk
		if chunkData, ok := result.Vector.Metadata["chunk"]; ok {
			chunkJSON, err := json.Marshal(chunkData)
			if err == nil {
				var docChunk document.DocumentChunk
				if err := json.Unmarshal(chunkJSON, &docChunk); err == nil {
					chunk = &docChunk
				}
			}
		}

		// 如果没有chunk数据或解析失败，创建一个基础chunk
		if chunk == nil {
			content := ""
			if textValue, ok := result.Vector.Metadata["text"]; ok {
				if text, ok := textValue.(string); ok {
					content = text
				}
			}

			chunk = &document.DocumentChunk{
				Content:    content,
				DocumentID: result.Vector.ID,
				Metadata:   result.Vector.Metadata,
			}
		}

		searchResults[i] = &search.VectorSearchResult{
			Chunk:    chunk,
			Score:    result.Score,
			Metadata: result.Vector.Metadata,
		}
	}

	return searchResults, nil
}

// StoreDocumentChunks 存储文档块到PostgreSQL
func (p *PostgresVectorIntegration) StoreDocumentChunks(ctx context.Context, chunks []*document.DocumentChunk) error {
	if p.vectorStore == nil {
		return errors.New("未设置向量存储")
	}
	if p.embedding == nil {
		return errors.New("未设置嵌入向量提供者")
	}
	if len(chunks) == 0 {
		return nil
	}

	// 提取文本内容
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = chunk.Content
	}

	// 获取嵌入向量
	embeddings, err := p.embedding.GetEmbeddings(texts)
	if err != nil {
		return fmt.Errorf("获取嵌入向量失败: %w", err)
	}

	// 准备要存储的向量
	vectors := make([]Vector, len(chunks))
	for i, chunk := range chunks {
		// 生成ID
		id := chunk.DocumentID
		if id == "" {
			id = fmt.Sprintf("chunk-%d", i)
		} else if !strings.Contains(id, "-") {
			id = fmt.Sprintf("%s-%d", id, i)
		}

		// 序列化chunk为JSON以存储在元数据中
		chunkJSON, _ := json.Marshal(chunk)
		var chunkData map[string]interface{}
		_ = json.Unmarshal(chunkJSON, &chunkData)

		// 准备元数据
		metadata := make(map[string]interface{})
		if chunk.Metadata != nil {
			for k, v := range chunk.Metadata {
				metadata[k] = v
			}
		}
		metadata["chunk"] = chunkData
		metadata["text"] = chunk.Content
		metadata["document_id"] = chunk.DocumentID

		vectors[i] = Vector{
			ID:       id,
			Values:   embeddings[i],
			Metadata: metadata,
		}
	}

	// 存储向量
	return p.vectorStore.Store(ctx, vectors)
}

// CreateDocumentSearchTool 创建文档搜索工具
func (p *PostgresVectorIntegration) CreateDocumentSearchTool(chunker *document.DocumentChunker, chunkSize int, chunkOverlap int, chunkStrategy document.ChunkStrategy) (*document.DocumentSearchAdapter, error) {
	// 首先创建向量搜索工具
	vectorTool, err := p.CreateSearchTool()
	if err != nil {
		return nil, err
	}

	// 创建一个适配器来连接PostgreSQL和文档搜索功能
	adapter := &document.DocumentSearchAdapter{
		VectorTool:     vectorTool,
		Chunker:        chunker,
		ChunkSize:      chunkSize,
		ChunkOverlap:   chunkOverlap,
		ChunkStrategy:  chunkStrategy,
		StoreChunkFunc: p.StoreDocumentChunks,
	}

	return adapter, nil
}
