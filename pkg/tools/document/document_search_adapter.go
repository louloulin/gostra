package document

import (
	"context"
	"errors"
	"fmt"

	"github.com/yourusername/gostra/pkg/tools"
	"github.com/yourusername/gostra/pkg/tools/search"
)

// DocumentSearchAdapter 是一个接口，定义了文档搜索所需的方法
type DocumentSearchAdapter interface {
	// Search 在文档中搜索内容，返回匹配的文档块及其相关性分数
	// query: 搜索查询
	// topK: 返回结果的最大数量
	// threshold: 相似度阈值，低于此值的结果会被过滤
	// filters: 可选的元数据过滤条件
	Search(ctx context.Context, query string, topK int, threshold float64, filters map[string]interface{}) ([]SearchResult, error)

	// LoadDocument 加载文档到搜索适配器中
	// doc: 要加载的文档
	// metadata: 可选的文档元数据
	LoadDocument(ctx context.Context, doc *ProcessedDocument, metadata map[string]interface{}) error

	// GetDocuments 获取已加载的所有文档
	GetDocuments(ctx context.Context) ([]*ProcessedDocument, error)

	// DeleteDocument 从搜索适配器中删除文档
	// docID: 要删除的文档ID
	DeleteDocument(ctx context.Context, docID string) error

	// Clear 清除所有文档
	Clear(ctx context.Context) error
}

// SearchResult 表示文档搜索的结果
type SearchResult struct {
	// Content 是匹配的文档内容
	Content string `json:"content"`

	// Score 是相似度分数
	Score float64 `json:"score"`

	// Metadata 是文档元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Document 是原始文档的引用（可选）
	Document *ProcessedDocument `json:"document,omitempty"`

	// ChunkIndex 是分块索引（如适用）
	ChunkIndex int `json:"chunk_index,omitempty"`
}

// DocumentSearchAdapter 文档搜索适配器
// 连接PostgreSQL向量存储与文档搜索功能
type DocumentSearchAdapterImpl struct {
	// 向量搜索工具
	VectorTool *search.VectorSearchTool
	// 文档分块工具
	Chunker *DocumentChunker
	// 块大小
	ChunkSize int
	// 块重叠
	ChunkOverlap int
	// 分块策略
	ChunkStrategy ChunkStrategy
	// 存储块的函数
	StoreChunkFunc func(ctx context.Context, chunks []*DocumentChunk) error
	// 工具ID
	ID string
	// 工具描述
	Description string
	// 输入模式
	Schema tools.Schema
}

// NewDocumentSearchAdapter 创建一个新的文档搜索适配器
func NewDocumentSearchAdapter(vectorTool *search.VectorSearchTool, chunker *DocumentChunker, storeFunc func(ctx context.Context, chunks []*DocumentChunk) error) *DocumentSearchAdapterImpl {
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

	return &DocumentSearchAdapterImpl{
		VectorTool:     vectorTool,
		Chunker:        chunker,
		ChunkSize:      1000, // 默认大小
		ChunkOverlap:   100,  // 默认重叠
		ChunkStrategy:  StrategyRecursive,
		StoreChunkFunc: storeFunc,
		ID:             "document_search",
		Description:    "对文档内容进行语义搜索，返回相关文档块",
		Schema:         schema,
	}
}

// GetID 返回工具ID
func (a *DocumentSearchAdapterImpl) GetID() string {
	if a.ID == "" {
		return "document_search"
	}
	return a.ID
}

// GetDescription 返回工具描述
func (a *DocumentSearchAdapterImpl) GetDescription() string {
	if a.Description == "" {
		return "对文档内容进行语义搜索，返回相关文档块"
	}
	return a.Description
}

// GetInputSchema 返回输入架构
func (a *DocumentSearchAdapterImpl) GetInputSchema() tools.Schema {
	return a.Schema
}

// AddDocument 添加文档到搜索索引
func (a *DocumentSearchAdapterImpl) AddDocument(ctx context.Context, content string, documentID string, metadata map[string]interface{}) error {
	if a.VectorTool == nil {
		return errors.New("未设置向量搜索工具")
	}
	if a.Chunker == nil {
		return errors.New("未设置文档分块工具")
	}
	if a.StoreChunkFunc == nil {
		return errors.New("未设置存储块函数")
	}

	// 准备分块参数
	executeParams := map[string]interface{}{
		"content":  content,
		"strategy": string(a.ChunkStrategy),
		"size":     float64(a.ChunkSize),
		"overlap":  float64(a.ChunkOverlap),
	}

	// 分块文档
	result, err := a.Chunker.Execute(executeParams, &tools.ExecuteOptions{
		Context: ctx,
	})
	if err != nil {
		return fmt.Errorf("分块文档失败: %w", err)
	}

	// 转换结果为DocumentChunk切片
	chunks, ok := result.([]*DocumentChunk)
	if !ok {
		return fmt.Errorf("分块结果类型错误")
	}

	// 设置文档ID和元数据
	for _, chunk := range chunks {
		chunk.DocumentID = documentID
		if metadata != nil {
			if chunk.Metadata == nil {
				chunk.Metadata = make(map[string]interface{})
			}
			for k, v := range metadata {
				chunk.Metadata[k] = v
			}
		}
	}

	// 使用提供的函数存储块
	return a.StoreChunkFunc(ctx, chunks)
}

// Execute 执行工具
func (a *DocumentSearchAdapterImpl) Execute(params map[string]interface{}, options *tools.ExecuteOptions) (interface{}, error) {
	if a.VectorTool == nil {
		return nil, errors.New("未设置向量搜索工具")
	}

	ctx := context.Background()
	if options != nil && options.Context != nil {
		var ok bool
		ctx, ok = options.Context.(context.Context)
		if !ok {
			return nil, errors.New("无效的上下文类型")
		}
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
		if err := a.AddDocument(ctx, content, documentID, metadata); err != nil {
			return nil, fmt.Errorf("添加文档失败: %w", err)
		}
	}

	// 准备转发给向量搜索工具的参数
	vectorParams := make(map[string]interface{})
	for k, v := range params {
		vectorParams[k] = v
	}

	// 执行向量搜索
	return a.VectorTool.Execute(vectorParams, options)
}
