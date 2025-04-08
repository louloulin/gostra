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

// GenerateWithFunctionCalls 生成文本并支持函数调用
func (p *OpenAIProvider) GenerateWithFunctionCalls(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (*models.ResponseWithFunctionCalls, error) {
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
	}

	// 如果有工具，添加工具
	if len(options.Tools) > 0 {
		requestBody["tools"] = options.Tools
	}

	// 添加工具选择策略
	if options.ToolChoice != nil {
		requestBody["tool_choice"] = options.ToolChoice
	}

	// 添加函数调用策略（兼容旧版API）
	if options.FunctionCall != nil {
		requestBody["function_call"] = options.FunctionCall
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
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned error status: %d, body: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var openaiResp struct {
		Choices []struct {
			Message struct {
				Content      string `json:"content"`
				FunctionCall *struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function_call"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &openaiResp); err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %w", err)
	}

	if len(openaiResp.Choices) == 0 {
		return nil, errors.New("no response from API")
	}

	// 处理响应
	choice := openaiResp.Choices[0]
	response := &models.ResponseWithFunctionCalls{
		Text:         choice.Message.Content,
		FinishReason: choice.FinishReason,
	}

	// 处理函数调用（旧版API）
	if choice.Message.FunctionCall != nil {
		response.FunctionCall = &models.FunctionCall{
			Name:      choice.Message.FunctionCall.Name,
			Arguments: choice.Message.FunctionCall.Arguments,
		}
	}

	// 处理工具调用（新版API）
	if len(choice.Message.ToolCalls) > 0 {
		response.ToolCalls = make([]models.ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			response.ToolCalls[i] = models.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: models.FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}
	}

	return response, nil
}

// StreamWithFunctionCalls 流式生成文本并支持函数调用
func (p *OpenAIProvider) StreamWithFunctionCalls(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (<-chan *models.ResponseChunk, error) {
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

	// 添加函数调用策略（兼容旧版API）
	if options.FunctionCall != nil {
		requestBody["function_call"] = options.FunctionCall
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
	outputChan := make(chan *models.ResponseChunk, 100)

	// 跟踪工具调用累积状态
	type toolCallState struct {
		id        string
		type_     string
		name      string
		arguments strings.Builder
		index     int
	}

	// 启动goroutine处理流式响应
	go func() {
		defer resp.Body.Close()
		defer close(outputChan)

		// 使用标准库的bufio.Reader
		reader := bufio.NewReader(resp.Body)

		// 跟踪当前正在处理的工具调用
		toolCallStates := make(map[string]*toolCallState)
		functionCallArgs := strings.Builder{}
		var currentFunctionName string
		var toolIDs []string

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
				// 发送最终块，表示流结束
				outputChan <- &models.ResponseChunk{
					IsFinished: true,
				}
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
			var streamResp struct {
				Choices []struct {
					Delta struct {
						Content      string `json:"content"`
						FunctionCall *struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function_call"`
						ToolCalls []struct {
							Index    int    `json:"index"`
							ID       string `json:"id"`
							Type     string `json:"type"`
							Function struct {
								Name      string `json:"name"`
								Arguments string `json:"arguments"`
							} `json:"function"`
						} `json:"tool_calls"`
					} `json:"delta"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
			}

			if err := json.Unmarshal([]byte(line), &streamResp); err != nil {
				// 解析错误，跳过此行
				continue
			}

			// 如果没有选择，跳过
			if len(streamResp.Choices) == 0 {
				continue
			}

			choice := streamResp.Choices[0]
			chunk := &models.ResponseChunk{}

			// 处理文本内容
			if choice.Delta.Content != "" {
				chunk.Text = choice.Delta.Content
			}

			// 处理函数调用（旧版API）
			if choice.Delta.FunctionCall != nil {
				if choice.Delta.FunctionCall.Name != "" {
					currentFunctionName = choice.Delta.FunctionCall.Name
				}

				if choice.Delta.FunctionCall.Arguments != "" {
					functionCallArgs.WriteString(choice.Delta.FunctionCall.Arguments)
				}

				chunk.FunctionCallChunk = &models.FunctionCallChunk{
					Name:      currentFunctionName,
					Arguments: choice.Delta.FunctionCall.Arguments,
					Index:     0,
				}
			}

			// 处理工具调用（新版API）
			if len(choice.Delta.ToolCalls) > 0 {
				for _, tc := range choice.Delta.ToolCalls {
					id := tc.ID
					if id == "" && len(toolIDs) > tc.Index {
						id = toolIDs[tc.Index]
					}

					// 如果是新的工具调用
					if id != "" && toolCallStates[id] == nil {
						toolCallStates[id] = &toolCallState{
							id:    id,
							type_: tc.Type,
							index: tc.Index,
						}

						if len(toolIDs) <= tc.Index {
							// 确保toolIDs数组足够大
							for i := len(toolIDs); i <= tc.Index; i++ {
								toolIDs = append(toolIDs, "")
							}
						}
						toolIDs[tc.Index] = id
					}

					// 更新工具调用状态
					if state := toolCallStates[id]; state != nil {
						if tc.Function.Name != "" {
							state.name = tc.Function.Name
						}
						if tc.Function.Arguments != "" {
							state.arguments.WriteString(tc.Function.Arguments)
						}

						chunk.ToolCallChunk = &models.ToolCallChunk{
							ID:   id,
							Type: state.type_,
							Function: &models.FunctionCallChunk{
								Name:      state.name,
								Arguments: tc.Function.Arguments,
							},
							Index: tc.Index,
						}
					}
				}
			}

			// 检查是否完成
			if choice.FinishReason != "" {
				chunk.IsFinished = true
				chunk.FinishReason = choice.FinishReason

				// 如果是函数调用完成，发送完整的函数调用
				if choice.FinishReason == "function_call" && currentFunctionName != "" {
					chunk.FunctionCallChunk = &models.FunctionCallChunk{
						Name:      currentFunctionName,
						Arguments: functionCallArgs.String(),
						Index:     0,
					}
				}

				// 如果是工具调用完成，发送所有完整的工具调用
				if choice.FinishReason == "tool_calls" && len(toolCallStates) > 0 {
					// 只发送第一个工具调用，因为我们每个块只能包含一个工具调用
					// 其他工具调用会在下一个迭代中处理
					for id, state := range toolCallStates {
						chunk.ToolCallChunk = &models.ToolCallChunk{
							ID:   id,
							Type: state.type_,
							Function: &models.FunctionCallChunk{
								Name:      state.name,
								Arguments: state.arguments.String(),
							},
							Index: state.index,
						}
						break
					}
				}
			}

			// 如果有内容，发送块
			if chunk.Text != "" || chunk.FunctionCallChunk != nil || chunk.ToolCallChunk != nil || chunk.IsFinished {
				select {
				case <-ctx.Done():
					return
				case outputChan <- chunk:
					// 成功发送
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
