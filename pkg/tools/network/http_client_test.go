package network

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPClientToolGetID(t *testing.T) {
	client := NewHTTPClientTool(nil)
	if id := client.GetID(); id != "http_client" {
		t.Errorf("Expected tool ID 'http_client', got '%s'", id)
	}
}

func TestHTTPClientToolGetDescription(t *testing.T) {
	client := NewHTTPClientTool(nil)
	if desc := client.GetDescription(); desc == "" {
		t.Error("Tool description should not be empty")
	}
}

func TestHTTPClientToolGetInputSchema(t *testing.T) {
	client := NewHTTPClientTool(nil)
	schema := client.GetInputSchema()

	if schema == nil {
		t.Fatal("Input schema should not be nil")
	}

	// 验证模式
	jsonSchema, err := schema.JSONSchema()
	if err != nil {
		t.Fatalf("Failed to get JSON schema: %v", err)
	}

	// 检查必填字段
	required, ok := jsonSchema["required"].([]string)
	if !ok {
		t.Fatal("Required field list should be a string array")
	}

	hasURL := false
	for _, field := range required {
		if field == "url" {
			hasURL = true
			break
		}
	}

	if !hasURL {
		t.Error("URL should be a required field")
	}

	// 检查属性
	properties, ok := jsonSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Properties should be a map")
	}

	// 检查是否包含必要的属性
	requiredProps := []string{"url", "method", "headers", "body"}
	for _, prop := range requiredProps {
		if _, exists := properties[prop]; !exists {
			t.Errorf("Schema should have '%s' property", prop)
		}
	}
}

func TestHTTPClientToolExecuteSuccess(t *testing.T) {
	// 创建模拟服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 检查请求方法
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}

		// 返回JSON响应
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Hello, World!"}`))
	}))
	defer server.Close()

	// 创建HTTP客户端工具
	client := NewHTTPClientTool(nil)

	// 执行请求
	params := map[string]interface{}{
		"url":    server.URL,
		"method": "GET",
	}

	response, err := client.Execute(params, nil)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}

	// 检查响应
	httpResp, ok := response.(HTTPResponse)
	if !ok {
		t.Fatal("Response should be of type HTTPResponse")
	}

	if httpResp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, httpResp.StatusCode)
	}

	// 检查JSON解析
	messageObj, ok := httpResp.Body.(map[string]interface{})
	if !ok {
		t.Fatal("Response body should be parsed as JSON object")
	}

	message, ok := messageObj["message"].(string)
	if !ok || message != "Hello, World!" {
		t.Errorf("Expected message 'Hello, World!', got '%v'", messageObj["message"])
	}
}

func TestHTTPClientToolExecuteWithParams(t *testing.T) {
	// 创建模拟服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 检查查询参数
		if r.URL.Query().Get("param1") != "value1" {
			t.Errorf("Expected param1=value1, got param1=%s", r.URL.Query().Get("param1"))
		}
		if r.URL.Query().Get("param2") != "value2" {
			t.Errorf("Expected param2=value2, got param2=%s", r.URL.Query().Get("param2"))
		}

		// 返回成功响应
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	// 创建HTTP客户端工具
	client := NewHTTPClientTool(nil)

	// 执行带查询参数的请求
	params := map[string]interface{}{
		"url": server.URL,
		"params": map[string]interface{}{
			"param1": "value1",
			"param2": "value2",
		},
	}

	_, err := client.Execute(params, nil)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
}

func TestHTTPClientToolExecuteWithHeaders(t *testing.T) {
	// 创建模拟服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 检查请求头
		if r.Header.Get("X-Custom-Header") != "custom-value" {
			t.Errorf("Expected X-Custom-Header=custom-value, got X-Custom-Header=%s", r.Header.Get("X-Custom-Header"))
		}
		if r.Header.Get("Authorization") != "Bearer token123" {
			t.Errorf("Expected Authorization=Bearer token123, got Authorization=%s", r.Header.Get("Authorization"))
		}

		// 返回成功响应
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	// 创建HTTP客户端工具
	client := NewHTTPClientTool(nil)

	// 执行带自定义头的请求
	params := map[string]interface{}{
		"url": server.URL,
		"headers": map[string]interface{}{
			"X-Custom-Header": "custom-value",
			"Authorization":   "Bearer token123",
		},
	}

	_, err := client.Execute(params, nil)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
}

