package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/yourusername/gostra/pkg/tools"
	"github.com/yourusername/gostra/pkg/tools/network"
)

func main() {
	// 创建HTTP客户端工具
	config := &network.HTTPClientConfig{
		BaseURL: "https://jsonplaceholder.typicode.com",
		Headers: map[string]string{
			"Accept": "application/json",
		},
		UserAgent: "Gostra-Example/1.0",
	}
	httpTool := network.NewHTTPClientTool(config)

	// 打印工具的基本信息
	fmt.Printf("Tool ID: %s\n", httpTool.GetID())
	fmt.Printf("Description: %s\n", httpTool.GetDescription())

	// 创建上下文
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 创建执行选项
	options := &tools.ExecuteOptions{
		Context: ctx,
	}

	fmt.Println("\n==== GET 请求示例 ====")
	getExample(httpTool, options)

	fmt.Println("\n==== POST 请求示例 ====")
	postExample(httpTool, options)

	fmt.Println("\n==== PUT 请求示例 ====")
	putExample(httpTool, options)

	fmt.Println("\n==== DELETE 请求示例 ====")
	deleteExample(httpTool, options)

	fmt.Println("\n==== 带查询参数的请求示例 ====")
	queryParamsExample(httpTool, options)

	fmt.Println("\n==== 自定义请求头示例 ====")
	customHeadersExample(httpTool, options)

	fmt.Println("\n==== 错误处理示例 ====")
	errorHandlingExample(httpTool, options)
}

// GET 请求示例
func getExample(httpTool tools.Tool, options *tools.ExecuteOptions) {
	params := map[string]interface{}{
		"url": "/posts/1",
	}

	response, err := httpTool.Execute(params, options)
	if err != nil {
		fmt.Printf("GET请求失败: %v\n", err)
		return
	}

	// 处理响应
	httpResp, ok := response.(network.HTTPResponse)
	if !ok {
		fmt.Println("响应类型转换失败")
		return
	}

	fmt.Printf("状态码: %d\n", httpResp.StatusCode)
	prettyPrintJSON(httpResp.Body)
}

// POST 请求示例
func postExample(httpTool tools.Tool, options *tools.ExecuteOptions) {
	params := map[string]interface{}{
		"url":    "/posts",
		"method": "POST",
		"body": map[string]interface{}{
			"title":  "Gostra HTTP工具测试",
			"body":   "这是使用Gostra HTTP工具发送的POST请求",
			"userId": 1,
		},
	}

	response, err := httpTool.Execute(params, options)
	if err != nil {
		fmt.Printf("POST请求失败: %v\n", err)
		return
	}

	// 处理响应
	httpResp, ok := response.(network.HTTPResponse)
	if !ok {
		fmt.Println("响应类型转换失败")
		return
	}

	fmt.Printf("状态码: %d\n", httpResp.StatusCode)
	prettyPrintJSON(httpResp.Body)
}

// PUT 请求示例
func putExample(httpTool tools.Tool, options *tools.ExecuteOptions) {
	params := map[string]interface{}{
		"url":    "/posts/1",
		"method": "PUT",
		"body": map[string]interface{}{
			"id":     1,
			"title":  "Gostra HTTP工具更新测试",
			"body":   "这是使用Gostra HTTP工具发送的PUT请求",
			"userId": 1,
		},
	}

	response, err := httpTool.Execute(params, options)
	if err != nil {
		fmt.Printf("PUT请求失败: %v\n", err)
		return
	}

	// 处理响应
	httpResp, ok := response.(network.HTTPResponse)
	if !ok {
		fmt.Println("响应类型转换失败")
		return
	}

	fmt.Printf("状态码: %d\n", httpResp.StatusCode)
	prettyPrintJSON(httpResp.Body)
}

// DELETE 请求示例
func deleteExample(httpTool tools.Tool, options *tools.ExecuteOptions) {
	params := map[string]interface{}{
		"url":    "/posts/1",
		"method": "DELETE",
	}

	response, err := httpTool.Execute(params, options)
	if err != nil {
		fmt.Printf("DELETE请求失败: %v\n", err)
		return
	}

	// 处理响应
	httpResp, ok := response.(network.HTTPResponse)
	if !ok {
		fmt.Println("响应类型转换失败")
		return
	}

	fmt.Printf("状态码: %d\n", httpResp.StatusCode)
	if httpResp.Body != nil {
		prettyPrintJSON(httpResp.Body)
	} else {
		fmt.Println("响应体为空")
	}
}

// 带查询参数的请求示例
func queryParamsExample(httpTool tools.Tool, options *tools.ExecuteOptions) {
	params := map[string]interface{}{
		"url": "/comments",
		"params": map[string]interface{}{
			"postId": 1,
			"_limit": 2,
		},
	}

	response, err := httpTool.Execute(params, options)
	if err != nil {
		fmt.Printf("查询参数请求失败: %v\n", err)
		return
	}

	// 处理响应
	httpResp, ok := response.(network.HTTPResponse)
	if !ok {
		fmt.Println("响应类型转换失败")
		return
	}

	fmt.Printf("状态码: %d\n", httpResp.StatusCode)
	prettyPrintJSON(httpResp.Body)
}

// 自定义请求头示例
func customHeadersExample(httpTool tools.Tool, options *tools.ExecuteOptions) {
	params := map[string]interface{}{
		"url": "/posts/1",
		"headers": map[string]interface{}{
			"X-Custom-Header": "custom-value",
			"Cache-Control":   "no-cache",
		},
	}

	response, err := httpTool.Execute(params, options)
	if err != nil {
		fmt.Printf("自定义请求头请求失败: %v\n", err)
		return
	}

	// 处理响应
	httpResp, ok := response.(network.HTTPResponse)
	if !ok {
		fmt.Println("响应类型转换失败")
		return
	}

	fmt.Printf("状态码: %d\n", httpResp.StatusCode)
	prettyPrintJSON(httpResp.Body)
}

// 错误处理示例
func errorHandlingExample(httpTool tools.Tool, options *tools.ExecuteOptions) {
	// 设置一个很短的超时
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	timeoutOptions := &tools.ExecuteOptions{
		Context: ctx,
	}

	params := map[string]interface{}{
		"url": "/posts/1",
	}

	_, err := httpTool.Execute(params, timeoutOptions)
	if err != nil {
		fmt.Printf("预期的错误: %v\n", err)
	} else {
		fmt.Println("期望出现超时错误，但请求成功了")
	}

	// 无效的URL
	invalidParams := map[string]interface{}{
		"url": "/nonexistent",
	}

	response, err := httpTool.Execute(invalidParams, options)
	if err != nil {
		fmt.Printf("无效URL错误: %v\n", err)
		return
	}

	// 处理404响应
	httpResp, ok := response.(network.HTTPResponse)
	if !ok {
		fmt.Println("响应类型转换失败")
		return
	}

	fmt.Printf("无效URL状态码: %d\n", httpResp.StatusCode)
}

// 工具函数：格式化打印JSON
func prettyPrintJSON(data interface{}) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Printf("JSON格式化失败: %v\n", err)
		return
	}
	fmt.Println(string(jsonBytes))
}
