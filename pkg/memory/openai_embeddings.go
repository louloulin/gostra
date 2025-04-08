package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// OpenAIEmbeddingProvider 实现了基于OpenAI API的嵌入服务
type OpenAIEmbeddingProvider struct {
	apiKey       string
	model        string
	httpClient   *http.Client
	baseURL      string
	maxBatchSize int
	mu           sync.Mutex
	lastRequest  time.Time
	rateLimit    time.Duration // 请求间隔时间
}

// OpenAIEmbeddingOptions 配置选项
type OpenAIEmbeddingOptions struct {
	APIKey       string
	Model        string
	BaseURL      string
	Timeout      time.Duration
	MaxBatchSize int
	RateLimit    time.Duration // 请求间隔时间
}

// NewOpenAIEmbeddingProvider 创建一个新的OpenAI嵌入提供者
func NewOpenAIEmbeddingProvider(opts *OpenAIEmbeddingOptions) (*OpenAIEmbeddingProvider, error) {
	if opts == nil {
		return nil, fmt.Errorf("options cannot be nil")
	}

	if opts.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	model := opts.Model
	if model == "" {
		model = "text-embedding-3-small" // 默认模型
	}

	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	maxBatchSize := opts.MaxBatchSize
	if maxBatchSize <= 0 {
		maxBatchSize = 16 // 默认批处理大小
	}

	rateLimit := opts.RateLimit
	if rateLimit == 0 {
		rateLimit = 200 * time.Millisecond // 默认限制：5 RPS
	}

	return &OpenAIEmbeddingProvider{
		apiKey:       opts.APIKey,
		model:        model,
		httpClient:   &http.Client{Timeout: timeout},
		baseURL:      baseURL,
		maxBatchSize: maxBatchSize,
		rateLimit:    rateLimit,
	}, nil
}

// GetEmbedding 获取文本的嵌入向量
func (p *OpenAIEmbeddingProvider) GetEmbedding(ctx context.Context, text string) (Embedding, error) {
	embeddings, err := p.GetEmbeddings(ctx, []string{text})
	if err != nil {
		return nil, err
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return embeddings[0], nil
}

// GetEmbeddings 批量获取文本的嵌入向量
func (p *OpenAIEmbeddingProvider) GetEmbeddings(ctx context.Context, texts []string) ([]Embedding, error) {
	if len(texts) == 0 {
		return []Embedding{}, nil
	}

	// 实现简单的速率限制
	p.mu.Lock()
	now := time.Now()
	if p.lastRequest.Add(p.rateLimit).After(now) {
		wait := p.lastRequest.Add(p.rateLimit).Sub(now)
		p.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
			// 继续执行
		}
		p.mu.Lock()
	}
	p.lastRequest = time.Now()
	p.mu.Unlock()

	// 准备请求体
	requestBody := map[string]interface{}{
		"model": p.model,
		"input": texts,
	}

	requestData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/embeddings", bytes.NewBuffer(requestData))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	// 发送请求
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned error %d: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var response struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %w", err)
	}

	// 验证响应
	if len(response.Data) != len(texts) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(response.Data))
	}

	// 提取嵌入向量
	embeddings := make([]Embedding, len(response.Data))
	for i, data := range response.Data {
		embeddings[i] = data.Embedding
	}

	return embeddings, nil
}

// BatchProcess 批量处理文本以获取嵌入向量
func (p *OpenAIEmbeddingProvider) BatchProcess(ctx context.Context, texts []string) ([]Embedding, error) {
	if len(texts) == 0 {
		return []Embedding{}, nil
	}

	// 如果文本数量小于最大批量大小，直接处理
	if len(texts) <= p.maxBatchSize {
		return p.GetEmbeddings(ctx, texts)
	}

	// 分批处理
	var allEmbeddings []Embedding
	for i := 0; i < len(texts); i += p.maxBatchSize {
		end := i + p.maxBatchSize
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[i:end]
		embeddings, err := p.GetEmbeddings(ctx, batch)
		if err != nil {
			return nil, err
		}

		allEmbeddings = append(allEmbeddings, embeddings...)
	}

	return allEmbeddings, nil
}