func TestHTTPClientToolExecutePost(t *testing.T) {
	// 创建模拟服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 检查请求方法
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		// 检查内容类型
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type=application/json, got Content-Type=%s", r.Header.Get("Content-Type"))
		}

		// 解析请求体
		var requestBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("Failed to decode request body: %v", err)
		}

		// 检查请求体字段
		if name, ok := requestBody["name"].(string); !ok || name != "test" {
			t.Errorf("Expected name=test, got name=%v", requestBody["name"])
		}
		if age, ok := requestBody["age"].(float64); !ok || age != 30 {
			t.Errorf("Expected age=30, got age=%v", requestBody["age"])
		}

		// 返回成功响应
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id": 123}`))
	}))
	defer server.Close()

	// 创建HTTP客户端工具
	client := NewHTTPClientTool(nil)

	// 执行POST请求
	params := map[string]interface{}{
		"url":    server.URL,
		"method": "POST",
		"body": map[string]interface{}{
			"name": "test",
			"age":  30,
		},
	}

	response, err := client.Execute(params, nil)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}

	// 检查响应
	httpResp, ok := response.(HTTPResponse)
	if !ok {
		t.Fatal("Response should be of type HTTPResponse")
	}

	if httpResp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status code %d, got %d", http.StatusCreated, httpResp.StatusCode)
	}
}

func TestHTTPClientToolExecuteRawBody(t *testing.T) {
	// 创建模拟服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 检查请求方法
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		// 读取请求体
		bodyBytes := make([]byte, 1024)
		n, err := r.Body.Read(bodyBytes)
		if err != nil && err.Error() != "EOF" {
			t.Fatalf("Failed to read request body: %v", err)
		}

		// 检查请求体内容
		expectedBody := "raw text body"
		if string(bodyBytes[:n]) != expectedBody {
			t.Errorf("Expected body '%s', got '%s'", expectedBody, string(bodyBytes[:n]))
		}

		// 返回成功响应
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	// 创建HTTP客户端工具
	client := NewHTTPClientTool(nil)

	// 执行带原始请求体的请求
	params := map[string]interface{}{
		"url":     server.URL,
		"method":  "POST",
		"rawBody": "raw text body",
	}

	_, err := client.Execute(params, nil)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
}

func TestHTTPClientToolExecuteError(t *testing.T) {
	// 创建HTTP客户端工具
	client := NewHTTPClientTool(nil)

	// 执行请求到不存在的服务器
	params := map[string]interface{}{
		"url":     "http://localhost:54321", // 使用一个极不可能在用的端口
		"timeout": 1,                        // 设置1秒超时，加快测试速度
	}

	_, err := client.Execute(params, nil)
	if err == nil {
		t.Fatal("Expected error for request to non-existent server, got nil")
	}
}

func TestHTTPClientToolValidateURL(t *testing.T) {
	// 创建HTTP客户端工具
	client := NewHTTPClientTool(nil)

	// 测试缺少URL参数
	params := map[string]interface{}{
		"method": "GET",
	}

	_, err := client.Execute(params, nil)
	if err == nil {
		t.Fatal("Expected error for missing URL, got nil")
	}

	// 测试空URL参数
	params["url"] = ""

	_, err = client.Execute(params, nil)
	if err == nil {
		t.Fatal("Expected error for empty URL, got nil")
	}
}

func TestHTTPClientToolWithBaseURL(t *testing.T) {
	// 创建模拟服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 检查路径
		if r.URL.Path != "/api/test" {
			t.Errorf("Expected path /api/test, got %s", r.URL.Path)
		}

		// 返回成功响应
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	// 创建带基础URL的HTTP客户端工具
	client := NewHTTPClientTool(&HTTPClientConfig{
		BaseURL: server.URL,
	})

	// 执行请求，使用相对路径
	params := map[string]interface{}{
		"url": "/api/test",
	}

	_, err := client.Execute(params, nil)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
}

func TestHTTPClientToolWithDefaultHeaders(t *testing.T) {
	// 创建模拟服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 检查默认请求头
		if r.Header.Get("X-Default-Header") != "default-value" {
			t.Errorf("Expected X-Default-Header=default-value, got X-Default-Header=%s", r.Header.Get("X-Default-Header"))
		}

		// 检查自定义User-Agent
		if r.Header.Get("User-Agent") != "CustomAgent/1.0" {
			t.Errorf("Expected User-Agent=CustomAgent/1.0, got User-Agent=%s", r.Header.Get("User-Agent"))
		}

		// 返回成功响应
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	// 创建带默认头的HTTP客户端工具
	client := NewHTTPClientTool(&HTTPClientConfig{
		UserAgent: "CustomAgent/1.0",
		Headers: map[string]string{
			"X-Default-Header": "default-value",
		},
	})

	// 执行请求
	params := map[string]interface{}{
		"url": server.URL,
	}

	_, err := client.Execute(params, nil)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
}
