package network

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/yourusername/gostra/pkg/tools"
)

// HTTPMethod 表示HTTP请求方法
type HTTPMethod string

const (
	// MethodGET HTTP GET方法
	MethodGET HTTPMethod = "GET"
	// MethodPOST HTTP POST方法
	MethodPOST HTTPMethod = "POST"
	// MethodPUT HTTP PUT方法
	MethodPUT HTTPMethod = "PUT"
	// MethodDELETE HTTP DELETE方法
	MethodDELETE HTTPMethod = "DELETE"
	// MethodPATCH HTTP PATCH方法
	MethodPATCH HTTPMethod = "PATCH"
	// MethodHEAD HTTP HEAD方法
	MethodHEAD HTTPMethod = "HEAD"
	// MethodOPTIONS HTTP OPTIONS方法
	MethodOPTIONS HTTPMethod = "OPTIONS"
)

// HTTPClientTool 是一个进行HTTP请求的工具
type HTTPClientTool struct {
	client    *http.Client
	baseURL   string
	userAgent string
	headers   map[string]string
}

// HTTPClientConfig 配置HTTP客户端工具
type HTTPClientConfig struct {
	// 超时时间
	Timeout time.Duration
	// 基础URL，可选
	BaseURL string
	// User-Agent头，可选
	UserAgent string
	// 默认请求头，可选
	Headers map[string]string
}

// NewHTTPClientTool 创建一个新的HTTP客户端工具
func NewHTTPClientTool(config *HTTPClientConfig) *HTTPClientTool {
	timeout := 30 * time.Second
	if config != nil && config.Timeout > 0 {
		timeout = config.Timeout
	}

	headers := make(map[string]string)
	if config != nil && config.Headers != nil {
		// 复制默认请求头
		for k, v := range config.Headers {
			headers[k] = v
		}
	}

	userAgent := "Gostra-HTTPClient/1.0"
	if config != nil && config.UserAgent != "" {
		userAgent = config.UserAgent
	}

	baseURL := ""
	if config != nil {
		baseURL = config.BaseURL
	}

	return &HTTPClientTool{
		client: &http.Client{
			Timeout: timeout,
		},
		baseURL:   baseURL,
		userAgent: userAgent,
		headers:   headers,
	}
}

// GetID 返回工具ID
func (t *HTTPClientTool) GetID() string {
	return "http_client"
}

// GetDescription 返回工具描述
func (t *HTTPClientTool) GetDescription() string {
	return "向指定URL发送HTTP请求，支持GET、POST、PUT、DELETE等方法。可以设置请求头、请求体，并返回响应内容。"
}

// GetInputSchema 返回输入参数的JSON模式
func (t *HTTPClientTool) GetInputSchema() tools.Schema {
	schema := tools.NewSimpleSchema(tools.TypeObject, "HTTP请求参数")

	// 添加URL参数
	urlSchema := tools.NewSimpleSchema(tools.TypeString, "请求的URL")
	schema.AddProperty("url", urlSchema, true)

	// 添加方法参数
	methodSchema := tools.NewSimpleSchema(tools.TypeString, "HTTP方法 (GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS)")
	// 注: SimpleSchema不直接支持Enum，这里简化处理
	schema.AddProperty("method", methodSchema, false)

	// 添加头部参数
	headersSchema := tools.NewSimpleSchema(tools.TypeObject, "请求头")
	schema.AddProperty("headers", headersSchema, false)

	// 添加查询参数
	paramsSchema := tools.NewSimpleSchema(tools.TypeObject, "URL查询参数")
	schema.AddProperty("params", paramsSchema, false)

	// 添加请求体参数
	bodySchema := tools.NewSimpleSchema(tools.TypeObject, "请求体 (用于POST, PUT等方法)")
	schema.AddProperty("body", bodySchema, false)

	// 添加原始请求体参数
	rawBodySchema := tools.NewSimpleSchema(tools.TypeString, "原始请求体字符串 (当body不是JSON对象时使用)")
	schema.AddProperty("rawBody", rawBodySchema, false)

	// 添加超时参数
	timeoutSchema := tools.NewSimpleSchema(tools.TypeNumber, "请求超时时间(秒)")
	schema.AddProperty("timeout", timeoutSchema, false)

	return schema
}

