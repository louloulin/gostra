package common

// DocumentChunk 文档块
type DocumentChunk struct {
	// Content 是文档块的内容
	Content string `json:"content"`

	// DocumentID 是原始文档的ID
	DocumentID string `json:"document_id,omitempty"`

	// ChunkIndex 是分块的索引
	ChunkIndex int `json:"chunk_index"`

	// Metadata 是文档块的元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// SearchResult 表示搜索的结果
type SearchResult struct {
	// Content 是匹配的文档内容
	Content string `json:"content"`

	// DocumentID 是文档ID
	DocumentID string `json:"document_id,omitempty"`

	// Score 是相似度分数
	Score float32 `json:"score"`

	// Metadata 是文档元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// ChunkIndex 是分块索引
	ChunkIndex int `json:"chunk_index,omitempty"`

	// Chunk 是原始文档块引用
	Chunk *DocumentChunk `json:"chunk,omitempty"`
}

// ChunkStrategy 分块策略
type ChunkStrategy string

const (
	// 固定大小分块
	ChunkStrategyFixed ChunkStrategy = "fixed"

	// 递归分块
	ChunkStrategyRecursive ChunkStrategy = "recursive"

	// 段落分块
	ChunkStrategyParagraph ChunkStrategy = "paragraph"

	// 句子分块
	ChunkStrategySentence ChunkStrategy = "sentence"
)
