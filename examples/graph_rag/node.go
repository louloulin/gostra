package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// GraphNode represents a node in the knowledge graph
type GraphNode struct {
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
	Edges []*GraphEdge
}

// GraphEdge represents a directed connection between two nodes
type GraphEdge struct {
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

// NewGraphNode creates a new node with the provided content
func NewGraphNode(content string, metadata map[string]interface{}) *GraphNode {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	return &GraphNode{
		ID:        uuid.New().String(),
		Content:   content,
		Metadata:  metadata,
		Vector:    nil, // Will be populated when embedding is generated
		CreatedAt: time.Now(),
		Edges:     make([]*GraphEdge, 0),
	}
}

// AddEdge creates and adds a new edge from this node to the target node
func (n *GraphNode) AddEdge(edgeType string, targetNode *GraphNode, properties map[string]interface{}) *GraphEdge {
	if properties == nil {
		properties = make(map[string]interface{})
	}

	edge := &GraphEdge{
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
func (n *GraphNode) GetEdgesByType(edgeType string) []*GraphEdge {
	edges := make([]*GraphEdge, 0)
	for _, edge := range n.Edges {
		if edge.Type == edgeType {
			edges = append(edges, edge)
		}
	}
	return edges
}

// ToJSON serializes the node to JSON
func (n *GraphNode) ToJSON() (string, error) {
	data, err := json.Marshal(n)
	if err != nil {
		return "", fmt.Errorf("failed to marshal node to JSON: %w", err)
	}
	return string(data), nil
}

// FromJSON deserializes a node from JSON
func GraphNodeFromJSON(data string) (*GraphNode, error) {
	var node GraphNode
	if err := json.Unmarshal([]byte(data), &node); err != nil {
		return nil, fmt.Errorf("failed to unmarshal node from JSON: %w", err)
	}
	return &node, nil
}
