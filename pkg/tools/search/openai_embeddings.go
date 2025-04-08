package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// OpenAIEmbeddingModel 表示OpenAI嵌入模型
type OpenAIEmbeddingModel string

const (
	// ModelTextEmbeddingAda002 是OpenAI的text-embedding-ada-002模型
	ModelTextEmbeddingAda002 OpenAIEmbeddingModel = "text-embedding-ada-002"
	// ModelTextEmbedding3Small 是OpenAI的text-embedding-3-small模型
	ModelTextEmbedding3Small OpenAIEmbeddingModel = "text-embedding-3-small"
	// ModelTextEmbedding3Large 是OpenAI的text-embedding-3-large模型
	ModelTextEmbedding3Large OpenAIEmbeddingModel = "text-embedding-3-large"
)

// OpenAIEmbeddingOptions OpenAI嵌入选项
type OpenAIEmbeddingOptions struct {
	// APIKey OpenAI API密钥
	APIKey string
	// Model 模型名称
	Model OpenAIEmbeddingModel
	// BaseURL API基础URL
	BaseURL string
	// Timeout 请求超时时间
	Timeout time.Duration
	// BatchSize 批处理大小
	BatchSize int
	// MaxRetries 最大重试次数
	MaxRetries int
	// RetryWait 重试等待时间
	RetryWait time.Duration
	// OrgID 组织ID
	OrgID string
}

// OpenAIEmbeddingProvider 提供OpenAI嵌入功能
type OpenAIEmbeddingProvider struct {
	options   OpenAIEmbeddingOptions
	client    *http.Client
	rateLock  sync.Mutex
	lastQuery time.Time
}

// OpenAIEmbeddingRequest OpenAI嵌入请求结构
type OpenAIEmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// OpenAIEmbeddingResponse OpenAI嵌入响应结构
type OpenAIEmbeddingResponse struct {
	Object string                      `json:"object"`
	Data   []OpenAIEmbeddingData       `json:"data"`
	Model  string                      `json:"model"`
	Usage  OpenAIEmbeddingUsage        `json:"usage"`
	Error  *OpenAIEmbeddingErrorObject `json:"error,omitempty"`
}

