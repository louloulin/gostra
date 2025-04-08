package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/yourusername/gostra/pkg/memory"
	"github.com/yourusername/gostra/pkg/tools"
	"github.com/yourusername/gostra/pkg/tools/common"
	"github.com/yourusername/gostra/pkg/tools/document"
	"github.com/yourusername/gostra/pkg/tools/search"
)

func main() {
	ctx := context.Background()

	// 创建OpenAI嵌入提供者
	openAIAPIKey := os.Getenv("OPENAI_API_KEY")
	if openAIAPIKey == "" {
		log.Fatal("未设置OPENAI_API_KEY环境变量")
	}

	embeddingProvider, err := memory.NewOpenAIEmbeddingProvider(&memory.OpenAIEmbeddingOptions{
		APIKey:  openAIAPIKey,
		Model:   "text-embedding-3-small",
		BaseURL: "https://api.openai.com/v1",
	})
	if err != nil {
		log.Fatalf("创建嵌入提供者失败: %v", err)
	}

	// 创建内存向量存储
	vectorStore := memory.NewInMemoryVectorStore(embeddingProvider)

	// 创建文档分块工具
	chunker := document.NewDocumentChunker(document.DocumentChunkerOptions{
		DefaultParams: document.ChunkParams{
			Strategy: document.StrategyFixed,
			Size:     1000,
			Overlap:  100,
		},
	})

	// 创建向量搜索工具
	vectorSearchTool := search.NewVectorSearchTool(search.VectorSearchOptions{
		EmbeddingProvider: embeddingProvider,
		Dimension:         1536,
		StoreText:         true,
	})

	// 创建文档搜索工具
	docSearchTool := search.NewDocumentSearchTool(search.DocumentSearchOptions{
		VectorSearchTool:  vectorSearchTool,
		DocumentChunker:   chunker,
		EmbeddingProvider: embeddingProvider,
		ChunkSize:         500,
		ChunkOverlap:      50,
		ChunkStrategy:     common.ChunkStrategyFixed,
	})

	// 示例文档内容
	sampleDocs := []struct {
		ID      string
		Content string
		Meta    map[string]interface{}
	}{
		{
			ID:      "doc1",
			Content: "Go是一种开源编程语言，可以轻松构建简单、可靠和高效的软件。Go由Google开发，于2009年首次发布。",
			Meta:    map[string]interface{}{"source": "go-intro", "type": "programming"},
		},
		{
			ID:      "doc2",
			Content: "Python是一种解释型、高级、通用的编程语言。Python的设计强调代码的可读性和简洁的语法，这使得程序员能够用更少的代码表达概念。",
			Meta:    map[string]interface{}{"source": "python-intro", "type": "programming"},
		},
		{
			ID:      "doc3",
			Content: "人工智能（AI）是计算机科学的一个分支，它试图理解智能的本质，并生产出一种新的能以人类智能相似方式做出反应的智能机器。",
			Meta:    map[string]interface{}{"source": "ai-intro", "type": "technology"},
		},
	}

	// 添加文档到搜索工具
	fmt.Println("正在添加文档到搜索索引...")
	for _, doc := range sampleDocs {
		err := docSearchTool.AddDocument(doc.Content, doc.ID, doc.Meta)
		if err != nil {
			log.Fatalf("添加文档失败: %v", err)
		}
	}

	// 执行搜索
	fmt.Println("\n开始搜索测试...")
	searchTests := []string{
		"什么是Go语言？",
		"Python的主要特点是什么？",
		"人工智能是如何工作的？",
	}

	for _, query := range searchTests {
		fmt.Printf("\n查询: %s\n", query)

		result, err := docSearchTool.Execute(map[string]interface{}{
			"query": query,
			"limit": float64(3),
		}, &tools.ExecuteOptions{
			Context: ctx,
		})
		if err != nil {
			log.Fatalf("搜索失败: %v", err)
		}

		// 处理结果
		if searchResults, ok := result.([]*search.VectorSearchResult); ok {
			fmt.Printf("找到 %d 个结果:\n", len(searchResults))
			for i, r := range searchResults {
				fmt.Printf("%d. 文档ID: %s, 相似度: %.4f\n", i+1, r.Chunk.DocumentID, r.Score)
				fmt.Printf("   内容: %s\n", r.Chunk.Content)
				fmt.Printf("   元数据: %v\n", r.Metadata)
			}
		} else {
			fmt.Println("搜索结果类型错误")
		}
	}

	// 清除文档
	fmt.Println("\n清除所有文档...")
	docSearchTool.Clear()
	fmt.Println("文档已清除")
}
