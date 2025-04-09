package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// NodeType represents the type of node in the knowledge graph
type NodeType string

const (
	// DocumentNode represents a document chunk node
	DocumentNode NodeType = "document"
	// QueryNode represents a query node
	QueryNode NodeType = "query"
	// EntityNode represents an entity node
	EntityNode NodeType = "entity"
)

// Node represents a node in the knowledge graph
type Node struct {
	ID          string            // Unique identifier for the node
	Type        NodeType          // Type of the node
	Content     string            // Content of the node
	Embedding   []float64         // Vector embedding of the node
	Metadata    map[string]string // Additional metadata
	Score       float64           // Score for ranking purposes
	Connections []*Edge           // Connections to other nodes
}

// Edge represents a connection between two nodes in the knowledge graph
type Edge struct {
	Source   *Node             // Source node
	Target   *Node             // Target node
	Weight   float64           // Weight or strength of the connection
	Type     string            // Type of relationship
	Metadata map[string]string // Additional metadata
}

// KnowledgeGraph represents a graph of connected nodes
type KnowledgeGraph struct {
	Nodes map[string]*Node // All nodes in the graph
	Edges []*Edge          // All edges in the graph
}

// NewKnowledgeGraph creates a new knowledge graph
func NewKnowledgeGraph() *KnowledgeGraph {
	return &KnowledgeGraph{
		Nodes: make(map[string]*Node),
		Edges: []*Edge{},
	}
}

// AddNode adds a node to the graph
func (g *KnowledgeGraph) AddNode(node *Node) {
	if _, exists := g.Nodes[node.ID]; !exists {
		g.Nodes[node.ID] = node
	}
}

// AddEdge adds an edge between two nodes
func (g *KnowledgeGraph) AddEdge(source, target *Node, weight float64, edgeType string, metadata map[string]string) {
	edge := &Edge{
		Source:   source,
		Target:   target,
		Weight:   weight,
		Type:     edgeType,
		Metadata: metadata,
	}

	source.Connections = append(source.Connections, edge)
	g.Edges = append(g.Edges, edge)
}

// GetNode retrieves a node by ID
func (g *KnowledgeGraph) GetNode(id string) (*Node, bool) {
	node, exists := g.Nodes[id]
	return node, exists
}

// GetNeighbors returns all neighbors of a node
func (g *KnowledgeGraph) GetNeighbors(node *Node) []*Node {
	neighbors := []*Node{}
	for _, edge := range node.Connections {
		neighbors = append(neighbors, edge.Target)
	}
	return neighbors
}

// ApplyPageRank applies PageRank algorithm to the graph
func (g *KnowledgeGraph) ApplyPageRank(dampingFactor float64, iterations int) {
	// Initialize all nodes with a score of 1.0 / n
	n := float64(len(g.Nodes))
	for _, node := range g.Nodes {
		node.Score = 1.0 / n
	}

	// Run PageRank iterations
	for i := 0; i < iterations; i++ {
		scores := make(map[string]float64)

		// Calculate new scores
		for _, node := range g.Nodes {
			scores[node.ID] = (1.0 - dampingFactor) / n

			// Calculate sum of incoming connections
			for _, edge := range g.Edges {
				if edge.Target.ID == node.ID {
					sourceNode := edge.Source
					outgoingEdges := len(sourceNode.Connections)
					if outgoingEdges > 0 {
						scores[node.ID] += dampingFactor * sourceNode.Score * edge.Weight / float64(outgoingEdges)
					}
				}
			}
		}

		// Update scores
		for id, score := range scores {
			g.Nodes[id].Score = score
		}
	}
}

// GetTopNodes returns the top n nodes by score
func (g *KnowledgeGraph) GetTopNodes(n int) []*Node {
	nodes := make([]*Node, 0, len(g.Nodes))
	for _, node := range g.Nodes {
		nodes = append(nodes, node)
	}

	// Sort nodes by score (descending)
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Score > nodes[j].Score
	})

	// Return the top n nodes
	if n > len(nodes) {
		n = len(nodes)
	}
	return nodes[:n]
}

// GetPathBetween finds the shortest path between two nodes
func (g *KnowledgeGraph) GetPathBetween(source, target string) []*Node {
	// Basic implementation of Dijkstra's algorithm
	sourceNode, exists := g.GetNode(source)
	if !exists {
		return nil
	}

	targetNode, exists := g.GetNode(target)
	if !exists {
		return nil
	}

	// Initialize distances and visited nodes
	distances := make(map[string]float64)
	previous := make(map[string]string)
	visited := make(map[string]bool)

	// Set initial values
	for id := range g.Nodes {
		distances[id] = math.Inf(1)
	}
	distances[source] = 0

	// Find shortest path
	for len(visited) < len(g.Nodes) {
		// Find node with minimum distance
		current := ""
		minDist := math.Inf(1)
		for id, dist := range distances {
			if !visited[id] && dist < minDist {
				minDist = dist
				current = id
			}
		}

		// If no node found or target reached, break
		if current == "" || current == target {
			break
		}

		// Mark current node as visited
		visited[current] = true

		// Update distances to neighbors
		currentNode := g.Nodes[current]
		for _, edge := range currentNode.Connections {
			neighbor := edge.Target.ID
			if !visited[neighbor] {
				dist := distances[current] + 1/edge.Weight
				if dist < distances[neighbor] {
					distances[neighbor] = dist
					previous[neighbor] = current
				}
			}
		}
	}

	// Reconstruct path
	path := []*Node{}
	for at := target; at != ""; at = previous[at] {
		node, _ := g.GetNode(at)
		path = append([]*Node{node}, path...)
		if at == source {
			break
		}
	}

	// Return path (empty if no path found)
	if len(path) > 0 && path[0].ID == source {
		return path
	}
	return nil
}

// String returns a string representation of the graph
func (g *KnowledgeGraph) String() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Knowledge Graph with %d nodes and %d edges\n", len(g.Nodes), len(g.Edges)))

	// Print nodes with highest scores
	sb.WriteString("\nTop 5 nodes by score:\n")
	topNodes := g.GetTopNodes(5)
	for i, node := range topNodes {
		sb.WriteString(fmt.Sprintf("%d. %s (Score: %.4f): %s\n",
			i+1, node.ID, node.Score, truncateString(node.Content, 50)))
	}

	return sb.String()
}

// Helper function to truncate long strings
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// GraphSearchResult represents a search result from the knowledge graph
type GraphSearchResult struct {
	DirectMatches []*Node        // Directly matched nodes
	RelatedNodes  []*Node        // Related nodes found through graph analysis
	Relationships []Relationship // Identified relationships between nodes
	Summary       string         // Summary of the findings
}

// Relationship represents a relationship between nodes
type Relationship struct {
	Source      *Node   // Source node
	Target      *Node   // Target node
	Type        string  // Type of relationship
	Explanation string  // Explanation of the relationship
	Strength    float64 // Strength of the relationship
}

// NewGraphSearchResult creates a new graph search result
func NewGraphSearchResult() *GraphSearchResult {
	return &GraphSearchResult{
		DirectMatches: []*Node{},
		RelatedNodes:  []*Node{},
		Relationships: []Relationship{},
	}
}