// OpenAIEmbeddingData OpenAI嵌入数据
type OpenAIEmbeddingData struct {
	Object    string    `json:"object"`
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

// OpenAIEmbeddingUsage OpenAI嵌入用量
type OpenAIEmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// OpenAIEmbeddingErrorObject OpenAI错误对象
type OpenAIEmbeddingErrorObject struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// NewOpenAIEmbeddingProvider 创建新的OpenAI嵌入提供者
func NewOpenAIEmbeddingProvider(options OpenAIEmbeddingOptions) (*OpenAIEmbeddingProvider, error) {
	if options.APIKey == "" {
		return nil, errors.New("必须提供API密钥")
	}

	// 设置默认值
	if options.Model == "" {
		options.Model = ModelTextEmbedding3Small
	}
	if options.BaseURL == "" {
		options.BaseURL = "https://api.openai.com/v1/embeddings"
	}
	if options.Timeout == 0 {
		options.Timeout = 30 * time.Second
	}
	if options.BatchSize == 0 {
		options.BatchSize = 20
	}
	if options.MaxRetries == 0 {
		options.MaxRetries = 3
	}
	if options.RetryWait == 0 {
		options.RetryWait = 1 * time.Second
	}

	client := &http.Client{
		Timeout: options.Timeout,
	}

	return &OpenAIEmbeddingProvider{
		options:   options,
		client:    client,
		lastQuery: time.Now().Add(-10 * time.Second), // 初始化为过去时间，避免首次请求等待
	}, nil
}

// GetEmbedding 获取单个文本的嵌入向量
func (p *OpenAIEmbeddingProvider) GetEmbedding(text string) ([]float32, error) {
	embeddings, err := p.GetEmbeddings([]string{text})
	if err != nil {
		return nil, err
	}

	if len(embeddings) == 0 {
		return nil, errors.New("无法获取嵌入向量")
	}

	return embeddings[0], nil
}

// GetEmbeddings 批量获取文本的嵌入向量
func (p *OpenAIEmbeddingProvider) GetEmbeddings(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	// 分批处理，避免超过API限制
	if len(texts) > p.options.BatchSize {
		return p.batchProcess(texts)
	}

	// 限制请求频率
	p.rateLock.Lock()
	timeSinceLastQuery := time.Since(p.lastQuery)
	if timeSinceLastQuery < time.Second {
		time.Sleep(time.Second - timeSinceLastQuery)
	}
	p.lastQuery = time.Now()
	p.rateLock.Unlock()

	// 准备请求
	reqBody := OpenAIEmbeddingRequest{
		Model: string(p.options.Model),
		Input: texts,
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("无法序列化请求: %w", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequest("POST", p.options.BaseURL, bytes.NewBuffer(reqJSON))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.options.APIKey)
	if p.options.OrgID != "" {
		req.Header.Set("OpenAI-Organization", p.options.OrgID)
	}

	// 发送请求，带重试逻辑
	var resp *http.Response
	var respBody []byte

	for retry := 0; retry <= p.options.MaxRetries; retry++ {
		if retry > 0 {
			time.Sleep(p.options.RetryWait * time.Duration(retry))
		}

		resp, err = p.client.Do(req)
		if err != nil {
			if retry == p.options.MaxRetries {
				return nil, fmt.Errorf("发送请求失败: %w", err)
			}
			continue
		}

		defer resp.Body.Close()
		respBody, err = io.ReadAll(resp.Body)
		if err != nil {
			if retry == p.options.MaxRetries {
				return nil, fmt.Errorf("读取响应失败: %w", err)
			}
			continue
		}

		// 如果不是429 (Too Many Requests)或5xx错误，则不重试
		if resp.StatusCode != 429 && resp.StatusCode < 500 {
			break
		}

		if retry == p.options.MaxRetries {
			return nil, fmt.Errorf("API返回错误状态码: %d", resp.StatusCode)
		}
	}

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		var errorResp OpenAIEmbeddingResponse
		if err := json.Unmarshal(respBody, &errorResp); err == nil && errorResp.Error != nil {
			return nil, fmt.Errorf("API错误: %s", errorResp.Error.Message)
		}
		return nil, fmt.Errorf("API返回非200状态码: %d", resp.StatusCode)
	}

	// 解析响应
	var embeddingResp OpenAIEmbeddingResponse
	if err := json.Unmarshal(respBody, &embeddingResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查数据条数是否匹配
	if len(embeddingResp.Data) != len(texts) {
		return nil, fmt.Errorf("返回的嵌入数量与请求不匹配: 期望 %d, 实际 %d", len(texts), len(embeddingResp.Data))
	}

	// 提取嵌入向量
	embeddings := make([][]float32, len(embeddingResp.Data))
	for _, data := range embeddingResp.Data {
		embeddings[data.Index] = data.Embedding
	}

	return embeddings, nil
}

// batchProcess 批量处理嵌入请求
func (p *OpenAIEmbeddingProvider) batchProcess(texts []string) ([][]float32, error) {
	batchSize := p.options.BatchSize
	numBatches := (len(texts) + batchSize - 1) / batchSize
	allEmbeddings := make([][]float32, len(texts))

	for batchIndex := 0; batchIndex < numBatches; batchIndex++ {
		start := batchIndex * batchSize
		end := start + batchSize
		if end > len(texts) {
			end = len(texts)
		}

		batchTexts := texts[start:end]
		batchEmbeddings, err := p.GetEmbeddings(batchTexts)
		if err != nil {
			return nil, fmt.Errorf("处理批次 %d 失败: %w", batchIndex, err)
		}

		for j, embedding := range batchEmbeddings {
			allEmbeddings[start+j] = embedding
		}
	}

	return allEmbeddings, nil
}

// WithContext 返回支持上下文的嵌入提供者
func (p *OpenAIEmbeddingProvider) WithContext(ctx context.Context) *OpenAIEmbeddingProviderWithContext {
	return &OpenAIEmbeddingProviderWithContext{
		provider: p,
		ctx:      ctx,
	}
}

// OpenAIEmbeddingProviderWithContext 带上下文的OpenAI嵌入提供者
type OpenAIEmbeddingProviderWithContext struct {
	provider *OpenAIEmbeddingProvider
	ctx      context.Context
}

// GetEmbedding 获取单个文本的嵌入向量（带上下文）
func (p *OpenAIEmbeddingProviderWithContext) GetEmbedding(text string) ([]float32, error) {
	if p.ctx.Err() != nil {
		return nil, p.ctx.Err()
	}
	return p.provider.GetEmbedding(text)
}

// GetEmbeddings 批量获取文本的嵌入向量（带上下文）
func (p *OpenAIEmbeddingProviderWithContext) GetEmbeddings(texts []string) ([][]float32, error) {
	if p.ctx.Err() != nil {
		return nil, p.ctx.Err()
	}
	return p.provider.GetEmbeddings(texts)
}
