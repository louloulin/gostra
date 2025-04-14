package main

import (
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/louloulin/gostra/pkg/memory"
	"github.com/louloulin/gostra/pkg/models"
	"github.com/louloulin/gostra/pkg/tools"
	"github.com/louloulin/gostra/pkg/tools/document"
	"github.com/louloulin/gostra/pkg/tools/search"
)

const (
	// Edge type constants
	EdgeSimilarity    = "similarity"
	EdgeSemanticLink  = "semantic_link"
	EdgeReference     = "reference"
	EdgeChronological = "chronological"

	// Similarity threshold for creating graph connections
	DefaultSimilarityThreshold = 0.75

	// Default chunk size for documents
	DefaultChunkSize    = 1000
	DefaultChunkOverlap = 200
)

// GraphRAGTool is a tool that implements the Graph RAG (Retrieval Augmented Generation) functionality
type GraphRAGTool struct {
	// Vector store for storing and retrieving embeddings
	vectorStore memory.VectorStorage

	// Knowledge graph for storing and querying nodes and relationships
	graph *KnowledgeGraph

	// Embedding provider for generating vector embeddings
	embeddingProvider search.EmbeddingProvider

	// Document processor for chunking and processing documents
	documentProcessor *document.ProcessedDocument

	// Configuration options
	similarityThreshold float64
	chunkSize           int
	chunkOverlap        int

	// Lock for concurrent access to the graph
	mu sync.RWMutex
}

// GraphRAGOptions contains configuration options for the GraphRAGTool
type GraphRAGOptions struct {
	VectorStore         memory.VectorStorage
	EmbeddingProvider   search.EmbeddingProvider
	SimilarityThreshold float64
	ChunkSize           int
	ChunkOverlap        int
}

// NewGraphRAGTool creates a new Graph RAG tool with the provided options
func NewGraphRAGTool(options GraphRAGOptions) (*GraphRAGTool, error) {
	if options.VectorStore == nil {
		return nil, fmt.Errorf("vector store is required")
	}

	if options.EmbeddingProvider == nil {
		return nil, fmt.Errorf("embedding provider is required")
	}

	if options.SimilarityThreshold <= 0 {
		options.SimilarityThreshold = DefaultSimilarityThreshold
	}

	if options.ChunkSize <= 0 {
		options.ChunkSize = DefaultChunkSize
	}

	if options.ChunkOverlap <= 0 {
		options.ChunkOverlap = DefaultChunkOverlap
	}

	return &GraphRAGTool{
		vectorStore:         options.VectorStore,
		graph:               NewKnowledgeGraph(),
		embeddingProvider:   options.EmbeddingProvider,
		documentProcessor:   &document.ProcessedDocument{},
		similarityThreshold: options.SimilarityThreshold,
		chunkSize:           options.ChunkSize,
		chunkOverlap:        options.ChunkOverlap,
		mu:                  sync.RWMutex{},
	}, nil
}

// AddDocument adds a document to the graph RAG system
func (g *GraphRAGTool) AddDocument(documentContent string, documentID string, metadata map[string]interface{}) error {
	// Create a simple chunk from the document content
	chunk := document.Chunk{
		Content:  documentContent,
		ID:       documentID,
		Metadata: metadata,
	}

	// Add the chunk to the graph
	if err := g.addChunkToGraph(chunk, 0, documentID); err != nil {
		return fmt.Errorf("failed to add chunk to graph: %v", err)
	}

	// For simplicity, we're not chunking or creating relationships between chunks
	return nil
}

// addChunkToGraph adds a single document chunk to the graph
func (g *GraphRAGTool) addChunkToGraph(chunk document.Chunk, chunkIndex int, documentPath string) error {
	// Generate embedding for the chunk
	embedding, err := g.embeddingProvider.GetEmbedding(chunk.Content)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %v", err)
	}

	// Create unique ID for the chunk
	chunkID := fmt.Sprintf("doc:%s:chunk:%d", uuid.New().String(), chunkIndex)

	// Create metadata
	metadata := map[string]string{
		"source":     documentPath,
		"index":      fmt.Sprintf("%d", chunkIndex),
		"chunk_size": fmt.Sprintf("%d", len(chunk.Content)),
		"type":       "document_chunk",
	}

	// Store the embedding in the vector store
	if err := g.vectorStore.Store(chunkID, embedding, metadata); err != nil {
		return fmt.Errorf("failed to store embedding: %v", err)
	}

	// Create a node for the chunk in the knowledge graph
	g.mu.Lock()
	defer g.mu.Unlock()

	node := &Node{
		ID:        chunkID,
		Type:      DocumentNode,
		Content:   chunk.Content,
		Embedding: embedding,
		Metadata:  metadata,
		Score:     1.0, // Initial score
	}

	g.graph.AddNode(node)

	return nil
}

