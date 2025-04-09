package main

// GraphSearchResult contains the result of a graph RAG search
type GraphSearchResult struct {
	// Direct matches found from vector search
	DirectMatches []*Node

	// Related nodes found through graph traversal
	RelatedNodes []*Node

	// Relationships discovered between nodes
	Relationships []Relationship

	// Generated summary from LLM
	Summary string
}

// Relationship represents a connection between two nodes with explanation
type Relationship struct {
	// Source node
	Source *Node

	// Target node
	Target *Node

	// Type of relationship
	Type string

	// Human-readable explanation of the relationship
	Explanation string

	// Strength of the relationship (0-1)
	Strength float64
}

// NewGraphSearchResult creates a new empty graph search result
func NewGraphSearchResult() *GraphSearchResult {
	return &GraphSearchResult{
		DirectMatches: make([]*Node, 0),
		RelatedNodes:  make([]*Node, 0),
		Relationships: make([]Relationship, 0),
		Summary:       "",
	}
}

// truncateString truncates a string to the specified length and adds ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
