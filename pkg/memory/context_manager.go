package memory

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	stdctx "context" // 重命名标准context包，避免冲突

	"github.com/asynkron/protoactor-go/actor"
	"github.com/google/uuid"
)

// ContextItem 表示上下文中的一个项目
type ContextItem struct {
	ID        string                 `json:"id"`
	Content   string                 `json:"content"`
	Type      string                 `json:"type"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	Priority  int                    `json:"priority"`
	Source    string                 `json:"source,omitempty"` // 例如：用户、工具、LLM
	TTL       time.Duration          `json:"ttl,omitempty"`    // 生存时间
}

// ContextRetrieval 表示上下文检索结果
type ContextRetrieval struct {
	Items      []*ContextItem `json:"items"`
	TokenCount int            `json:"token_count"` // 粗略估计的token数量
}

// ContextManager 定义了上下文管理器的接口
type ContextManager interface {
	// AddItem 添加一个项目到上下文
	AddItem(ctx context.Context, item *ContextItem) error

	// GetItems 获取上下文中的所有项目
	GetItems(ctx context.Context, filter map[string]interface{}, limit int) ([]*ContextItem, error)

	// DeleteItem 删除上下文中的一个项目
	DeleteItem(ctx context.Context, itemID string) error

	// RetrieveContext 检索相关上下文
	RetrieveContext(ctx context.Context, query string, maxTokens int, filter map[string]interface{}) (*ContextRetrieval, error)

	// Clear 清空上下文
	Clear(ctx context.Context) error

	// SetVectorStore 设置向量存储
	SetVectorStore(vectorStore VectorStore)

	// SetEmbeddingProvider 设置嵌入向量提供者
	SetEmbeddingProvider(provider EmbeddingProvider)
}

// 上下文检索策略
type ContextRetrievalStrategy string

const (
	// StrategyRecency 基于时间的检索策略
	StrategyRecency ContextRetrievalStrategy = "recency"
	// StrategyRelevance 基于相关性的检索策略
	StrategyRelevance ContextRetrievalStrategy = "relevance"
	// StrategyHybrid 混合检索策略
	StrategyHybrid ContextRetrievalStrategy = "hybrid"
	// StrategyPriority 基于优先级的检索策略
	StrategyPriority ContextRetrievalStrategy = "priority"
)

// AdvancedContextManagerOptions 高级上下文管理器的选项
type AdvancedContextManagerOptions struct {
	// 向量存储
	VectorStore VectorStore
	// 嵌入向量提供者
	EmbeddingProvider EmbeddingProvider
	// 检索策略
	Strategy ContextRetrievalStrategy
	// 混合检索时的相关性权重 (0-1)
	RelevanceWeight float32
	// 混合检索时的时间权重 (0-1)
	RecencyWeight float32
	// 混合检索时的优先级权重 (0-1)
	PriorityWeight float32
	// Actor系统根上下文
	ActorContext *actor.RootContext
	// 上下文项目过期检查间隔
	ExpirationCheckInterval time.Duration
}

// AdvancedContextManager 实现了高级上下文管理
type AdvancedContextManager struct {
	vectorStore             VectorStore
	embeddingProvider       EmbeddingProvider
	items                   map[string]*ContextItem
	strategy                ContextRetrievalStrategy
	relevanceWeight         float32
	recencyWeight           float32
	priorityWeight          float32
	mu                      sync.RWMutex
	actorCtx                *actor.RootContext
	actorPID                *actor.PID
	expirationCheckInterval time.Duration
}

// NewAdvancedContextManager 创建一个新的高级上下文管理器
func NewAdvancedContextManager(opts *AdvancedContextManagerOptions) (*AdvancedContextManager, error) {
	if opts == nil {
		return nil, errors.New("options cannot be nil")
	}

	// 设置默认值
	if opts.Strategy == "" {
		opts.Strategy = StrategyHybrid
	}
	if opts.Strategy == StrategyHybrid {
		if opts.RelevanceWeight == 0 && opts.RecencyWeight == 0 && opts.PriorityWeight == 0 {
			// 设置默认权重
			opts.RelevanceWeight = 0.6
			opts.RecencyWeight = 0.3
			opts.PriorityWeight = 0.1
		}
	}
	if opts.ExpirationCheckInterval == 0 {
		opts.ExpirationCheckInterval = 5 * time.Minute
	}

	manager := &AdvancedContextManager{
		vectorStore:             opts.VectorStore,
		embeddingProvider:       opts.EmbeddingProvider,
		items:                   make(map[string]*ContextItem),
		strategy:                opts.Strategy,
		relevanceWeight:         opts.RelevanceWeight,
		recencyWeight:           opts.RecencyWeight,
		priorityWeight:          opts.PriorityWeight,
		actorCtx:                opts.ActorContext,
		expirationCheckInterval: opts.ExpirationCheckInterval,
	}

	// 如果提供了Actor上下文，启动一个Actor来管理上下文
	if opts.ActorContext != nil {
		props := actor.PropsFromProducer(func() actor.Actor {
			return &contextManagerActor{manager: manager}
		})
		pid, err := opts.ActorContext.SpawnNamed(props, "context_manager_"+uuid.New().String())
		if err != nil {
			return nil, fmt.Errorf("failed to spawn context manager actor: %w", err)
		}
		manager.actorPID = pid
	}

	return manager, nil
}

// AddItem 添加一个项目到上下文
func (m *AdvancedContextManager) AddItem(ctx context.Context, item *ContextItem) error {
	if item == nil {
		return errors.New("item cannot be nil")
	}

	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now()
	}

	// 如果提供了内容但没有嵌入向量，且有嵌入向量提供者，则获取嵌入向量
	if item.Content != "" && m.embeddingProvider != nil && m.vectorStore != nil {
		embedding, err := m.embeddingProvider.GetEmbedding(ctx, item.Content)
		if err != nil {
			return fmt.Errorf("failed to get embedding: %w", err)
		}

		// 添加到向量存储
		err = m.vectorStore.Add(ctx, []VectorEntry{
			{
				ID:        item.ID,
				Content:   item.Content,
				Embedding: embedding,
				Metadata: map[string]interface{}{
					"type":     item.Type,
					"source":   item.Source,
					"priority": item.Priority,
					"created":  item.CreatedAt.UnixNano(),
				},
			},
		})
		if err != nil {
			return fmt.Errorf("failed to add to vector store: %w", err)
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[item.ID] = item

	return nil
}

// GetItems 获取上下文中的所有项目
func (m *AdvancedContextManager) GetItems(ctx context.Context, filter map[string]interface{}, limit int) ([]*ContextItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 检查过期项目
	m.checkExpiredItems()

	result := make([]*ContextItem, 0, len(m.items))
	for _, item := range m.items {
		if m.matchesFilter(item, filter) {
			result = append(result, item)
		}
	}

	// 按创建时间排序，最新的在前
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	// 限制结果数量
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result, nil
}

// DeleteItem 删除上下文中的一个项目
func (m *AdvancedContextManager) DeleteItem(ctx context.Context, itemID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.items[itemID]; !exists {
		return fmt.Errorf("item with ID %s not found", itemID)
	}

	delete(m.items, itemID)

	// 如果有向量存储，也从向量存储中删除
	if m.vectorStore != nil {
		if err := m.vectorStore.Delete(ctx, []string{itemID}); err != nil {
			return fmt.Errorf("failed to delete from vector store: %w", err)
		}
	}

	return nil
}

// RetrieveContext 检索相关上下文
func (m *AdvancedContextManager) RetrieveContext(ctx context.Context, query string, maxTokens int, filter map[string]interface{}) (*ContextRetrieval, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 检查过期项目
	m.checkExpiredItems()

	var items []*ContextItem
	var err error

	switch m.strategy {
	case StrategyRelevance:
		items, err = m.retrieveByRelevance(ctx, query, maxTokens, filter)
	case StrategyRecency:
		items, err = m.retrieveByRecency(ctx, maxTokens, filter)
	case StrategyPriority:
		items, err = m.retrieveByPriority(ctx, maxTokens, filter)
	case StrategyHybrid:
		items, err = m.retrieveHybrid(ctx, query, maxTokens, filter)
	default:
		// 默认使用混合策略
		items, err = m.retrieveHybrid(ctx, query, maxTokens, filter)
	}

	if err != nil {
		return nil, err
	}

	// 计算粗略的token数量
	tokenCount := 0
	for _, item := range items {
		// 简单估计：每个单词约1.5个token
		tokenCount += int(float64(len(item.Content)) * 0.15)
	}

	return &ContextRetrieval{
		Items:      items,
		TokenCount: tokenCount,
	}, nil
}

// Clear 清空上下文
func (m *AdvancedContextManager) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.items = make(map[string]*ContextItem)

	// 如果有向量存储，也清空向量存储
	if m.vectorStore != nil {
		if err := m.vectorStore.Clear(ctx); err != nil {
			return fmt.Errorf("failed to clear vector store: %w", err)
		}
	}

	return nil
}

// SetVectorStore 设置向量存储
func (m *AdvancedContextManager) SetVectorStore(vectorStore VectorStore) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.vectorStore = vectorStore
}

// SetEmbeddingProvider 设置嵌入向量提供者
func (m *AdvancedContextManager) SetEmbeddingProvider(provider EmbeddingProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.embeddingProvider = provider
}

// retrieveByRelevance 通过相关性检索
func (m *AdvancedContextManager) retrieveByRelevance(ctx context.Context, query string, maxTokens int, filter map[string]interface{}) ([]*ContextItem, error) {
	if m.vectorStore == nil || m.embeddingProvider == nil {
		return nil, errors.New("vector store and embedding provider are required for relevance-based retrieval")
	}

	// 从向量存储中检索
	results, err := m.vectorStore.SearchByText(ctx, query, 50, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to search vector store: %w", err)
	}

	// 转换为ContextItem
	items := make([]*ContextItem, 0, len(results))
	for _, result := range results {
		id := result.Entry.ID
		if item, exists := m.items[id]; exists {
			items = append(items, item)
		}
	}

	// 计算并限制token数量
	totalTokens := 0
	resultItems := make([]*ContextItem, 0, len(items))
	for _, item := range items {
		// 简单估计token数量：平均每个单词约1.5个token
		tokens := int(float64(len(item.Content)) * 0.15)
		if totalTokens+tokens > maxTokens {
			break
		}
		resultItems = append(resultItems, item)
		totalTokens += tokens
	}

	return resultItems, nil
}

// retrieveByRecency 通过时间检索
func (m *AdvancedContextManager) retrieveByRecency(ctx context.Context, maxTokens int, filter map[string]interface{}) ([]*ContextItem, error) {
	// 获取所有符合过滤条件的项目
	allItems := make([]*ContextItem, 0, len(m.items))
	for _, item := range m.items {
		if m.matchesFilter(item, filter) {
			allItems = append(allItems, item)
		}
	}

	// 按时间排序
	sort.Slice(allItems, func(i, j int) bool {
		return allItems[i].CreatedAt.After(allItems[j].CreatedAt)
	})

	// 计算并限制token数量
	totalTokens := 0
	resultItems := make([]*ContextItem, 0, len(allItems))
	for _, item := range allItems {
		tokens := int(float64(len(item.Content)) * 0.15)
		if totalTokens+tokens > maxTokens {
			break
		}
		resultItems = append(resultItems, item)
		totalTokens += tokens
	}

	return resultItems, nil
}

// retrieveByPriority 通过优先级检索
func (m *AdvancedContextManager) retrieveByPriority(ctx context.Context, maxTokens int, filter map[string]interface{}) ([]*ContextItem, error) {
	// 获取所有符合过滤条件的项目
	allItems := make([]*ContextItem, 0, len(m.items))
	for _, item := range m.items {
		if m.matchesFilter(item, filter) {
			allItems = append(allItems, item)
		}
	}

	// 按优先级排序，高优先级在前
	sort.Slice(allItems, func(i, j int) bool {
		return allItems[i].Priority > allItems[j].Priority
	})

	// 计算并限制token数量
	totalTokens := 0
	resultItems := make([]*ContextItem, 0, len(allItems))
	for _, item := range allItems {
		tokens := int(float64(len(item.Content)) * 0.15)
		if totalTokens+tokens > maxTokens {
			break
		}
		resultItems = append(resultItems, item)
		totalTokens += tokens
	}

	return resultItems, nil
}

// retrieveHybrid 混合检索策略
func (m *AdvancedContextManager) retrieveHybrid(ctx context.Context, query string, maxTokens int, filter map[string]interface{}) ([]*ContextItem, error) {
	if m.vectorStore == nil || m.embeddingProvider == nil {
		// 如果没有向量存储或嵌入提供者，退化为按时间检索
		return m.retrieveByRecency(ctx, maxTokens, filter)
	}

	// 获取相关性检索结果
	relevanceResults, err := m.vectorStore.SearchByText(ctx, query, 100, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to search vector store: %w", err)
	}

	// 获取所有符合过滤条件的项目
	allItems := make([]*ContextItem, 0, len(m.items))
	for _, item := range m.items {
		if m.matchesFilter(item, filter) {
			allItems = append(allItems, item)
		}
	}

	// 创建ID到相关性分数的映射
	relevanceScores := make(map[string]float32)
	for _, result := range relevanceResults {
		relevanceScores[result.Entry.ID] = result.Score
	}

	// 计算最新时间和最旧时间
	var newest, oldest time.Time
	if len(allItems) > 0 {
		newest = allItems[0].CreatedAt
		oldest = allItems[0].CreatedAt
		for _, item := range allItems {
			if item.CreatedAt.After(newest) {
				newest = item.CreatedAt
			}
			if item.CreatedAt.Before(oldest) {
				oldest = item.CreatedAt
			}
		}
	}

	// 计算时间范围
	var timeRange float64 = 1
	if len(allItems) > 1 {
		timeRange = float64(newest.Sub(oldest))
		if timeRange == 0 {
			timeRange = 1
		}
	}

	// 计算最高和最低优先级
	highestPriority := -9999
	lowestPriority := 9999
	for _, item := range allItems {
		if item.Priority > highestPriority {
			highestPriority = item.Priority
		}
		if item.Priority < lowestPriority {
			lowestPriority = item.Priority
		}
	}

	priorityRange := float64(highestPriority - lowestPriority)
	if priorityRange == 0 {
		priorityRange = 1
	}

	// 计算混合得分
	type scoredItem struct {
		item  *ContextItem
		score float64
	}

	scoredItems := make([]scoredItem, 0, len(allItems))
	for _, item := range allItems {
		var relevanceScore, recencyScore, priorityScore, finalScore float64

		// 相关性得分（如果有）
		if score, ok := relevanceScores[item.ID]; ok {
			relevanceScore = float64(score)
		}

		// 时间得分
		if timeRange > 0 {
			recencyScore = float64(newest.Sub(item.CreatedAt)) / timeRange
			recencyScore = 1.0 - recencyScore // 转换为0-1，越新越高
		}

		// 优先级得分
		if priorityRange > 0 {
			priorityScore = float64(item.Priority-lowestPriority) / priorityRange
		}

		// 计算最终得分
		finalScore = float64(m.relevanceWeight)*relevanceScore +
			float64(m.recencyWeight)*recencyScore +
			float64(m.priorityWeight)*priorityScore

		scoredItems = append(scoredItems, scoredItem{
			item:  item,
			score: finalScore,
		})
	}

	// 按最终得分排序
	sort.Slice(scoredItems, func(i, j int) bool {
		return scoredItems[i].score > scoredItems[j].score
	})

	// 计算并限制token数量
	totalTokens := 0
	resultItems := make([]*ContextItem, 0, len(scoredItems))
	for _, scoredItem := range scoredItems {
		tokens := int(float64(len(scoredItem.item.Content)) * 0.15)
		if totalTokens+tokens > maxTokens {
			break
		}
		resultItems = append(resultItems, scoredItem.item)
		totalTokens += tokens
	}

	return resultItems, nil
}

// matchesFilter 检查项目是否匹配过滤条件
func (m *AdvancedContextManager) matchesFilter(item *ContextItem, filter map[string]interface{}) bool {
	if filter == nil || len(filter) == 0 {
		return true
	}

	for key, value := range filter {
		switch key {
		case "type":
			if item.Type != value.(string) {
				return false
			}
		case "source":
			if item.Source != value.(string) {
				return false
			}
		case "priority":
			if item.Priority != value.(int) {
				return false
			}
		default:
			// 检查元数据
			if item.Metadata != nil {
				if metaValue, ok := item.Metadata[key]; ok {
					if metaValue != value {
						return false
					}
				} else {
					return false
				}
			} else {
				return false
			}
		}
	}

	return true
}

// checkExpiredItems 检查并删除过期的项目
func (m *AdvancedContextManager) checkExpiredItems() {
	now := time.Now()
	for id, item := range m.items {
		if item.TTL > 0 && item.CreatedAt.Add(item.TTL).Before(now) {
			delete(m.items, id)
			// 异步从向量存储中删除
			if m.vectorStore != nil {
				go func(itemID string) {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					m.vectorStore.Delete(ctx, []string{itemID})
				}(id)
			}
		}
	}
}

// ActorBasedContextManager 通过Actor实现上下文管理
type ActorBasedContextManager struct {
	actorCtx *actor.RootContext
	pid      *actor.PID
}

// contextManagerActor 实现上下文管理Actor
type contextManagerActor struct {
	manager    *AdvancedContextManager
	expiration *time.Ticker
}

// Receive 处理Actor消息
func (a *contextManagerActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *actor.Started:
		// 启动定期检查过期项目的ticker
		a.expiration = time.NewTicker(a.manager.expirationCheckInterval)
		go func() {
			for range a.expiration.C {
				a.manager.mu.Lock()
				a.manager.checkExpiredItems()
				a.manager.mu.Unlock()
			}
		}()

	case *actor.Stopping:
		// 停止ticker
		if a.expiration != nil {
			a.expiration.Stop()
		}

	case *AddContextItemMessage:
		ctx, cancel := stdctx.WithTimeout(stdctx.Background(), 5*time.Second)
		defer cancel()
		err := a.manager.AddItem(ctx, msg.Item)
		context.Respond(&ContextManagerResponse{
			Success: err == nil,
			Error:   errorToString(err),
		})

	case *GetContextItemsMessage:
		ctx, cancel := stdctx.WithTimeout(stdctx.Background(), 5*time.Second)
		defer cancel()
		items, err := a.manager.GetItems(ctx, msg.Filter, msg.Limit)
		context.Respond(&ContextItemsResponse{
			Items:   items,
			Success: err == nil,
			Error:   errorToString(err),
		})

	case *DeleteContextItemMessage:
		ctx, cancel := stdctx.WithTimeout(stdctx.Background(), 5*time.Second)
		defer cancel()
		err := a.manager.DeleteItem(ctx, msg.ItemID)
		context.Respond(&ContextManagerResponse{
			Success: err == nil,
			Error:   errorToString(err),
		})

	case *RetrieveContextMessage:
		ctx, cancel := stdctx.WithTimeout(stdctx.Background(), 5*time.Second)
		defer cancel()
		retrieval, err := a.manager.RetrieveContext(
			ctx,
			msg.Query,
			msg.MaxTokens,
			msg.Filter,
		)
		context.Respond(&ContextRetrievalResponse{
			Retrieval: retrieval,
			Success:   err == nil,
			Error:     errorToString(err),
		})

	case *ClearContextMessage:
		ctx, cancel := stdctx.WithTimeout(stdctx.Background(), 5*time.Second)
		defer cancel()
		err := a.manager.Clear(ctx)
		context.Respond(&ContextManagerResponse{
			Success: err == nil,
			Error:   errorToString(err),
		})
	}
}

// AddContextItemMessage 添加上下文项目消息
type AddContextItemMessage struct {
	Item *ContextItem
}

// GetContextItemsMessage 获取上下文项目消息
type GetContextItemsMessage struct {
	Filter map[string]interface{}
	Limit  int
}

// DeleteContextItemMessage 删除上下文项目消息
type DeleteContextItemMessage struct {
	ItemID string
}

// RetrieveContextMessage 检索上下文消息
type RetrieveContextMessage struct {
	Query     string
	MaxTokens int
	Filter    map[string]interface{}
}

// ClearContextMessage 清空上下文消息
type ClearContextMessage struct{}

// ContextManagerResponse 上下文管理器响应
type ContextManagerResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// ContextItemsResponse 上下文项目响应
type ContextItemsResponse struct {
	Items   []*ContextItem `json:"items,omitempty"`
	Success bool           `json:"success"`
	Error   string         `json:"error,omitempty"`
}

// ContextRetrievalResponse 上下文检索响应
type ContextRetrievalResponse struct {
	Retrieval *ContextRetrieval `json:"retrieval,omitempty"`
	Success   bool              `json:"success"`
	Error     string            `json:"error,omitempty"`
}

// 辅助函数：错误转字符串
func errorToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
