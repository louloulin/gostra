package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/yourusername/gostra/pkg/agent"
	"github.com/yourusername/gostra/pkg/memory"
	"github.com/yourusername/gostra/pkg/models"
	"github.com/yourusername/gostra/pkg/models/openai"
)

func main() {
	// 获取OpenAI API密钥
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("未设置OPENAI_API_KEY环境变量")
	}

	// 创建OpenAI模型提供者
	modelProvider, err := openai.NewOpenAIProvider(&openai.OpenAIOptions{
		APIKey:  apiKey,
		ModelID: "gpt-3.5-turbo",
	})
	if err != nil {
		log.Fatalf("创建OpenAI模型提供者失败: %v", err)
	}

	// 创建嵌入向量提供者
	embeddingProvider, err := memory.NewOpenAIEmbeddingProvider(memory.OpenAIEmbeddingOptions{
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
		SystemPrompt:   "你是一个具备上下文感知能力的AI助手。你的回答将根据上下文历史和相关信息进行调整。",
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

	// 添加一些上下文项目
	addContextItems(contextManager)

	// 查询Agent，测试上下文管理
	runAgentQueries(rootCtx, pid, contextManager)

	// 清理
	rootCtx.Stop(pid)
	time.Sleep(1 * time.Second) // 等待Actor停止
	fmt.Println("Agent已停止")
}

// 添加上下文项目
func addContextItems(ctx memory.ContextManager) {
	c := context.Background()

	// 添加一些示例文档
	documents := []struct {
		content  string
		priority int
		source   string
		itemType string
	}{
		{
			content:  "人工智能(AI)是计算机科学的一个分支，它致力于创建能够模拟人类智能的系统。现代AI技术包括机器学习、深度学习、自然语言处理等领域。",
			priority: 5,
			source:   "document",
			itemType: "knowledge",
		},
		{
			content:  "机器学习是人工智能的一个子集，它使用统计技术让计算机系统能够"学习"，而无需明确编程。常见的机器学习类型包括监督学习、无监督学习和强化学习。",
			priority: 4,
			source:   "document",
			itemType: "knowledge",
		},
		{
			content:  "深度学习是机器学习的一个子集，它使用神经网络结构进行学习。神经网络模拟人类大脑的结构，由多层神经元组成，能够处理复杂的数据模式。",
			priority: 3,
			source:   "document",
			itemType: "knowledge",
		},
	}

	// 添加文档到上下文
	for _, doc := range documents {
		item := &memory.ContextItem{
			Content:  doc.content,
			Type:     doc.itemType,
			Priority: doc.priority,
			Source:   doc.source,
		}
		if err := ctx.AddItem(c, item); err != nil {
			log.Printf("添加上下文项目失败: %v", err)
		}
	}

	// 添加用户偏好
	preferences := &memory.ContextItem{
		Content:  "用户偏好简洁、准确的回答，不需要太多技术细节。",
		Type:     "preference",
		Priority: 5,
		Source:   "user",
	}
	if err := ctx.AddItem(c, preferences); err != nil {
		log.Printf("添加用户偏好失败: %v", err)
	}

	fmt.Println("已添加上下文项目")
}

// 运行Agent查询
func runAgentQueries(rootCtx *actor.RootContext, pid *actor.PID, contextManager memory.ContextManager) {
	queries := []string{
		"什么是人工智能？",
		"机器学习和深度学习的关系是什么？",
		"解释一下神经网络的工作原理",
	}

	ctx := context.Background()

	for _, query := range queries {
		fmt.Printf("\n查询: %s\n", query)

		// 检索相关上下文
		contextRetrieval, err := contextManager.RetrieveContext(ctx, query, 2000, nil)
		if err != nil {
			log.Printf("检索上下文失败: %v", err)
			continue
		}

		// 创建消息，包含上下文
		messages := make([]models.Message, 0)

		// 添加系统消息，包含用户偏好
		var systemPrompt string
		for _, item := range contextRetrieval.Items {
			if item.Type == "preference" {
				systemPrompt += item.Content + "\n"
			}
		}
		if systemPrompt != "" {
			messages = append(messages, models.Message{
				Role:    "system",
				Content: systemPrompt,
			})
		}

		// 添加上下文知识
		contextContent := "以下是相关的上下文信息：\n\n"
		for _, item := range contextRetrieval.Items {
			if item.Type == "knowledge" {
				contextContent += "- " + item.Content + "\n"
			}
		}
		if contextContent != "以下是相关的上下文信息：\n\n" {
			messages = append(messages, models.Message{
				Role:    "user",
				Content: contextContent,
			})
			messages = append(messages, models.Message{
				Role:    "assistant",
				Content: "我理解了这些信息，请问您有什么问题？",
			})
		}

		// 添加用户查询
		messages = append(messages, models.Message{
			Role:    "user",
			Content: query,
		})

		// 发送请求给Agent
		generateMsg := &agent.AgentGenerateMessage{
			Messages: messages,
		}

		future := rootCtx.RequestFuture(pid, generateMsg, 30*time.Second)
		result, err := future.Result()
		if err != nil {
			log.Printf("获取Agent响应失败: %v", err)
			continue
		}

		// 处理响应
		if response, ok := result.(*agent.AgentGenerateResponse); ok {
			fmt.Printf("Agent响应: %s\n", response.Text)

			// 保存对话到上下文
			dialogItem := &memory.ContextItem{
				Content:  "用户: " + query + "\n助手: " + response.Text,
				Type:     "dialog",
				Priority: 4,
				Source:   "conversation",
				TTL:      30 * time.Minute, // 对话上下文30分钟后过期
			}
			if err := contextManager.AddItem(ctx, dialogItem); err != nil {
				log.Printf("保存对话到上下文失败: %v", err)
			}
		} else {
			fmt.Printf("未能解析Agent响应: %v\n", result)
		}
	}
} 