// HTTPResponse 表示HTTP响应
type HTTPResponse struct {
	StatusCode int               `json:"statusCode"` // HTTP状态码
	Status     string            `json:"status"`     // HTTP状态描述
	Headers    map[string]string `json:"headers"`    // 响应头
	Body       interface{}       `json:"body"`       // 响应体(尝试解析为JSON)
	Text       string            `json:"text"`       // 原始响应文本
}

// Execute 执行HTTP请求
func (t *HTTPClientTool) Execute(params map[string]interface{}, options *tools.ExecuteOptions) (interface{}, error) {
	// 创建上下文，如果options提供了上下文则使用它
	ctx := context.Background()
	if options != nil && options.Context != nil {
		if contextValue, ok := options.Context.(context.Context); ok {
			ctx = contextValue
		}
	}

	// 获取URL
	urlRaw, ok := params["url"].(string)
	if !ok || urlRaw == "" {
		return nil, tools.NewValidationError("URL is required")
	}

	// 构建完整URL
	fullURL := urlRaw
	if t.baseURL != "" && !isAbsoluteURL(urlRaw) {
		fullURL = t.baseURL + urlRaw
	}

	// 获取HTTP方法
	methodRaw, ok := params["method"].(string)
	if !ok || methodRaw == "" {
		methodRaw = string(MethodGET)
	}
	method := HTTPMethod(methodRaw)

	// 准备请求体
	var bodyReader io.Reader
	if method == MethodPOST || method == MethodPUT || method == MethodPATCH {
		if body, ok := params["body"]; ok && body != nil {
			bodyJSON, err := json.Marshal(body)
			if err != nil {
				return nil, tools.NewValidationError("invalid request body: " + err.Error())
			}
			bodyReader = bytes.NewReader(bodyJSON)
		} else if rawBody, ok := params["rawBody"].(string); ok && rawBody != "" {
			bodyReader = bytes.NewReader([]byte(rawBody))
		}
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, string(method), fullURL, bodyReader)
	if err != nil {
		return nil, tools.NewValidationError("failed to create request: " + err.Error())
	}

	// 设置超时
	timeout := 30.0
	if timeoutRaw, ok := params["timeout"].(float64); ok && timeoutRaw > 0 {
		timeout = timeoutRaw
	}
	client := *t.client
	client.Timeout = time.Duration(timeout) * time.Second

	// 设置查询参数
	if paramsRaw, ok := params["params"].(map[string]interface{}); ok && len(paramsRaw) > 0 {
		q := req.URL.Query()
		for key, value := range paramsRaw {
			q.Add(key, toString(value))
		}
		req.URL.RawQuery = q.Encode()
	}

	// 设置默认请求头
	req.Header.Set("User-Agent", t.userAgent)
	for key, value := range t.headers {
		req.Header.Set(key, value)
	}

	// 设置JSON内容类型（如果有请求体）
	if bodyReader != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// 设置自定义请求头
	if headersRaw, ok := params["headers"].(map[string]interface{}); ok && len(headersRaw) > 0 {
		for key, value := range headersRaw {
			req.Header.Set(key, toString(value))
		}
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return nil, tools.NewValidationError("request failed: " + err.Error())
	}
	defer resp.Body.Close()

	// 处理响应
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, tools.NewValidationError("failed to read response: " + err.Error())
	}

	// 构建响应
	response := HTTPResponse{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Headers:    extractHeaders(resp.Header),
		Text:       string(responseBody),
	}

	// 尝试解析JSON响应
	var jsonResponse interface{}
	if err := json.Unmarshal(responseBody, &jsonResponse); err == nil {
		response.Body = jsonResponse
	} else {
		response.Body = response.Text
	}

	return response, nil
}

// 辅助函数：检查URL是否是绝对URL
func isAbsoluteURL(url string) bool {
	return len(url) > 0 && (url[0] == '/' || (len(url) >= 4 && url[0:4] == "http"))
}

// 辅助函数：提取HTTP响应头
func extractHeaders(header http.Header) map[string]string {
	result := make(map[string]string)
	for name, values := range header {
		if len(values) > 0 {
			result[name] = values[0]
		}
	}
	return result
}

// 辅助函数：将值转换为字符串
func toString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	default:
		bytes, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(bytes)
	}
}
