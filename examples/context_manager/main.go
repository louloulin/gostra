// Package main demonstrates using the context manager
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/louloulin/gostra/pkg/agent"
	"github.com/louloulin/gostra/pkg/memory"
	"github.com/louloulin/gostra/pkg/models/openai"
)

// Document represents a simple document
type Document struct {
	content  string
	priority int
	source   string
	itemType string
}

func main() {
	fmt.Println("Context Manager Example")

	// 获取OpenAI API密钥
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("未设置OPENAI_API_KEY环境变量")
	}

	// 创建OpenAI模型提供者
	modelProvider, err := openai.NewOpenAIProvider(&openai.OpenAIOptions{
		APIKey: apiKey,
		Model:  "gpt-3.5-turbo",
	})
	if err != nil {
		log.Fatalf("创建OpenAI模型提供者失败: %v", err)
	}

	// 创建嵌入向量提供者
	embeddingProvider, err := memory.NewOpenAIEmbeddingProvider(&memory.OpenAIEmbeddingOptions{
		APIKey: apiKey,
		Model:  "text-embedding-3-small",
	})
	if err != nil {
		log.Fatalf("创建嵌入向量提供者失败: %v", err)
	}

	// 创建向量存储
	vectorStore := memory.NewInMemoryVectorStore(embeddingProvider)

	// 创建上下文管理器
	contextManager, err := memory.NewAdvancedContextManager(&memory.AdvancedContextManagerOptions{
		VectorStore:       vectorStore,
		EmbeddingProvider: embeddingProvider,
		Strategy:          memory.StrategyHybrid,
		RelevanceWeight:   0.6,
		RecencyWeight:     0.3,
		PriorityWeight:    0.1,
	})
	if err != nil {
		log.Fatalf("创建上下文管理器失败: %v", err)
	}

	// 创建基础内存提供者
	memoryProvider := memory.NewInMemoryProvider()

	// 创建一个Actor系统
	actorSystem := actor.NewActorSystem()

	// 创建Agent
	agentOptions := &agent.ActorAgentOptions{
		ID:             "context-agent",
		Name:           "ContextAwareAgent",
		SystemPrompt:   "You are an AI assistant with context awareness. Your responses will be adjusted based on the context history and related information.",
		ModelProvider:  modelProvider,
		MemoryProvider: memoryProvider,
		ActorSystem:    actorSystem,
	}

	props, err := agent.NewActorAgent(agentOptions)
	if err != nil {
		log.Fatalf("创建Agent失败: %v", err)
	}

	rootCtx := actor.NewRootContext(actorSystem, nil)
	pid, err := rootCtx.SpawnNamed(props, "context-agent")
	if err != nil {
		log.Fatalf("启动Agent失败: %v", err)
	}

	fmt.Println("Agent已启动，PID:", pid.String())

	// Add context items
	ctx := context.Background()
	addContextItems(ctx, contextManager)

	// Query the context
	searchContext(ctx, contextManager, "artificial intelligence and neural networks")

	// 清理
	rootCtx.Stop(pid)
	time.Sleep(1 * time.Second) // 等待Actor停止
	fmt.Println("Agent已停止")
}

// Add sample items to the context
func addContextItems(ctx context.Context, contextManager memory.ContextManager) {
	// Sample documents
	documents := []Document{
		{
			content:  "Artificial Intelligence (AI) is a branch of computer science dedicated to creating systems capable of performing tasks that typically require human intelligence.",
			priority: 5,
			source:   "document",
			itemType: "knowledge",
		},
		{
			content:  "Machine Learning is a subset of AI that uses statistical techniques to enable computers to 'learn' without being explicitly programmed.",
			priority: 4,
			source:   "document",
			itemType: "knowledge",
		},
		{
			content:  "Deep Learning is a subset of Machine Learning that uses neural network structures for learning. Neural networks simulate the structure of the human brain.",
			priority: 3,
			source:   "document",
			itemType: "knowledge",
		},
	}

	fmt.Println("Adding context items...")

	// Add documents to context
	for i, doc := range documents {
		item := &memory.ContextItem{
			Content:  doc.content,
			Type:     doc.itemType,
			Priority: doc.priority,
			Source:   doc.source,
		}
		if err := contextManager.AddItem(ctx, item); err != nil {
			log.Printf("Failed to add context item %d: %v", i, err)
		} else {
			fmt.Printf("Added item %d: %s...\n", i, doc.content[:30])
		}
	}

	// Add user preferences
	preferences := &memory.ContextItem{
		Content:  "User prefers concise, accurate answers without excessive technical details.",
		Type:     "preference",
		Priority: 5,
		Source:   "user",
	}
	if err := contextManager.AddItem(ctx, preferences); err != nil {
		log.Printf("Failed to add user preferences: %v", err)
	} else {
		fmt.Println("Added user preferences")
	}
}

// Search the context with a query
func searchContext(ctx context.Context, contextManager memory.ContextManager, query string) {
	fmt.Printf("\nQuerying context with: %s\n", query)

	// Retrieve relevant context
	contextRetrieval, err := contextManager.RetrieveContext(ctx, query, 2000, nil)
	if err != nil {
		log.Printf("Failed to retrieve context: %v", err)
		return
	}

	// Display results
	fmt.Printf("Found %d relevant context items:\n", len(contextRetrieval.Items))
	for i, item := range contextRetrieval.Items {
		fmt.Printf("%d. [%s] Priority %d: %s\n",
			i+1,
			item.Type,
			item.Priority,
			item.Content)
	}
}
