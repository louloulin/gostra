package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/louloulin/gostra/pkg/actor"
	"github.com/louloulin/gostra/pkg/agent"
	"github.com/louloulin/gostra/pkg/memory"
	"github.com/louloulin/gostra/pkg/models/openai"
	"github.com/louloulin/gostra/pkg/tools"
	"github.com/louloulin/gostra/pkg/tools/document"
	"github.com/louloulin/gostra/pkg/tools/search"
)

// ToolAdapter adapts a tools.Tool to agent.Tool
type ToolAdapter struct {
	tool tools.Tool
}

// GetID returns the tool ID
func (a *ToolAdapter) GetID() string {
	return a.tool.GetID()
}

// GetDescription returns the tool description
func (a *ToolAdapter) GetDescription() string {
	return a.tool.GetDescription()
}

// GetInputSchema returns the input schema of the tool
func (a *ToolAdapter) GetInputSchema() tools.Schema {
	return a.tool.GetInputSchema().(tools.Schema)
}

// Execute executes the tool
func (a *ToolAdapter) Execute(params map[string]interface{}, options *tools.ExecuteOptions) (interface{}, error) {
	return a.tool.Execute(params, options)
}

func main() {
	// Get environment variables
	openaiKey := os.Getenv("OPENAI_API_KEY")
	if openaiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	pgConnStr := os.Getenv("POSTGRES_CONNECTION_STRING")
	if pgConnStr == "" {
		log.Fatal("POSTGRES_CONNECTION_STRING environment variable is required")
	}

	// Create context
	ctx := context.Background()

	// Initialize Actor system
	system := actor.NewActorSystem(&actor.Configuration{
		Agents: make(map[string]interface{}),
		Tools:  make(map[string]interface{}),
	})

	// Create OpenAI provider
	openaiProvider, err := openai.NewOpenAIProvider(&openai.Options{
		APIKey: openaiKey,
		Model:  "gpt-4", // Use GPT-4 for better reasoning
	})
	if err != nil {
		log.Fatalf("Failed to create OpenAI provider: %v", err)
	}

	// Create in-memory provider for agent
	memoryProvider := memory.NewInMemoryProvider()

	// Create embedding provider
	embeddingProvider, err := search.NewOpenAIEmbeddingProvider(search.OpenAIEmbeddingOptions{
		APIKey: openaiKey,
		Model:  search.ModelTextEmbedding3Small,
	})
	if err != nil {
		log.Fatalf("Failed to create embedding provider: %v", err)
	}

	// Create PostgreSQL vector storage
	pgVectorStore, err := memory.NewPostgresVectorStorage(memory.PostgresVectorOptions{
		ConnectionString: pgConnStr,
		TableName:        "graph_rag_example",
		VectorDimension:  1536, // Dimension for text-embedding-3-small
		BatchSize:        100,
	})
	if err != nil {
		log.Fatalf("Failed to create PostgreSQL vector storage: %v", err)
	}
	defer pgVectorStore.Close()

	// Clear any previous data
	if err := pgVectorStore.Clear(ctx); err != nil {
		log.Printf("Warning: Failed to clear vector storage: %v", err)
	}

	// Create document chunker with options
	chunkerOptions := document.DocumentChunkerOptions{
		DefaultParams: document.ChunkParams{
			Strategy:  document.StrategyRecursive,
			Size:      500,
			Overlap:   100,
			Separator: "\n",
		},
	}
	documentChunker := document.NewDocumentChunker(chunkerOptions)

	// Create vector search tool
	vectorSearchTool := search.NewVectorSearchTool(search.VectorSearchOptions{
		EmbeddingProvider: embeddingProvider,
		Dimension:         1536,
		DistanceMetric:    "cosine",
	})

	// Create Graph RAG tool
	graphRAGTool := search.NewGraphRAGTool(search.GraphRAGOptions{
		EmbeddingProvider: embeddingProvider,
		VectorSearchTool:  vectorSearchTool,
		Threshold:         0.7,
		Dimension:         1536,
		MaxNodes:          15,
	})

	// Create document search tool
	documentSearchTool := search.NewDocumentSearchTool(search.DocumentSearchOptions{
		VectorSearchTool:  vectorSearchTool,
		DocumentChunker:   documentChunker,
		EmbeddingProvider: embeddingProvider,
		ChunkSize:         500,
		ChunkOverlap:      100,
		ChunkStrategy:     document.StrategyRecursive,
	})

	// Adapt tools for agent
	graphRAGToolAdapter := &ToolAdapter{tool: graphRAGTool}
	documentSearchToolAdapter := &ToolAdapter{tool: documentSearchTool}

	// Create Graph RAG agent
	graphRAGAgent, err := agent.NewAgent(&agent.Options{
		Name:           "graph-rag-agent",
		ModelProvider:  openaiProvider,
		MemoryProvider: memoryProvider,
		Tools:          []tools.Tool{
			// Empty tools list - we'll register them manually
		},
		SystemPrompt: `You are an assistant that uses knowledge graphs to answer questions.
Graph RAG builds a knowledge graph from document chunks and uses PageRank to find the most relevant information.
Please format your answers as follows:

1. DIRECT FACTS: List directly stated facts from the text relevant to the question (2-3 bullet points)
2. CONNECTIONS: List relationships discovered between different parts of the text (2-3 bullet points)
3. CONCLUSION: One sentence summary

Keep each section brief and focus on the most important points.`,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Register tools with agent
	if err := graphRAGAgent.RegisterTool(graphRAGToolAdapter); err != nil {
		log.Fatalf("Failed to register graphRAGTool: %v", err)
	}
	if err := graphRAGAgent.RegisterTool(documentSearchToolAdapter); err != nil {
		log.Fatalf("Failed to register documentSearchTool: %v", err)
	}

	// Register agent with actor system
	_, err = system.RegisterAgent("graph-rag-agent", graphRAGAgent)
	if err != nil {
		log.Fatalf("Failed to register agent with actor system: %v", err)
	}

	// Start actor system
	if err := system.Start(); err != nil {
		log.Fatalf("Failed to start actor system: %v", err)
	}
	defer system.Stop()

	// Sample document content
	sampleDocument := `# Urban Development: A Case Study

## 1. Early Settlement (1800-1850)
Riverdale was founded by European immigrants in 1823. The location was chosen for its proximity to a river, providing transportation and irrigation. The initial settlers were primarily farmers and craftsmen who established a self-sufficient community. By 1840, the settlement had grown to about 300 people.

## 2. Industrialization (1850-1900)
In 1853, the first water mill was established in Riverdale, marking the beginning of industrialization. This development attracted more workers and merchants to the area. The arrival of the railroad in 1870 greatly boosted the local economy. By the end of the 19th century, Riverdale had become an important industrial center in the region, with a population growth to approximately 3,000 people.

Key developments during this period:
- 1853: First water mill established
- 1862: Riverdale Bank founded, providing capital for local businesses
- 1870: Railroad arrives, connecting Riverdale to major cities
- 1878: First public school built
- 1885: Coal mine begins operation, providing energy for industry
- 1891: Population surpasses 2,500

## 3. Industrial Boom (1900-1950)
The first half of the 20th century was a period of industrial prosperity for Riverdale. Steel mills, textile factories, and machinery plants were established during this time, providing employment for thousands. During both World Wars, local industries supplied large amounts of war materials, further boosting economic development.

Important changes brought by industrial prosperity:
- Urban infrastructure: New roads, electrical systems, and water supply systems were built
- Housing development: Large residential areas were constructed for workers' families
- Public facilities: New hospitals, libraries, and recreational facilities were built
- Population diversity: Immigrants from different countries made the city's population more diverse
- Social issues: Industrial pollution and labor conditions began to raise concerns

By 1950, Riverdale's population had reached approximately 25,000, making it a significant city in the region.

## 4. Economic Transition (1950-2000)
Beginning in the 1950s, changes in the global industrial landscape gradually affected Riverdale. Traditional industries began to decline, and many factories closed or relocated to areas with lower labor costs. The main challenges and response strategies during this period included:

### Challenges
- Industrial decline: Major factories closed, leading to rising unemployment
- Population loss: Many young people left to seek better opportunities elsewhere
- Aging infrastructure: Infrastructure built during the industrial era needed updating
- Environmental issues: Legacy pollution from industry needed addressing

### Response Strategies
- Economic diversification: Attracting new businesses, especially in service sectors and light industry
- Education investment: Expanding community colleges, providing retraining programs
- Urban renewal: Redeveloping abandoned industrial areas
- Tourism development: Utilizing industrial heritage for cultural tourism

By 2000, Riverdale had partially achieved economic transition, with a stabilized population of about 20,000.`

	// Add document to search tools
	fmt.Println("Adding document to search tools...")
	executeOpts := &tools.ExecuteOptions{}
	addDocParams := map[string]interface{}{
		"content":     sampleDocument,
		"document_id": "riverdale-case-study",
		"metadata": map[string]interface{}{
			"title":    "Urban Development: A Case Study",
			"type":     "case-study",
			"period":   "1800-2000",
			"theme":    "urban development",
			"keywords": []string{"urban", "industrial", "economic transition"},
			"time_periods": map[string]interface{}{
				"early_settlement": map[string]interface{}{
					"start_year": 1800,
					"end_year":   1850,
					"population": 300,
				},
				"industrialization": map[string]interface{}{
					"start_year": 1850,
					"end_year":   1900,
					"population": 3000,
				},
				"industrial_boom": map[string]interface{}{
					"start_year": 1900,
					"end_year":   1950,
					"population": 25000,
				},
				"transition": map[string]interface{}{
					"start_year": 1950,
					"end_year":   2000,
					"population": 20000,
				},
			},
		},
	}

	_, err = documentSearchTool.Execute(addDocParams, executeOpts)
	if err != nil {
		log.Fatalf("Failed to add document: %v", err)
	}

	// Wait for document to be processed and indexed
	fmt.Println("Waiting for document indexing...")
	time.Sleep(2 * time.Second)

	// Demo queries
	demoQueries := []string{
		"What are the main historical stages of Riverdale's development?",
		"How did the railroad impact Riverdale's development? Find relationships through the knowledge graph.",
		"How did Riverdale respond to industrial decline? What strategies were used?",
	}

	fmt.Println("\n===== Graph RAG Demo =====\n")

	// Create a thread ID for this session
	threadID := "graph-rag-demo-" + time.Now().Format("20060102150405")

	// Execute queries using Graph RAG
	for _, query := range demoQueries {
		fmt.Printf("===== Query: %s =====\n\n", query)

		// Set up run options
		runOpts := &agent.RunOptions{
			ThreadID:       threadID,
			Input:          query,
			AvailableTools: []tools.Tool{graphRAGToolAdapter, documentSearchToolAdapter},
		}

		// Execute agent run
		response, err := graphRAGAgent.Run(ctx, runOpts)
		if err != nil {
			log.Printf("Error executing query: %v", err)
			continue
		}

		// Print the response
		fmt.Printf("%s\n\n", response)
		fmt.Println(strings.Repeat("-", 80))

		// Add a small delay between queries
		time.Sleep(1 * time.Second)
	}
}