// createRelationshipsBetweenDocumentChunks creates relationships between document chunks
func (g *GraphRAGTool) createRelationshipsBetweenDocumentChunks(documentPath string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Find all nodes related to this document
	documentNodes := []*Node{}
	for _, node := range g.graph.Nodes {
		if node.Type == DocumentNode && node.Metadata["source"] == documentPath {
			documentNodes = append(documentNodes, node)
		}
	}

	// Create chronological relationships between consecutive chunks
	for i := 0; i < len(documentNodes)-1; i++ {
		currentNode := documentNodes[i]
		nextNode := documentNodes[i+1]

		// Add chronological edge
		g.graph.AddEdge(currentNode, nextNode, 0.9, EdgeChronological, map[string]string{
			"relationship": "follows",
		})
	}

	// Create similarity relationships between chunks
	for i := 0; i < len(documentNodes); i++ {
		for j := i + 1; j < len(documentNodes); j++ {
			node1 := documentNodes[i]
			node2 := documentNodes[j]

			// Calculate similarity
			similarity := cosineSimilarity(node1.Embedding, node2.Embedding)

			// If similarity is above threshold, create a relationship
			if similarity >= g.similarityThreshold {
				// Add similarity edge
				g.graph.AddEdge(node1, node2, similarity, EdgeSimilarity, map[string]string{
					"similarity": fmt.Sprintf("%.4f", similarity),
				})

				// Add reverse edge
				g.graph.AddEdge(node2, node1, similarity, EdgeSimilarity, map[string]string{
					"similarity": fmt.Sprintf("%.4f", similarity),
				})
			}
		}
	}

	return nil
}

// Search performs a graph-enhanced search for the given query
func (g *GraphRAGTool) Search(query string, options models.GenerateOptions) (*GraphSearchResult, error) {
	result := NewGraphSearchResult()

	// 1. Get embedding for the query
	queryEmbedding, err := g.embeddingProvider.GetEmbedding(query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %v", err)
	}

	// 2. Initial vector search to find similar chunks
	searchResults, err := g.vectorStore.Search(queryEmbedding, 5, nil)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %v", err)
	}

	// 3. Create a query node
	queryNode := &Node{
		ID:        fmt.Sprintf("query:%s", uuid.New().String()),
		Type:      QueryNode,
		Content:   query,
		Embedding: queryEmbedding,
		Metadata:  map[string]string{"type": "query"},
	}

	// 4. Add query node to the graph (temporarily)
	g.mu.Lock()
	g.graph.AddNode(queryNode)

	// 5. Connect query node with relevant document nodes
	for _, item := range searchResults {
		if node, exists := g.graph.GetNode(item.ID); exists {
			similarity := cosineSimilarity(queryEmbedding, node.Embedding)

			// Add edge from query to document
			g.graph.AddEdge(queryNode, node, similarity, EdgeSimilarity, map[string]string{
				"similarity": fmt.Sprintf("%.4f", similarity),
			})

			// Add to direct matches
			result.DirectMatches = append(result.DirectMatches, node)
		}
	}

	// 6. Apply PageRank to find important nodes in the context of this query
	g.graph.ApplyPageRank(0.85, 10)

	// 7. Get extended context by following graph relationships
	neighbors := g.graph.GetNeighbors(queryNode)
	secondDegreeNodes := make(map[string]*Node)

	// Add first-degree neighbors
	for _, neighbor := range neighbors {
		// Get second-degree neighbors
		for _, secondNeighbor := range g.graph.GetNeighbors(neighbor) {
			if secondNeighbor.ID != queryNode.ID && secondNeighbor.Type == DocumentNode {
				secondDegreeNodes[secondNeighbor.ID] = secondNeighbor
			}
		}
	}

	// 8. Remove query node from the graph after analysis
	delete(g.graph.Nodes, queryNode.ID)
	g.mu.Unlock()

	// 9. Add second-degree nodes to related nodes
	for _, node := range secondDegreeNodes {
		result.RelatedNodes = append(result.RelatedNodes, node)
	}

	// 10. Analyze relationships between the nodes
	result.Relationships = g.analyzeRelationships(result.DirectMatches, result.RelatedNodes)

	// 11. Generate a summary of the findings using LLM
	summary, err := g.generateSummary(query, result, options)
	if err != nil {
		return nil, fmt.Errorf("failed to generate summary: %v", err)
	}
	result.Summary = summary

	return result, nil
}

