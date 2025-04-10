package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/louloulin/gostra/pkg/tools"
	"github.com/louloulin/gostra/pkg/tools/document"
)

func main() {
	// 创建文档解析器工具
	parserOptions := document.ParserOptions{
		SupportedFormats: []string{"text/plain", "application/pdf", "text/html"},
	}
	documentParser := document.NewDocumentParser(parserOptions)

	// 打印工具信息
	fmt.Printf("工具ID: %s\n", documentParser.GetID())
	fmt.Printf("描述: %s\n", documentParser.GetDescription())
	fmt.Println()

	// 创建临时测试文件
	tempFile := createTempTextFile()
	defer os.Remove(tempFile)

	fmt.Printf("创建测试文件: %s\n", tempFile)
	fmt.Println()

	// 解析本地文件
	fmt.Println("===== 从本地文件解析文档 =====")
	parseLocalDocument(documentParser, tempFile)

	// 解析URL文件
	fmt.Println("\n===== 从URL解析文档 =====")
	fmt.Println("注: 此示例仅演示接口用法，实际运行时需要可访问的URL")
	parseUrlDocument(documentParser, "https://example.com/sample.txt")

	// 显示错误处理
	fmt.Println("\n===== 错误处理示例 =====")
	handleErrors(documentParser)
}

// 创建临时文本文件进行测试
func createTempTextFile() string {
	content := `# 示例文档

这是一个用于测试Gostra文档解析工具的简单文本文件。

## 特点

1. 纯文本格式
2. 包含Markdown语法
3. 用于演示文档解析功能

感谢使用Gostra框架！`

	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, "gostra_example_doc.txt")

	err := os.WriteFile(tempFile, []byte(content), 0644)
	if err != nil {
		fmt.Printf("创建临时文件失败: %v\n", err)
		os.Exit(1)
	}

	return tempFile
}

// 解析本地文档文件
func parseLocalDocument(parser tools.Tool, filePath string) {
	params := map[string]interface{}{
		"file_path":        filePath,
		"extract_metadata": true,
	}

	result, err := parser.Execute(params, nil)
	if err != nil {
		fmt.Printf("解析文档失败: %v\n", err)
		return
	}

	// 显示文档信息
	doc, ok := result.(*document.Document)
	if !ok {
		fmt.Println("结果类型转换失败")
		return
	}

	fmt.Printf("文件名: %s\n", doc.Filename)
	fmt.Printf("内容类型: %s\n", doc.ContentType)
	fmt.Printf("页数: %d\n", doc.PageCount)
	fmt.Println("元数据:", doc.Metadata)
	fmt.Println("内容预览 (前100个字符):")
	if len(doc.Content) > 100 {
		fmt.Println(doc.Content[:100] + "...")
	} else {
		fmt.Println(doc.Content)
	}
}

// 从URL解析文档
func parseUrlDocument(parser tools.Tool, url string) {
	params := map[string]interface{}{
		"url":              url,
		"extract_metadata": true,
	}

	result, err := parser.Execute(params, nil)
	if err != nil {
		fmt.Printf("解析URL文档失败: %v\n", err)
		return
	}

	// 显示文档信息
	doc, ok := result.(*document.Document)
	if !ok {
		fmt.Println("结果类型转换失败")
		return
	}

	fmt.Printf("文件名: %s\n", doc.Filename)
	fmt.Printf("内容类型: %s\n", doc.ContentType)
	fmt.Printf("页数: %d\n", doc.PageCount)
	fmt.Println("元数据:", doc.Metadata)
	fmt.Println("内容预览 (前100个字符):")
	if len(doc.Content) > 100 {
		fmt.Println(doc.Content[:100] + "...")
	} else {
		fmt.Println(doc.Content)
	}
}

// 错误处理示例
func handleErrors(parser tools.Tool) {
	// 示例1: 缺少必要参数
	fmt.Println("1. 缺少必要参数:")
	params1 := map[string]interface{}{}
	_, err := parser.Execute(params1, nil)
	fmt.Printf("   预期错误: %v\n", err)

	// 示例2: 文件不存在
	fmt.Println("\n2. 文件不存在:")
	params2 := map[string]interface{}{
		"file_path": "/path/to/nonexistent/file.txt",
	}
	_, err = parser.Execute(params2, nil)
	fmt.Printf("   预期错误: %v\n", err)

	// 示例3: 不支持的URL
	fmt.Println("\n3. 不支持的URL:")
	params3 := map[string]interface{}{
		"url": "ftp://example.com/file.txt", // 不支持的协议
	}
	_, err = parser.Execute(params3, nil)
	fmt.Printf("   预期错误: %v\n", err)
}
