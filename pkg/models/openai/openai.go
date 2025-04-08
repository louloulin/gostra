package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"bufio"

	"github.com/yourusername/gostra/pkg/models"
)

// 定义常量
const (
	defaultBaseURL = "https://api.openai.com/v1"
	defaultTimeout = 60 * time.Second
)

// OpenAIProvider 实现了ModelProvider接口
type OpenAIProvider struct {
	apiKey     string
	modelID    string
	orgID      string
	baseURL    string
	httpClient *http.Client
}

// Options 定义OpenAI配置选项
type Options struct {
	APIKey  string
	BaseURL string
	OrgID   string
	Model   string
	Timeout time.Duration
}

// NewOpenAIProvider 创建一个新的OpenAI提供者
func NewOpenAIProvider(opts *Options) (*OpenAIProvider, error) {
	if opts == nil {
		return nil, errors.New("options cannot be nil")
	}

	if opts.APIKey == "" {
		return nil, errors.New("API key is required")
	}

	if opts.Model == "" {
		return nil, errors.New("model ID is required")
	}

	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	return &OpenAIProvider{
		apiKey:     opts.APIKey,
		modelID:    opts.Model,
		orgID:      opts.OrgID,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: timeout},
	}, nil
}

// GetID 返回模型ID
func (p *OpenAIProvider) GetID() string {
	return p.modelID
}

// GetProvider 返回提供者名称
func (p *OpenAIProvider) GetProvider() string {
	return "openai"
}

// Generate 生成文本
func (p *OpenAIProvider) Generate(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (string, error) {
	if len(messages) == 0 {
		return "", errors.New("messages array is empty")
	}

	if options == nil {
		options = models.DefaultGenerateOptions()
	}

	// 构建请求体
	requestBody := map[string]interface{}{
		"model":       p.modelID,
		"messages":    convertMessagesToOpenAIFormat(messages),
		"temperature": options.Temperature,
		"max_tokens":  options.MaxTokens,
		"top_p":       options.TopP,
	}

	// 如果有工具，添加工具
	if len(options.Tools) > 0 {
		requestBody["tools"] = options.Tools
	}

	// 添加工具选择策略
	if options.ToolChoice != nil {
		requestBody["tool_choice"] = options.ToolChoice
	}

	// 将请求体转换为JSON
	requestData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %w", err)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", strings.NewReader(string(requestData)))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	// 设置请求头
	p.setHeaders(req)

	// 发送请求
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned error status: %d, body: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("error unmarshaling response: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", errors.New("no response from API")
	}

	return response.Choices[0].Message.Content, nil
}

// Stream 流式生成文本
func (p *OpenAIProvider) Stream(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (<-chan string, error) {
	if len(messages) == 0 {
		return nil, errors.New("messages array is empty")
	}

	if options == nil {
		options = models.DefaultGenerateOptions()
	}

	// 构建请求体
	requestBody := map[string]interface{}{
		"model":       p.modelID,
		"messages":    convertMessagesToOpenAIFormat(messages),
		"temperature": options.Temperature,
		"max_tokens":  options.MaxTokens,
		"top_p":       options.TopP,
		"stream":      true,
	}

	// 如果有工具，添加工具
	if len(options.Tools) > 0 {
		requestBody["tools"] = options.Tools
	}

	// 添加工具选择策略
	if options.ToolChoice != nil {
		requestBody["tool_choice"] = options.ToolChoice
	}

	// 将请求体转换为JSON
	requestData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", strings.NewReader(string(requestData)))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// 设置请求头
	p.setHeaders(req)

	// 发送请求
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned error status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// 创建输出通道
	outputChan := make(chan string, 100)

	// 启动goroutine处理流式响应
	go func() {
		defer resp.Body.Close()
		defer close(outputChan)

		// 使用标准库的bufio.Reader
		reader := bufio.NewReader(resp.Body)
		for {
			// 检查上下文是否已取消
			select {
			case <-ctx.Done():
				return
			default:
				// 继续处理
			}

			// 读取一行
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					return
				}
				// 忽略其他错误，继续读取
				continue
			}

			// 去除空白
			line = strings.TrimSpace(line)

			// 跳过空行
			if line == "" {
				continue
			}

			// 处理数据结束信号
			if line == "data: [DONE]" {
				return
			}

			// 检查并去除SSE前缀
			if strings.HasPrefix(line, "data: ") {
				line = strings.TrimPrefix(line, "data: ")
			} else {
				// 不是数据行，跳过
				continue
			}

			// 解析JSON
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
					FinishReason *string `json:"finish_reason"`
				} `json:"choices"`
			}

			if err := json.Unmarshal([]byte(line), &chunk); err != nil {
				// 解析错误，跳过此行
				continue
			}

			// 检查是否有内容
			if len(chunk.Choices) > 0 {
				content := chunk.Choices[0].Delta.Content
				if content != "" {
					// 发送内容到通道
					select {
					case <-ctx.Done():
						return
					case outputChan <- content:
						// 成功发送
					}
				}

				// 检查是否完成
				if chunk.Choices[0].FinishReason != nil {
					return
				}
			}
		}
	}()

	return outputChan, nil
}

// 设置请求头
func (p *OpenAIProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	if p.orgID != "" {
		req.Header.Set("OpenAI-Organization", p.orgID)
	}
}

// 将消息转换为OpenAI格式
func convertMessagesToOpenAIFormat(messages []models.Message) []map[string]string {
	openaiMessages := make([]map[string]string, len(messages))
	for i, msg := range messages {
		openaiMsg := map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		}
		if msg.Name != "" {
			openaiMsg["name"] = msg.Name
		}
		openaiMessages[i] = openaiMsg
	}
	return openaiMessages
}
