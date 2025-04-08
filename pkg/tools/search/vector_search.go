package search

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/yourusername/gostra/pkg/tools"
	"github.com/yourusername/gostra/pkg/tools/common"
)

// EmbeddingProvider 嵌入向量提供者接口
type EmbeddingProvider interface {
	// GetEmbedding 获取文本的嵌入向量
	GetEmbedding(text string) ([]float32, error)
	// GetEmbeddings 批量获取文本的嵌入向量
	GetEmbeddings(texts []string) ([][]float32, error)
}

// VectorIndexType 向量索引类型
type VectorIndexType string

const (
	// IndexTypeFlat 简单向量索引（暴力搜索）
	IndexTypeFlat VectorIndexType = "flat"
	// IndexTypeHNSW 分层导航小世界图索引
	IndexTypeHNSW VectorIndexType = "hnsw"
	// IndexTypeIVF 倒排文件索引
	IndexTypeIVF VectorIndexType = "ivf"
)

// SearchFilters 搜索过滤器
type SearchFilters map[string]interface{}

// VectorSearchResult 向量搜索结果
type VectorSearchResult struct {
	// 文档块
	Chunk *common.DocumentChunk `json:"chunk"`
	// 相似度得分
	Score float32 `json:"score"`
	// 额外元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// VectorSearchOptions 向量搜索选项
type VectorSearchOptions struct {
	// 嵌入向量提供者
	EmbeddingProvider EmbeddingProvider
	// 索引类型
	IndexType VectorIndexType
	// 向量维度
	Dimension int
	// 是否存储原始文本
	StoreText bool
	// 向量距离计算方法（cosine, euclidean, dot）
	DistanceMetric string
}

// VectorSearchTool 向量搜索工具
type VectorSearchTool struct {
	options VectorSearchOptions
	id      string
	desc    string
	schema  tools.Schema
	vectors map[string][]float32              // 向量存储，键为ID
	chunks  map[string]*common.DocumentChunk  // 对应的文档块
	meta    map[string]map[string]interface{} // 元数据
}

// NewVectorSearchTool 创建新的向量搜索工具
func NewVectorSearchTool(options VectorSearchOptions) *VectorSearchTool {
	// 设置默认参数
	if options.IndexType == "" {
		options.IndexType = IndexTypeFlat
	}
	if options.Dimension == 0 {
		options.Dimension = 1536 // 默认维度
	}
	if options.DistanceMetric == "" {
		options.DistanceMetric = "cosine" // 默认余弦相似度
	}

	// 创建输入模式
	schema := tools.NewSimpleSchema(tools.TypeObject, "向量搜索参数")

	querySchema := tools.NewSimpleSchema(tools.TypeString, "搜索查询文本")
	schema.AddProperty("query", querySchema, true)

	topKSchema := tools.NewSimpleSchema(tools.TypeInteger, "返回结果数量")
	schema.AddProperty("top_k", topKSchema, false)

	thresholdSchema := tools.NewSimpleSchema(tools.TypeNumber, "相似度阈值（0-1），低于此值的结果将被过滤")
	schema.AddProperty("threshold", thresholdSchema, false)

	filtersSchema := tools.NewSimpleSchema(tools.TypeObject, "元数据过滤条件")
	schema.AddProperty("filters", filtersSchema, false)

	return &VectorSearchTool{
		options: options,
		id:      "vector_search",
		desc:    "基于语义相似度搜索文档",
		schema:  schema,
		vectors: make(map[string][]float32),
		chunks:  make(map[string]*common.DocumentChunk),
		meta:    make(map[string]map[string]interface{}),
	}
}

// GetID 返回工具ID
func (t *VectorSearchTool) GetID() string {
	return t.id
}

// GetDescription 返回工具描述
func (t *VectorSearchTool) GetDescription() string {
	return t.desc
}

// GetInputSchema 返回输入架构
func (t *VectorSearchTool) GetInputSchema() tools.Schema {
	return t.schema
}

// Execute 执行工具
func (t *VectorSearchTool) Execute(params map[string]interface{}, options *tools.ExecuteOptions) (interface{}, error) {
	// 获取查询文本
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

	var filters SearchFilters
	if filtersVal, ok := params["filters"].(map[string]interface{}); ok {
		filters = filtersVal
	}

	// 执行向量搜索
	results, err := t.searchByText(query, topK, threshold, filters)
	if err != nil {
		return nil, fmt.Errorf("搜索失败: %w", err)
	}

	return results, nil
}

// AddItem 添加一个向量项
func (t *VectorSearchTool) AddItem(id string, chunk *common.DocumentChunk, vector []float32, metadata map[string]interface{}) error {
	if len(vector) != t.options.Dimension {
		return fmt.Errorf("向量维度不匹配，期望 %d，实际 %d", t.options.Dimension, len(vector))
	}

	t.vectors[id] = vector
	t.chunks[id] = chunk

	if metadata != nil {
		t.meta[id] = metadata
	} else {
		t.meta[id] = make(map[string]interface{})
	}

	return nil
}

// AddItems 批量添加向量项
func (t *VectorSearchTool) AddItems(chunks []*common.DocumentChunk, metadata []map[string]interface{}) error {
	if t.options.EmbeddingProvider == nil {
		return errors.New("未设置嵌入向量提供者")
	}

	// 提取文本内容
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = chunk.Content
	}

	// 获取嵌入向量
	embeddings, err := t.options.EmbeddingProvider.GetEmbeddings(texts)
	if err != nil {
		return fmt.Errorf("获取嵌入向量失败: %w", err)
	}

	// 添加到索引
	for i, chunk := range chunks {
		id := fmt.Sprintf("%s-%d", chunk.DocumentID, i)
		if chunk.DocumentID == "" {
			id = fmt.Sprintf("chunk-%d", i)
		}

		var meta map[string]interface{}
		if i < len(metadata) {
			meta = metadata[i]
		}

		if err := t.AddItem(id, chunk, embeddings[i], meta); err != nil {
			return err
		}
	}

	return nil
}