// analyzeRelationships finds and describes relationships between nodes
func (g *GraphRAGTool) analyzeRelationships(directMatches, relatedNodes []*Node) []Relationship {
	relationships := []Relationship{}

	// Analyze direct matches with each other
	for i := 0; i < len(directMatches); i++ {
		for j := i + 1; j < len(directMatches); j++ {
			node1 := directMatches[i]
			node2 := directMatches[j]

			similarity := cosineSimilarity(node1.Embedding, node2.Embedding)
			if similarity >= g.similarityThreshold {
				explanation := fmt.Sprintf("These nodes contain semantically similar information (%.2f similarity)", similarity)

				relationships = append(relationships, Relationship{
					Source:      node1,
					Target:      node2,
					Type:        EdgeSimilarity,
					Explanation: explanation,
					Strength:    similarity,
				})
			}
		}
	}

	// Analyze direct matches with related nodes
	for _, direct := range directMatches {
		for _, related := range relatedNodes {
			// Check for existing graph edges
			for _, edge := range direct.Connections {
				if edge.Target.ID == related.ID {
					explanation := fmt.Sprintf("Connected by %s relationship", edge.Type)

					relationships = append(relationships, Relationship{
						Source:      direct,
						Target:      related,
						Type:        edge.Type,
						Explanation: explanation,
						Strength:    edge.Weight,
					})
				}
			}
		}
	}

	return relationships
}

// generateSummary generates a summary of the search results using the LLM
func (g *GraphRAGTool) generateSummary(query string, result *GraphSearchResult, options models.GenerateOptions) (string, error) {
	// Prepare context for the LLM
	var contextBuilder strings.Builder

	// Add query
	contextBuilder.WriteString(fmt.Sprintf("Query: %s\n\n", query))

	// Add direct matches
	contextBuilder.WriteString("Direct Matches:\n")
	for i, node := range result.DirectMatches {
		contextBuilder.WriteString(fmt.Sprintf("%d. %s\n", i+1, truncateString(node.Content, 200)))
	}
	contextBuilder.WriteString("\n")

	// Add relationships
	if len(result.Relationships) > 0 {
		contextBuilder.WriteString("Relationships Found:\n")
		for i, rel := range result.Relationships {
			contextBuilder.WriteString(fmt.Sprintf("%d. %s: %s\n",
				i+1, rel.Type, rel.Explanation))
		}
		contextBuilder.WriteString("\n")
	}

	// Create system message
	systemMessage := "You are a knowledge graph assistant. Analyze the provided context from a graph RAG search and provide a concise summary that answers the query. Focus on synthesizing information and highlighting key relationships between concepts."

	// Create user message
	userMessage := contextBuilder.String() + "\nPlease provide a comprehensive summary that answers the query based on the above information."

	// Prepare messages for the LLM
	messages := []models.Message{
		{Role: "system", Content: systemMessage},
		{Role: "user", Content: userMessage},
	}

	// Set up generate options
	generateOptions := options

	// Generate response using LLM
	response, err := tools.Generate(messages, generateOptions)
	if err != nil {
		return "", fmt.Errorf("failed to generate summary: %v", err)
	}

	return response, nil
}

// cosineSimilarity calculates the cosine similarity between two vectors
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, magnitudeA, magnitudeB float64

	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		magnitudeA += a[i] * a[i]
		magnitudeB += b[i] * b[i]
	}

	magnitudeA = math.Sqrt(magnitudeA)
	magnitudeB = math.Sqrt(magnitudeB)

	if magnitudeA == 0 || magnitudeB == 0 {
		return 0
	}

	return dotProduct / (magnitudeA * magnitudeB)
}
