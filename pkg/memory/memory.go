package memory

import (
	"context"
	"errors"
	"time"
)

// Message 表示存储在内存中的消息
type Message struct {
	ID        string                 `json:"id"`
	Role      string                 `json:"role"`
	Content   string                 `json:"content"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

// Thread 表示一个内存线程，包含一系列消息
type Thread struct {
	ID        string                 `json:"id"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

// MemoryProvider 定义内存存储的接口
type MemoryProvider interface {
	// 线程相关操作
	CreateThread(ctx context.Context, metadata map[string]interface{}) (*Thread, error)
	GetThread(ctx context.Context, threadID string) (*Thread, error)
	UpdateThread(ctx context.Context, threadID string, metadata map[string]interface{}) (*Thread, error)
	DeleteThread(ctx context.Context, threadID string) error
	ListThreads(ctx context.Context, limit int, offset int) ([]*Thread, error)

	// 消息相关操作
	AddMessage(ctx context.Context, threadID string, role string, content string, metadata map[string]interface{}) (*Message, error)
	GetMessages(ctx context.Context, threadID string, limit int, offset int) ([]*Message, error)
	DeleteMessages(ctx context.Context, threadID string, messageIDs []string) error
}

// InMemoryProvider 实现了基于内存的存储
type InMemoryProvider struct {
	threads  map[string]*Thread
	messages map[string][]*Message
}

// NewInMemoryProvider 创建一个新的内存存储提供者
func NewInMemoryProvider() *InMemoryProvider {
	return &InMemoryProvider{
		threads:  make(map[string]*Thread),
		messages: make(map[string][]*Message),
	}
}

// CreateThread 创建一个新的线程
func (p *InMemoryProvider) CreateThread(ctx context.Context, metadata map[string]interface{}) (*Thread, error) {
	threadID := generateID()
	thread := &Thread{
		ID:        threadID,
		Metadata:  metadata,
		CreatedAt: time.Now(),
	}
	p.threads[threadID] = thread
	p.messages[threadID] = []*Message{}
	return thread, nil
}

// GetThread 获取一个线程
func (p *InMemoryProvider) GetThread(ctx context.Context, threadID string) (*Thread, error) {
	if thread, ok := p.threads[threadID]; ok {
		return thread, nil
	}
	return nil, errors.New("thread not found")
}

// UpdateThread 更新线程元数据
func (p *InMemoryProvider) UpdateThread(ctx context.Context, threadID string, metadata map[string]interface{}) (*Thread, error) {
	if thread, ok := p.threads[threadID]; ok {
		thread.Metadata = metadata
		return thread, nil
	}
	return nil, errors.New("thread not found")
}

// DeleteThread 删除一个线程及其消息
func (p *InMemoryProvider) DeleteThread(ctx context.Context, threadID string) error {
	if _, ok := p.threads[threadID]; ok {
		delete(p.threads, threadID)
		delete(p.messages, threadID)
		return nil
	}
	return errors.New("thread not found")
}

// ListThreads 列出所有线程
func (p *InMemoryProvider) ListThreads(ctx context.Context, limit int, offset int) ([]*Thread, error) {
	threads := make([]*Thread, 0, len(p.threads))
	for _, thread := range p.threads {
		threads = append(threads, thread)
	}

	// 分页
	if offset >= len(threads) {
		return []*Thread{}, nil
	}

	end := offset + limit
	if end > len(threads) {
		end = len(threads)
	}

	return threads[offset:end], nil
}

// AddMessage 添加一条消息到线程
func (p *InMemoryProvider) AddMessage(ctx context.Context, threadID string, role string, content string, metadata map[string]interface{}) (*Message, error) {
	if _, ok := p.threads[threadID]; !ok {
		return nil, errors.New("thread not found")
	}

	msg := &Message{
		ID:        generateID(),
		Role:      role,
		Content:   content,
		Metadata:  metadata,
		CreatedAt: time.Now(),
	}

	p.messages[threadID] = append(p.messages[threadID], msg)
	return msg, nil
}

// GetMessages 获取线程中的消息
func (p *InMemoryProvider) GetMessages(ctx context.Context, threadID string, limit int, offset int) ([]*Message, error) {
	if _, ok := p.threads[threadID]; !ok {
		return nil, errors.New("thread not found")
	}

	msgs := p.messages[threadID]

	// 分页
	if offset >= len(msgs) {
		return []*Message{}, nil
	}

	end := offset + limit
	if end > len(msgs) {
		end = len(msgs)
	}

	return msgs[offset:end], nil
}

// DeleteMessages 从线程中删除消息
func (p *InMemoryProvider) DeleteMessages(ctx context.Context, threadID string, messageIDs []string) error {
	if _, ok := p.threads[threadID]; !ok {
		return errors.New("thread not found")
	}

	// 创建一个消息ID到索引的映射
	idMap := make(map[string]bool)
	for _, id := range messageIDs {
		idMap[id] = true
	}

	// 过滤出不需要删除的消息
	filtered := make([]*Message, 0)
	for _, msg := range p.messages[threadID] {
		if !idMap[msg.ID] {
			filtered = append(filtered, msg)
		}
	}

	p.messages[threadID] = filtered
	return nil
}

// 生成唯一ID
func generateID() string {
	return "id_" + time.Now().Format("20060102150405") + "_" + randomString(8)
}

// 生成随机字符串
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		result[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		time.Sleep(1 * time.Nanosecond)
	}
	return string(result)
}