// searchByText 通过文本进行搜索
func (t *VectorSearchTool) searchByText(query string, topK int, threshold float32, filters SearchFilters) ([]*VectorSearchResult, error) {
	if t.options.EmbeddingProvider == nil {
		return nil, errors.New("未设置嵌入向量提供者")
	}

	// 获取查询文本的嵌入向量
	queryEmbedding, err := t.options.EmbeddingProvider.GetEmbedding(query)
	if err != nil {
		return nil, fmt.Errorf("获取查询嵌入向量失败: %w", err)
	}

	// 使用向量搜索
	return t.search(queryEmbedding, topK, threshold, filters)
}

// search 使用向量进行搜索
func (t *VectorSearchTool) search(queryVector []float32, topK int, threshold float32, filters SearchFilters) ([]*VectorSearchResult, error) {
	if len(t.vectors) == 0 {
		return []*VectorSearchResult{}, nil
	}

	results := make([]*VectorSearchResult, 0, len(t.vectors))

	// 计算所有向量与查询向量的相似度
	for id, vector := range t.vectors {
		// 检查过滤条件
		if !t.matchFilters(id, filters) {
			continue
		}

		// 计算相似度
		var score float32
		switch t.options.DistanceMetric {
		case "cosine":
			score = cosineSimilarity(queryVector, vector)
		case "euclidean":
			score = 1.0 / (1.0 + euclideanDistance(queryVector, vector))
		case "dot":
			score = dotProduct(queryVector, vector)
		default:
			score = cosineSimilarity(queryVector, vector)
		}

		// 应用阈值过滤
		if score < threshold {
			continue
		}

		results = append(results, &VectorSearchResult{
			Chunk:    t.chunks[id],
			Score:    score,
			Metadata: t.meta[id],
		})
	}

	// 按相似度排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// 限制返回数量
	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

// matchFilters 检查元数据是否匹配过滤条件
func (t *VectorSearchTool) matchFilters(id string, filters SearchFilters) bool {
	if filters == nil || len(filters) == 0 {
		return true
	}

	metadata, exists := t.meta[id]
	if !exists {
		return false
	}

	for key, value := range filters {
		metaValue, exists := metadata[key]
		if !exists {
			return false
		}

		// 简单的相等匹配，可以扩展为更复杂的匹配逻辑
		if metaValue != value {
			return false
		}
	}

	return true
}

// DeleteItem 删除一个向量项
func (t *VectorSearchTool) DeleteItem(id string) bool {
	_, exists := t.vectors[id]
	if !exists {
		return false
	}

	delete(t.vectors, id)
	delete(t.chunks, id)
	delete(t.meta, id)

	return true
}

// Clear 清空所有向量
func (t *VectorSearchTool) Clear() {
	t.vectors = make(map[string][]float32)
	t.chunks = make(map[string]*common.DocumentChunk)
	t.meta = make(map[string]map[string]interface{})
}

// GetCount 获取向量数量
func (t *VectorSearchTool) GetCount() int {
	return len(t.vectors)
}

// 向量距离计算函数

// cosineSimilarity 计算余弦相似度
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProd, aMag, bMag float32
	for i := 0; i < len(a); i++ {
		dotProd += a[i] * b[i]
		aMag += a[i] * a[i]
		bMag += b[i] * b[i]
	}

	aMag = float32(math.Sqrt(float64(aMag)))
	bMag = float32(math.Sqrt(float64(bMag)))

	if aMag == 0 || bMag == 0 {
		return 0
	}

	return dotProd / (aMag * bMag)
}

// euclideanDistance 计算欧几里得距离
func euclideanDistance(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var sum float32
	for i := 0; i < len(a); i++ {
		diff := a[i] - b[i]
		sum += diff * diff
	}

	return float32(math.Sqrt(float64(sum)))
}

// dotProduct 计算点积
func dotProduct(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var sum float32
	for i := 0; i < len(a); i++ {
		sum += a[i] * b[i]
	}

	return sum
}
