package memory

import (
	"context"
	"fmt"
)

// Vector represents a vector embedding
type Vector struct {
	ID       string                 `json:"id"`
	Values   []float32              `json:"values"`
	Metadata map[string]interface{} `json:"metadata"`
}

// VectorSearchResult represents a search result with similarity score
type VectorSearchResult struct {
	Vector   Vector  `json:"vector"`
	Score    float32 `json:"score"`
	Distance float32 `json:"distance"`
}

// VectorSearchOptions contains options for vector search
type VectorSearchOptions struct {
	Limit     int                    `json:"limit"`
	Threshold float32                `json:"threshold"`
	Filter    map[string]interface{} `json:"filter"`
}

// VectorStorage defines the interface for vector storage and retrieval
type VectorStorage interface {
	// Store stores vectors in the vector store
	Store(ctx context.Context, vectors []Vector) error

	// Search searches for similar vectors
	Search(ctx context.Context, query Vector, opts VectorSearchOptions) ([]VectorSearchResult, error)

	// Delete removes vectors from the store
	Delete(ctx context.Context, ids []string) error

	// Clear removes all vectors from the store
	Clear(ctx context.Context) error
}

// BaseVectorStore provides a base implementation of VectorStore
type BaseVectorStore struct {
	vectors map[string]Vector
}

// NewBaseVectorStore creates a new base vector store
func NewBaseVectorStore() *BaseVectorStore {
	return &BaseVectorStore{
		vectors: make(map[string]Vector),
	}
}

// Store implements VectorStore.Store
func (s *BaseVectorStore) Store(ctx context.Context, vectors []Vector) error {
	for _, v := range vectors {
		if v.ID == "" {
			return fmt.Errorf("vector ID cannot be empty")
		}
		if len(v.Values) == 0 {
			return fmt.Errorf("vector values cannot be empty")
		}
		s.vectors[v.ID] = v
	}
	return nil
}

// Search implements VectorStore.Search
func (s *BaseVectorStore) Search(ctx context.Context, query Vector, opts VectorSearchOptions) ([]VectorSearchResult, error) {
	if len(query.Values) == 0 {
		return nil, fmt.Errorf("query vector values cannot be empty")
	}

	if opts.Limit <= 0 {
		opts.Limit = 10
	}

	results := make([]VectorSearchResult, 0, len(s.vectors))
	for _, v := range s.vectors {
		if len(v.Values) != len(query.Values) {
			continue
		}

		// Calculate cosine similarity
		score := computeCosineSimilarity(query.Values, v.Values)
		if score < opts.Threshold {
			continue
		}

		// Apply metadata filter if provided
		if !matchesFilter(v.Metadata, opts.Filter) {
			continue
		}

		results = append(results, VectorSearchResult{
			Vector:   v,
			Score:    score,
			Distance: 1 - score,
		})
	}

	// Sort results by score (descending)
	sortVectorResults(results)

	// Limit results
	if len(results) > opts.Limit {
		results = results[:opts.Limit]
	}

	return results, nil
}

// Delete implements VectorStore.Delete
func (s *BaseVectorStore) Delete(ctx context.Context, ids []string) error {
	for _, id := range ids {
		delete(s.vectors, id)
	}
	return nil
}

// Clear implements VectorStore.Clear
func (s *BaseVectorStore) Clear(ctx context.Context) error {
	s.vectors = make(map[string]Vector)
	return nil
}

// computeCosineSimilarity calculates the cosine similarity between two vectors
func computeCosineSimilarity(a, b []float32) float32 {
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

// matchesFilter checks if metadata matches the filter criteria
func matchesFilter(metadata, filter map[string]interface{}) bool {
	if filter == nil {
		return true
	}

	for k, v := range filter {
		if metadata[k] != v {
			return false
		}
	}
	return true
}

// sortVectorResults sorts vector results by score in descending order
func sortVectorResults(results []VectorSearchResult) {
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].Score < results[j].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}
