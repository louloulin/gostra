package memory

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// TestAdvancedContextManager 测试高级上下文管理器
func TestAdvancedContextManager(t *testing.T) {
	// 创建嵌入提供者
	mockEmbedding := &MockEmbeddingProvider{}

	// 创建向量存储
	vectorStore := NewInMemoryVectorStore(mockEmbedding)

	// 创建上下文管理器
	opts := &AdvancedContextManagerOptions{
		VectorStore:       vectorStore,
		EmbeddingProvider: mockEmbedding,
		Strategy:          StrategyHybrid,
		RelevanceWeight:   0.6,
		RecencyWeight:     0.3,
		PriorityWeight:    0.1,
	}

	manager, err := NewAdvancedContextManager(opts)
	assert.NoError(t, err)
	assert.NotNil(t, manager)

	ctx := context.Background()

	// 测试添加项目
	testAddItems(t, ctx, manager)

	// 测试检索项目
	testRetrieveItems(t, ctx, manager)

	// 测试删除项目
	testDeleteItems(t, ctx, manager)

	// 测试过滤功能
	testFiltering(t, ctx, manager)

	// 测试不同的检索策略
	testRetrievalStrategies(t, ctx, manager)
}

// 测试添加项目
func testAddItems(t *testing.T, ctx context.Context, manager *AdvancedContextManager) {
	// 添加几个测试项目
	for i := 0; i < 5; i++ {
		item := &ContextItem{
			Content:  "Test content " + uuid.New().String(),
			Type:     "test",
			Priority: i,
			Source:   "user",
		}
		err := manager.AddItem(ctx, item)
		assert.NoError(t, err)
	}

	// 验证项目是否被正确添加
	items, err := manager.GetItems(ctx, nil, 10)
	assert.NoError(t, err)
	assert.Equal(t, 5, len(items))
}

// 测试检索项目
func testRetrieveItems(t *testing.T, ctx context.Context, manager *AdvancedContextManager) {
	// 先清空现有项目
	err := manager.Clear(ctx)
	assert.NoError(t, err)

	// 添加有不同优先级的项目
	items := []*ContextItem{
		{
			Content:  "This is a test about artificial intelligence",
			Type:     "document",
			Priority: 1,
			Source:   "user",
		},
		{
			Content:  "Machine learning is a subset of AI",
			Type:     "document",
			Priority: 2,
			Source:   "system",
		},
		{
			Content:  "Neural networks are used in deep learning",
			Type:     "document",
			Priority: 3,
			Source:   "user",
		},
	}

	for _, item := range items {
		err := manager.AddItem(ctx, item)
		assert.NoError(t, err)
	}

	// 使用混合检索
	retrieval, err := manager.RetrieveContext(ctx, "What is artificial intelligence?", 1000, nil)
	assert.NoError(t, err)
	assert.NotNil(t, retrieval)
	assert.GreaterOrEqual(t, len(retrieval.Items), 1)
}

// 测试删除项目
func testDeleteItems(t *testing.T, ctx context.Context, manager *AdvancedContextManager) {
	// 先清空现有项目
	err := manager.Clear(ctx)
	assert.NoError(t, err)

	// 添加项目
	item := &ContextItem{
		Content:  "Content to be deleted",
		Type:     "test",
		Priority: 1,
		Source:   "user",
	}
	err = manager.AddItem(ctx, item)
	assert.NoError(t, err)

	// 验证项目已添加
	items, err := manager.GetItems(ctx, nil, 10)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(items))

	// 删除项目
	err = manager.DeleteItem(ctx, items[0].ID)
	assert.NoError(t, err)

	// 验证项目已删除
	items, err = manager.GetItems(ctx, nil, 10)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(items))
}

// 测试过滤功能
func testFiltering(t *testing.T, ctx context.Context, manager *AdvancedContextManager) {
	// 先清空现有项目
	err := manager.Clear(ctx)
	assert.NoError(t, err)

	// 添加不同类型的项目
	items := []*ContextItem{
		{
			Content:  "User content",
			Type:     "message",
			Priority: 1,
			Source:   "user",
		},
		{
			Content:  "System content",
			Type:     "message",
			Priority: 2,
			Source:   "system",
		},
		{
			Content:  "Document content",
			Type:     "document",
			Priority: 3,
			Source:   "user",
		},
	}

	for _, item := range items {
		err := manager.AddItem(ctx, item)
		assert.NoError(t, err)
	}

	// 按类型过滤
	filter := map[string]interface{}{
		"type": "message",
	}
	filtered, err := manager.GetItems(ctx, filter, 10)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(filtered))

	// 按来源过滤
	filter = map[string]interface{}{
		"source": "user",
	}
	filtered, err = manager.GetItems(ctx, filter, 10)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(filtered))

	// 复合过滤
	filter = map[string]interface{}{
		"type":   "message",
		"source": "user",
	}
	filtered, err = manager.GetItems(ctx, filter, 10)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(filtered))
}

// 测试不同的检索策略
func testRetrievalStrategies(t *testing.T, ctx context.Context, manager *AdvancedContextManager) {
	// 先清空现有项目
	err := manager.Clear(ctx)
	assert.NoError(t, err)

	// 添加不同优先级和时间的项目
	items := []*ContextItem{
		{
			Content:   "Old content with low priority",
			Type:      "message",
			Priority:  1,
			Source:    "user",
			CreatedAt: time.Now().Add(-24 * time.Hour),
		},
		{
			Content:   "New content with low priority",
			Type:      "message",
			Priority:  1,
			Source:    "user",
			CreatedAt: time.Now(),
		},
		{
			Content:   "Old content with high priority",
			Type:      "message",
			Priority:  5,
			Source:    "system",
			CreatedAt: time.Now().Add(-24 * time.Hour),
		},
	}

	for _, item := range items {
		err := manager.AddItem(ctx, item)
		assert.NoError(t, err)
	}

	// 测试按优先级检索
	manager.strategy = StrategyPriority
	retrieval, err := manager.RetrieveContext(ctx, "", 1000, nil)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(retrieval.Items))
	assert.Equal(t, 5, retrieval.Items[0].Priority) // 高优先级应该排在前面

	// 测试按时间检索
	manager.strategy = StrategyRecency
	retrieval, err = manager.RetrieveContext(ctx, "", 1000, nil)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(retrieval.Items))
	assert.True(t, retrieval.Items[0].CreatedAt.After(retrieval.Items[1].CreatedAt)) // 新项目应该排在前面
}

// MockEmbeddingProvider 模拟嵌入向量提供者
type MockEmbeddingProvider struct{}

// GetEmbedding 获取单个文本的嵌入向量
func (p *MockEmbeddingProvider) GetEmbedding(ctx context.Context, text string) (Embedding, error) {
	// 返回固定维度的随机向量
	embedding := make(Embedding, 10)
	for i := range embedding {
		embedding[i] = float32(i) / 10.0
	}
	return embedding, nil
}

// GetEmbeddings 批量获取文本的嵌入向量
func (p *MockEmbeddingProvider) GetEmbeddings(ctx context.Context, texts []string) ([]Embedding, error) {
	embeddings := make([]Embedding, len(texts))
	for i := range texts {
		emb, _ := p.GetEmbedding(ctx, texts[i])
		embeddings[i] = emb
	}
	return embeddings, nil
}
