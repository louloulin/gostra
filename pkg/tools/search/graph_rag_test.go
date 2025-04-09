package search_test

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/yourusername/gostra/pkg/actor"
	"github.com/yourusername/gostra/pkg/agent"
	"github.com/yourusername/gostra/pkg/memory"
	"github.com/yourusername/gostra/pkg/models/openai"
	"github.com/yourusername/gostra/pkg/tools/document"
	"github.com/yourusername/gostra/pkg/tools/search"
)

// TestGraphRAGWithActorSystem is an integration test that demonstrates how the
// GraphRAGTool works with the actor system and PostgreSQL vector storage.
func TestGraphRAGWithActorSystem(t *testing.T) {
	// Skip if not running integration tests
	if os.Getenv("INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TESTS=true to run")
	}

	// Required environment variables
	openaiKey := os.Getenv("OPENAI_API_KEY")
	if openaiKey == "" {
		t.Skip("OPENAI_API_KEY environment variable is required")
	}

	pgConnStr := os.Getenv("POSTGRES_CONNECTION_STRING")
	if pgConnStr == "" {
		t.Skip("POSTGRES_CONNECTION_STRING environment variable is required")
	}

	// Create context
	ctx := context.Background()

	// Initialize Actor system
	system := actor.NewActorSystem(&actor.Configuration{
		SystemName: "graph-rag-test",
	})

	// Create OpenAI provider
	openaiProvider, err := openai.NewOpenAIProvider(&openai.Options{
		APIKey: openaiKey,
		Model:  "gpt-3.5-turbo", // Use a smaller model for testing
	})
	if err != nil {
		t.Fatalf("Failed to create OpenAI provider: %v", err)
	}

	// Create embedding provider
	embeddingProvider := &search.OpenAIEmbeddingProvider{
		APIKey: openaiKey,
		Model:  "text-embedding-3-small",
	}

	// Create PostgreSQL vector storage
	pgVectorStore, err := memory.NewPostgresVectorStorage(memory.PostgresVectorOptions{
		ConnectionString: pgConnStr,
		TableName:        "graph_rag_test",
		VectorDimension:  1536, // Dimension for text-embedding-3-small
		BatchSize:        100,
	})
	if err != nil {
		t.Fatalf("Failed to create PostgreSQL vector storage: %v", err)
	}
	defer pgVectorStore.Close()

	// Clear any previous test data
	if err := pgVectorStore.Clear(ctx); err != nil {
		t.Logf("Warning: Failed to clear vector storage: %v", err)
	}

	// Create document chunker
	documentChunker := document.NewDocumentChunker(document.DocumentChunkerOptions{})

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
		GetRelatedness: func(text1, text2 string) (float32, error) {
			// Calculate relatedness using embeddings
			emb1, err := embeddingProvider.GetEmbedding(text1)
			if err != nil {
				return 0, err
			}
			emb2, err := embeddingProvider.GetEmbedding(text2)
			if err != nil {
				return 0, err
			}
			// Manual implementation of cosine similarity for testing
			return calculateCosineSimilarity(emb1, emb2), nil
		},
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

	// Store vector handler function
	storeVectorFunction := func(chunks []*document.DocumentChunk) error {
		// Extract text content for embeddings
		texts := make([]string, len(chunks))
		for i, chunk := range chunks {
			texts[i] = chunk.Content
		}

		// Get embeddings
		embeddings, err := embeddingProvider.GetEmbeddings(texts)
		if err != nil {
			return err
		}

		// Convert to vector storage format
		vectors := make([]memory.Vector, len(chunks))
		for i, chunk := range chunks {
			// Generate a unique ID if DocumentID is not set
			chunkID := fmt.Sprintf("%s-%d", chunk.DocumentID, i)
			vectors[i] = memory.Vector{
				ID:       chunkID,
				Values:   embeddings[i],
				Metadata: chunk.Metadata,
			}
		}

		// Store in PostgreSQL
		return pgVectorStore.Store(ctx, vectors)
	}

	// Create Graph RAG agent
	graphRAGAgent, err := agent.NewAgent(&agent.AgentConfig{
		Name:          "graph-rag-agent",
		ModelProvider: openaiProvider,
		Tools:         []agent.Tool{graphRAGTool, documentSearchTool},
		Instructions: `You are an assistant that uses knowledge graphs to answer questions.
Graph RAG builds a knowledge graph from document chunks and uses PageRank to find the most relevant information.
Please format your answers as follows:

1. DIRECT FACTS: List directly stated facts from the text relevant to the question (2-3 bullet points)
2. CONNECTIONS: List relationships discovered between different parts of the text (2-3 bullet points)
3. CONCLUSION: One sentence summary

Keep each section brief and focus on the most important points.`,
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	// Register agent
	system.RegisterAgent("graph-rag-agent", graphRAGAgent)

	// Start actor system
	system.Start()
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

	// Add sample document
	err = documentSearchTool.AddDocument(sampleDocument, "riverdale-case-study", map[string]interface{}{
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
	})
	if err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}

	// Process document
	doc, err := document.NewProcessedDocument()
	if err != nil {
		t.Fatalf("Failed to create document processor: %v", err)
	}

	err = doc.LoadFromString(sampleDocument)
	if err != nil {
		t.Fatalf("Failed to load document: %v", err)
	}

	// Chunk the document
	chunks, err := documentChunker.Chunk(doc, 500, 100)
	if err != nil {
		t.Fatalf("Failed to chunk document: %v", err)
	}

	// Add document ID and metadata to chunks
	for i := range chunks {
		chunks[i].DocumentID = "riverdale-case-study"
		chunks[i].Metadata = map[string]interface{}{
			"document_id": "riverdale-case-study",
			"chunk_index": i,
		}
	}

	// Store chunks in vector database
	err = storeVectorFunction(chunks)
	if err != nil {
		t.Fatalf("Failed to store chunks: %v", err)
	}

	// Test queries
	testQueries := []struct {
		name  string
		query string
	}{
		{
			name:  "Historical stages",
			query: "What are the main historical stages of Riverdale's development?",
		},
		{
			name:  "Railroad impact",
			query: "How did the railroad impact Riverdale's development? Find relationships through the knowledge graph.",
		},
		{
			name:  "Industrial decline response",
			query: "How did Riverdale respond to industrial decline? What strategies were used?",
		},
	}

	// Execute queries using Graph RAG
	for _, tc := range testQueries {
		t.Run(tc.name, func(t *testing.T) {
			// Create graph RAG query params
			params := map[string]interface{}{
				"query":     tc.query,
				"top_k":     5,
				"threshold": 0.6,
				"method":    "simple",
			}

			// Execute query through agent system
			response, err := system.SendAndReceive(ctx, "graph-rag-agent", "Execute", "graph_rag", params)
			if err != nil {
				t.Fatalf("Failed to execute query: %v", err)
			}

			// Format prompt with graph results
			prompt := formatGraphResults(tc.query, response)

			// Generate final answer through agent system
			finalResponse, err := system.SendAndReceive(ctx, "graph-rag-agent", "Generate", prompt, nil)
			if err != nil {
				t.Fatalf("Failed to generate answer: %v", err)
			}

			finalText, ok := finalResponse.(string)
			if !ok {
				t.Fatalf("Unexpected response type: %T", finalResponse)
			}

			// Log the answer (for visual inspection during test)
			t.Logf("Query: %s\nAnswer: %s\n", tc.query, finalText)
		})
	}
}

// calculateCosineSimilarity calculates the cosine similarity between two vectors
func calculateCosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float32
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (sqrt32(normA) * sqrt32(normB))
}

// sqrt32 calculates the square root of a float32
func sqrt32(x float32) float32 {
	return float32(float64(x))
}

// formatGraphResults formats the graph search results into a prompt for the LLM
func formatGraphResults(query string, result interface{}) string {
	// Extract graph results from the response
	graphResult, ok := result.(map[string]interface{})
	if !ok {
		return "Error: unexpected result format"
	}

	// Format prompt with results
	prompt := "Based on the following knowledge graph search results, please answer the question:\n\n"
	prompt += "Question: " + query + "\n\n"
	prompt += "Results:\n"

	// Process nodes
	if results, ok := graphResult["results"].([]interface{}); ok {
		for i, n := range results {
			if node, ok := n.(map[string]interface{}); ok {
				content, _ := node["content"].(string)
				score, _ := node["score"].(float64)
				prompt += "- Result " + strconv.Itoa(i+1) + " (Score: " + fmt.Sprintf("%.2f", score) + "): " + content + "\n\n"
			}
		}
	}

	prompt += "Please format your answer as instructed earlier."
	return prompt
}
