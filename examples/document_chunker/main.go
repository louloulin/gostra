package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yourusername/gostra/pkg/tools"
	"github.com/yourusername/gostra/pkg/tools/document"
)

func main() {
	// 创建文档分块工具
	chunkerOptions := document.DocumentChunkerOptions{
		DefaultParams: document.ChunkParams{
			Strategy:  document.StrategyRecursive,
			Size:      200,
			Overlap:   50,
			Separator: "\n",
		},
	}
	documentChunker := document.NewDocumentChunker(chunkerOptions)

	// 打印工具信息
	fmt.Printf("工具ID: %s\n", documentChunker.GetID())
	fmt.Printf("描述: %s\n", documentChunker.GetDescription())
	fmt.Println()

	// 创建测试文件
	tempFile := createTempTextFile()
	defer os.Remove(tempFile)

	// 读取文件内容
	content, err := os.ReadFile(tempFile)
	if err != nil {
		fmt.Printf("读取文件失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("文件内容长度: %d 字符\n", len(content))
	fmt.Println()

	// 演示不同分块策略
	fmt.Println("===== 1. 递归分块策略 =====")
	demonstrateStrategy(documentChunker, string(content), document.StrategyRecursive, 200, 50, "\n")

	fmt.Println("\n===== 2. 固定大小分块策略 =====")
	demonstrateStrategy(documentChunker, string(content), document.StrategyFixed, 200, 50, "")

	fmt.Println("\n===== 3. 句子分块策略 =====")
	demonstrateStrategy(documentChunker, string(content), document.StrategySentence, 200, 30, "")

	fmt.Println("\n===== 4. 段落分块策略 =====")
	demonstrateStrategy(documentChunker, string(content), document.StrategyParagraph, 300, 50, "")

	// 展示自定义参数
	fmt.Println("\n===== 5. 自定义参数示例 =====")
	demonstrateCustomParams(documentChunker, string(content))
}

// 创建临时文本文件进行测试
func createTempTextFile() string {
	content := `# Gostra文档分块示例

## 文档分块的重要性

在处理大型文档时，将文档分解成更小的片段对于许多AI应用来说至关重要。以下是一些理由：

1. 上下文窗口限制：大多数语言模型有输入标记限制，需要将大文档分解成可管理的块。
2. 精确检索：较小的文本块使得向量搜索和语义匹配更加精确。
3. 并行处理：可以并行处理较小的文本块，提高效率。
4. 降低资源消耗：处理小块文本比整个大文档需要更少的内存和计算资源。

## 常见分块策略

文档可以使用多种策略进行分块：

### 固定大小分块

最简单的方法是将文档分成固定大小的块，不考虑语义边界。虽然实现简单，但可能会在不适当的位置切分句子或段落。

### 句子分块

根据句子边界（如句号、问号、感叹号等）划分文档。这保留了句子级别的语义完整性。

### 段落分块

按段落边界（通常是空行或缩进）分割文档。对于保留段落级语义结构很有用。

### 递归分块

结合多种策略的高级方法。首先尝试按大的语义边界（如章节或段落）分割，如果结果块仍然太大，则递归地应用更精细的分割（如句子或固定大小）。

## 分块中的重叠

块之间的重叠是关键概念。适当的重叠可以确保：

1. 上下文连续性：跨越块边界的信息不会丢失
2. 提高检索质量：搜索查询的关键词可能跨越块边界
3. 避免信息孤岛：防止相关信息被分到不同的无关块中

推荐的重叠大小通常是块大小的10-20%。

## 总结

高效的文档分块策略对于构建高质量的RAG（检索增强生成）系统至关重要。选择正确的分块方法和参数应该基于具体应用场景、文档类型和检索需求进行优化。`

	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, "gostra_chunker_example.md")

	err := os.WriteFile(tempFile, []byte(content), 0644)
	if err != nil {
		fmt.Printf("创建临时文件失败: %v\n", err)
		os.Exit(1)
	}

	return tempFile
}

// 演示特定分块策略
func demonstrateStrategy(chunker tools.Tool, content string, strategy document.ChunkStrategy, size, overlap int, separator string) {
	params := map[string]interface{}{
		"content":   content,
		"strategy":  string(strategy),
		"size":      float64(size),
		"overlap":   float64(overlap),
		"separator": separator,
	}

	result, err := chunker.Execute(params, nil)
	if err != nil {
		fmt.Printf("分块失败: %v\n", err)
		return
	}

	chunks, ok := result.([]*document.DocumentChunk)
	if !ok {
		fmt.Println("结果类型转换失败")
		return
	}

	fmt.Printf("使用策略：%s\n", strategy)
	fmt.Printf("块大小：%d，重叠：%d\n", size, overlap)
	fmt.Printf("生成块数量：%d\n", len(chunks))

	// 显示前两个块和最后一个块的信息
	if len(chunks) > 0 {
		fmt.Println("\n== 第一个块 ==")
		printChunkInfo(chunks[0])
	}

	if len(chunks) > 1 {
		fmt.Println("\n== 第二个块 ==")
		printChunkInfo(chunks[1])

		// 显示重叠情况
		showOverlap(chunks[0], chunks[1])
	}

	if len(chunks) > 2 {
		fmt.Println("\n== 最后一个块 ==")
		printChunkInfo(chunks[len(chunks)-1])
	}
}

// 显示自定义参数示例
func demonstrateCustomParams(chunker tools.Tool, content string) {
	// 只处理文档中的特定部分
	lines := strings.Split(content, "\n")
	var selectedLines []string
	inSection := false

	// 提取"常见分块策略"部分
	for _, line := range lines {
		if strings.Contains(line, "## 常见分块策略") {
			inSection = true
		} else if inSection && strings.HasPrefix(line, "## ") {
			inSection = false
		}

		if inSection {
			selectedLines = append(selectedLines, line)
		}
	}

	selectedContent := strings.Join(selectedLines, "\n")

	// 使用句子策略，较小的块大小
	params := map[string]interface{}{
		"content":  selectedContent,
		"strategy": "sentence",
		"size":     100.0,
		"overlap":  20.0,
	}

	result, err := chunker.Execute(params, nil)
	if err != nil {
		fmt.Printf("分块失败: %v\n", err)
		return
	}

	chunks, ok := result.([]*document.DocumentChunk)
	if !ok {
		fmt.Println("结果类型转换失败")
		return
	}

	fmt.Printf("选择性内容分块 - 句子策略\n")
	fmt.Printf("块大小：100，重叠：20\n")
	fmt.Printf("内容部分：\"常见分块策略\"\n")
	fmt.Printf("选择内容长度：%d 字符\n", len(selectedContent))
	fmt.Printf("生成块数量：%d\n", len(chunks))

	for i, chunk := range chunks {
		fmt.Printf("\n== 块 %d ==\n", i+1)
		printShortChunkInfo(chunk)
	}
}

// 打印块信息
func printChunkInfo(chunk *document.DocumentChunk) {
	fmt.Printf("位置: %d\n", chunk.Position)
	fmt.Printf("长度: %d 字符\n", len(chunk.Content))

	content := chunk.Content
	if len(content) > 100 {
		// 只显示前后50个字符
		content = content[:50] + " ... " + content[len(content)-50:]
	}
	fmt.Printf("内容预览: %s\n", content)
}

// 打印简短的块信息
func printShortChunkInfo(chunk *document.DocumentChunk) {
	content := chunk.Content
	if len(content) > 60 {
		content = content[:60] + "..."
	}
	fmt.Printf("位置: %d, 长度: %d, 内容: %s\n",
		chunk.Position, len(chunk.Content), content)
}

// 显示两个块之间的重叠
func showOverlap(chunk1, chunk2 *document.DocumentChunk) {
	if chunk2.Position <= chunk1.Position+len(chunk1.Content) {
		overlapStart := chunk2.Position
		overlapEnd := chunk1.Position + len(chunk1.Content)
		overlapSize := overlapEnd - overlapStart

		if overlapSize > 0 {
			overlapText := chunk1.Content[len(chunk1.Content)-overlapSize:]
			fmt.Printf("\n== 块间重叠 (%d 字符) ==\n", overlapSize)
			if len(overlapText) > 100 {
				fmt.Println(overlapText[:50] + " ... " + overlapText[len(overlapText)-50:])
			} else {
				fmt.Println(overlapText)
			}
		}
	}
}
