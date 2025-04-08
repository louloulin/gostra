package memory

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
)

// Embedding 表示文本的向量嵌入
type Embedding []float32

// VectorEntry 表示向量存储中的条目
type VectorEntry struct {
	ID        string                 `json:"id"`
	Content   string                 `json:"content"`
	Embedding Embedding              `json:"embedding"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ScoredVectorEntry 表示带有相似度分数的向量条目
type ScoredVectorEntry struct {
	Entry VectorEntry `json:"entry"`
	Score float32     `json:"score"`
}

// EmbeddingProvider 提供文本嵌入服务
type EmbeddingProvider interface {
	// GetEmbedding 获取文本的嵌入向量
	GetEmbedding(ctx context.Context, text string) (Embedding, error)

	// GetEmbeddings 批量获取文本的嵌入向量
	GetEmbeddings(ctx context.Context, texts []string) ([]Embedding, error)
}

// VectorStore 定义向量存储接口
type VectorStore interface {
	// Add 添加向量条目
	Add(ctx context.Context, entries []VectorEntry) error

	// Delete 删除向量条目
	Delete(ctx context.Context, ids []string) error

	// Search 搜索最相似的向量
	Search(ctx context.Context, query Embedding, limit int, filter map[string]interface{}) ([]ScoredVectorEntry, error)

	// SearchByText 通过文本搜索
	SearchByText(ctx context.Context, queryText string, limit int, filter map[string]interface{}) ([]ScoredVectorEntry, error)

	// Get 获取特定ID的向量条目
	Get(ctx context.Context, ids []string) ([]VectorEntry, error)

	// Update 更新向量条目
	Update(ctx context.Context, entries []VectorEntry) error

	// Clear 清空所有向量条目
	Clear(ctx context.Context) error
}

// InMemoryVectorStore 实现了基于内存的向量存储
type InMemoryVectorStore struct {
	entries       map[string]VectorEntry
	embeddingProv EmbeddingProvider
	mu            sync.RWMutex
}

// NewInMemoryVectorStore 创建一个新的内存向量存储
func NewInMemoryVectorStore(embeddingProvider EmbeddingProvider) *InMemoryVectorStore {
	return &InMemoryVectorStore{
		entries:       make(map[string]VectorEntry),
		embeddingProv: embeddingProvider,
	}
}

// Add 添加向量条目
func (s *InMemoryVectorStore) Add(ctx context.Context, entries []VectorEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, entry := range entries {
		if entry.ID == "" {
			return errors.New("entry ID cannot be empty")
		}
		if len(entry.Embedding) == 0 && entry.Content != "" && s.embeddingProv != nil {
			// 如果没有提供嵌入向量但提供了文本内容，尝试生成嵌入向量
			embedding, err := s.embeddingProv.GetEmbedding(ctx, entry.Content)
			if err != nil {
				return fmt.Errorf("failed to get embedding for content: %w", err)
			}
			entry.Embedding = embedding
		}
		if len(entry.Embedding) == 0 {
			return errors.New("entry embedding cannot be empty")
		}
		s.entries[entry.ID] = entry
	}
	return nil
}

// Delete 删除向量条目
func (s *InMemoryVectorStore) Delete(ctx context.Context, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, id := range ids {
		delete(s.entries, id)
	}
	return nil
}

// Search 搜索最相似的向量
func (s *InMemoryVectorStore) Search(ctx context.Context, query Embedding, limit int, filter map[string]interface{}) ([]ScoredVectorEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(query) == 0 {
		return nil, errors.New("query embedding cannot be empty")
	}

	// 计算所有向量与查询向量的相似度分数
	scores := make([]ScoredVectorEntry, 0, len(s.entries))
	for _, entry := range s.entries {
		// 如果有过滤条件，检查是否匹配
		if !matchFilter(entry, filter) {
			continue
		}

		score := cosineSimilarity(query, entry.Embedding)
		scores = append(scores, ScoredVectorEntry{
			Entry: entry,
			Score: score,
		})
	}

	// 按分数降序排序
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})

	// 限制结果数量
	if limit > 0 && limit < len(scores) {
		scores = scores[:limit]
	}

	return scores, nil
}

// SearchByText 通过文本搜索
func (s *InMemoryVectorStore) SearchByText(ctx context.Context, queryText string, limit int, filter map[string]interface{}) ([]ScoredVectorEntry, error) {
	if s.embeddingProv == nil {
		return nil, errors.New("embedding provider is required for text search")
	}

	// 获取查询文本的嵌入向量
	queryEmbedding, err := s.embeddingProv.GetEmbedding(ctx, queryText)
	if err != nil {
		return nil, fmt.Errorf("failed to get embedding for query text: %w", err)
	}

	// 使用嵌入向量进行搜索
	return s.Search(ctx, queryEmbedding, limit, filter)
}

// Get 获取特定ID的向量条目
func (s *InMemoryVectorStore) Get(ctx context.Context, ids []string) ([]VectorEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := make([]VectorEntry, 0, len(ids))
	for _, id := range ids {
		if entry, ok := s.entries[id]; ok {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

// Update 更新向量条目
func (s *InMemoryVectorStore) Update(ctx context.Context, entries []VectorEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, entry := range entries {
		if entry.ID == "" {
			return errors.New("entry ID cannot be empty")
		}
		if _, ok := s.entries[entry.ID]; !ok {
			return fmt.Errorf("entry with ID '%s' not found", entry.ID)
		}

		if len(entry.Embedding) == 0 && entry.Content != "" && s.embeddingProv != nil {
			// 如果没有提供嵌入向量但提供了文本内容，尝试生成嵌入向量
			embedding, err := s.embeddingProv.GetEmbedding(ctx, entry.Content)
			if err != nil {
				return fmt.Errorf("failed to get embedding for content: %w", err)
			}
			entry.Embedding = embedding
		}

		if len(entry.Embedding) == 0 {
			// 如果没有提供新的嵌入向量，保留原有的
			oldEntry := s.entries[entry.ID]
			entry.Embedding = oldEntry.Embedding
		}

		s.entries[entry.ID] = entry
	}
	return nil
}

// Clear 清空所有向量条目
func (s *InMemoryVectorStore) Clear(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries = make(map[string]VectorEntry)
	return nil
}

// 计算余弦相似度
func cosineSimilarity(a, b Embedding) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct float32
	var normA float32
	var normB float32

	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}

// 检查条目是否匹配过滤条件
func matchFilter(entry VectorEntry, filter map[string]interface{}) bool {
	if filter == nil || len(filter) == 0 {
		return true
	}

	for k, v := range filter {
		if entry.Metadata == nil {
			return false
		}

		entryVal, ok := entry.Metadata[k]
		if !ok || entryVal != v {
			return false
		}
	}

	return true
}

// VectorMemoryProvider 结合了基本内存存储和向量存储
type VectorMemoryProvider struct {
	MemoryProvider
	vectorStore   VectorStore
	embedProvider EmbeddingProvider
}

// NewVectorMemoryProvider 创建一个新的向量内存提供者
func NewVectorMemoryProvider(baseProvider MemoryProvider, vectorStore VectorStore, embedProvider EmbeddingProvider) *VectorMemoryProvider {
	return &VectorMemoryProvider{
		MemoryProvider: baseProvider,
		vectorStore:    vectorStore,
		embedProvider:  embedProvider,
	}
}

// AddMessage 添加消息并将其存储到向量存储中
func (p *VectorMemoryProvider) AddMessage(ctx context.Context, threadID string, role string, content string, metadata map[string]interface{}) (*Message, error) {
	// 使用基本提供者添加消息
	msg, err := p.MemoryProvider.AddMessage(ctx, threadID, role, content, metadata)
	if err != nil {
		return nil, err
	}

	// 如果没有嵌入提供者，跳过向量存储
	if p.embedProvider == nil || p.vectorStore == nil {
		return msg, nil
	}

	// 为消息内容获取嵌入向量
	embedding, err := p.embedProvider.GetEmbedding(ctx, content)
	if err != nil {
		// 记录错误但不阻止消息添加
		fmt.Printf("Failed to get embedding for message: %v\n", err)
		return msg, nil
	}

	// 构建向量条目
	entry := VectorEntry{
		ID:        msg.ID,
		Content:   content,
		Embedding: embedding,
		Metadata: map[string]interface{}{
			"thread_id": threadID,
			"role":      role,
		},
	}

	// 如果有其他元数据，添加到向量条目中
	if metadata != nil {
		for k, v := range metadata {
			entry.Metadata[k] = v
		}
	}

	// 添加到向量存储
	if err := p.vectorStore.Add(ctx, []VectorEntry{entry}); err != nil {
		// 记录错误但不阻止消息添加
		fmt.Printf("Failed to add message to vector store: %v\n", err)
	}

	return msg, nil
}

// DeleteMessages 从基本存储和向量存储中删除消息
func (p *VectorMemoryProvider) DeleteMessages(ctx context.Context, threadID string, messageIDs []string) error {
	// 使用基本提供者删除消息
	err := p.MemoryProvider.DeleteMessages(ctx, threadID, messageIDs)
	if err != nil {
		return err
	}

	// 如果没有向量存储，跳过
	if p.vectorStore == nil {
		return nil
	}

	// 从向量存储中删除
	if err := p.vectorStore.Delete(ctx, messageIDs); err != nil {
		// 记录错误但不阻止消息删除
		fmt.Printf("Failed to delete messages from vector store: %v\n", err)
	}

	return nil
}

// DeleteThread 删除线程及其所有消息
func (p *VectorMemoryProvider) DeleteThread(ctx context.Context, threadID string) error {
	// 获取线程中的所有消息
	messages, err := p.MemoryProvider.GetMessages(ctx, threadID, 1000, 0)
	if err != nil {
		return err
	}

	// 收集消息ID
	messageIDs := make([]string, len(messages))
	for i, msg := range messages {
		messageIDs[i] = msg.ID
	}

	// 使用基本提供者删除线程
	err = p.MemoryProvider.DeleteThread(ctx, threadID)
	if err != nil {
		return err
	}

	// 如果没有向量存储或没有消息，跳过
	if p.vectorStore == nil || len(messageIDs) == 0 {
		return nil
	}

	// 从向量存储中删除
	if err := p.vectorStore.Delete(ctx, messageIDs); err != nil {
		// 记录错误但不阻止线程删除
		fmt.Printf("Failed to delete thread messages from vector store: %v\n", err)
	}

	return nil
}

// SearchMemory 在向量存储中搜索相关消息
func (p *VectorMemoryProvider) SearchMemory(ctx context.Context, query string, limit int, threadID string) ([]*Message, error) {
	if p.vectorStore == nil || p.embedProvider == nil {
		return nil, errors.New("vector store and embedding provider are required for memory search")
	}

	// 构建过滤条件
	filter := map[string]interface{}{}
	if threadID != "" {
		filter["thread_id"] = threadID
	}

	// 搜索相关向量
	results, err := p.vectorStore.SearchByText(ctx, query, limit, filter)
	if err != nil {
		return nil, err
	}

	// 将搜索结果映射回消息
	messages := make([]*Message, 0, len(results))
	for _, result := range results {
		// 获取消息ID
		id := result.Entry.ID

		// 构建消息
		msg := &Message{
			ID:       id,
			Content:  result.Entry.Content,
			Metadata: result.Entry.Metadata,
		}

		// 获取角色
		if role, ok := result.Entry.Metadata["role"].(string); ok {
			msg.Role = role
		}

		messages = append(messages, msg)
	}

	return messages, nil
}

// GetSimilarMessages 获取与给定消息相似的其他消息
func (p *VectorMemoryProvider) GetSimilarMessages(ctx context.Context, messageContent string, limit int, threshold float32, threadID string) ([]*Message, error) {
	if p.vectorStore == nil || p.embedProvider == nil {
		return nil, errors.New("vector store and embedding provider are required for similarity search")
	}

	// 获取消息的嵌入向量
	embedding, err := p.embedProvider.GetEmbedding(ctx, messageContent)
	if err != nil {
		return nil, err
	}

	// 构建过滤条件
	filter := map[string]interface{}{}
	if threadID != "" {
		filter["thread_id"] = threadID
	}

	// 搜索相似向量
	results, err := p.vectorStore.Search(ctx, embedding, limit, filter)
	if err != nil {
		return nil, err
	}

	// 过滤掉低于阈值的结果
	filteredResults := make([]ScoredVectorEntry, 0)
	for _, result := range results {
		if result.Score >= threshold {
			filteredResults = append(filteredResults, result)
		}
	}

	// 将搜索结果映射回消息
	messages := make([]*Message, 0, len(filteredResults))
	for _, result := range filteredResults {
		// 获取消息ID
		id := result.Entry.ID

		// 构建消息
		msg := &Message{
			ID:       id,
			Content:  result.Entry.Content,
			Metadata: result.Entry.Metadata,
		}

		// 获取角色
		if role, ok := result.Entry.Metadata["role"].(string); ok {
			msg.Role = role
		}

		messages = append(messages, msg)
	}

	return messages, nil
}
