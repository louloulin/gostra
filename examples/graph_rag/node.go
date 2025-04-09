package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Node represents a node in the knowledge graph
type Node struct {
	// Unique identifier for the node
	ID string

	// Content of the node
	Content string

	// Metadata associated with the node
	Metadata map[string]interface{}

	// Vector representation of the node content
	Vector []float64

	// When the node was created
	CreatedAt time.Time

	// Edges connected to this node
	Edges []*Edge
}

// Edge represents a directed connection between two nodes
type Edge struct {
	// Type of relationship
	Type string

	// Source node ID
	SourceID string

	// Target node ID
	TargetID string

	// Properties of the edge
	Properties map[string]interface{}

	// When the edge was created
	CreatedAt time.Time
}

// NewNode creates a new node with the provided content
func NewNode(content string, metadata map[string]interface{}) *Node {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	return &Node{
		ID:        uuid.New().String(),
		Content:   content,
		Metadata:  metadata,
		Vector:    nil, // Will be populated when embedding is generated
		CreatedAt: time.Now(),
		Edges:     make([]*Edge, 0),
	}
}

// AddEdge creates and adds a new edge from this node to the target node
func (n *Node) AddEdge(edgeType string, targetNode *Node, properties map[string]interface{}) *Edge {
	if properties == nil {
		properties = make(map[string]interface{})
	}

	edge := &Edge{
		Type:       edgeType,
		SourceID:   n.ID,
		TargetID:   targetNode.ID,
		Properties: properties,
		CreatedAt:  time.Now(),
	}

	n.Edges = append(n.Edges, edge)
	return edge
}

// GetEdgesByType returns all edges of a specific type from this node
func (n *Node) GetEdgesByType(edgeType string) []*Edge {
	edges := make([]*Edge, 0)
	for _, edge := range n.Edges {
		if edge.Type == edgeType {
			edges = append(edges, edge)
		}
	}
	return edges
}

// ToJSON serializes the node to JSON
func (n *Node) ToJSON() (string, error) {
	data, err := json.Marshal(n)
	if err != nil {
		return "", fmt.Errorf("failed to marshal node to JSON: %w", err)
	}
	return string(data), nil
}

// FromJSON deserializes a node from JSON
func NodeFromJSON(data string) (*Node, error) {
	var node Node
	if err := json.Unmarshal([]byte(data), &node); err != nil {
		return nil, fmt.Errorf("failed to unmarshal node from JSON: %w", err)
	}
	return &node, nil
}
