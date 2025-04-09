package memory

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestPostgresVectorStorage(t *testing.T) {
	// Skip if no connection string is provided
	connStr := os.Getenv("POSTGRES_TEST_CONNECTION_STRING")
	if connStr == "" {
		t.Skip("Skipping PostgreSQL test: POSTGRES_TEST_CONNECTION_STRING not set")
	}

	// Create PostgreSQL vector storage
	opts := PostgresVectorOptions{
		ConnectionString: connStr,
		TableName:        "test_vectors",
		VectorDimension:  3,
		BatchSize:        10,
	}

	store, err := NewPostgresVectorStorage(opts)
	if err != nil {
		t.Fatalf("Failed to create PostgreSQL vector storage: %v", err)
	}
	defer store.Close()

	// Clean up before test
	err = store.Clear(context.Background())
	if err != nil {
		t.Fatalf("Failed to clear store: %v", err)
	}

	// Test vectors with more complex metadata
	vectors := []Vector{
		{
			ID:     "vec1",
			Values: []float32{1.0, 0.0, 0.0},
			Metadata: map[string]interface{}{
				"category": "test",
				"tag":      "a",
				"score":    "10",
				"nested": map[string]interface{}{
					"keywords": "machine learning, vector search",
					"id":       "1",
				},
			},
		},
		{
			ID:     "vec2",
			Values: []float32{0.0, 1.0, 0.0},
			Metadata: map[string]interface{}{
				"category": "test",
				"tag":      "b",
				"score":    "20",
				"nested": map[string]interface{}{
					"keywords": "embeddings, similarity",
					"id":       "2",
				},
			},
		},
		{
			ID:     "vec3",
			Values: []float32{0.0, 0.0, 1.0},
			Metadata: map[string]interface{}{
				"category": "test",
				"tag":      "c",
				"score":    "30",
				"nested": map[string]interface{}{
					"keywords": "databases, vectors",
					"id":       "3",
				},
			},
		},
		{
			ID:     "vec4",
			Values: []float32{0.5, 0.5, 0.0},
			Metadata: map[string]interface{}{
				"category": "prod",
				"tag":      "d",
				"score":    "25",
				"nested": map[string]interface{}{
					"keywords": "production, deployment",
					"id":       "4",
				},
			},
		},
	}

	// Test Store
	t.Run("Store", func(t *testing.T) {
		err := store.Store(context.Background(), vectors)
		if err != nil {
			t.Fatalf("Failed to store vectors: %v", err)
		}
	})

	// Test Search
	t.Run("Search", func(t *testing.T) {
		query := Vector{
			Values: []float32{1.0, 0.0, 0.0},
		}

		opts := VectorSearchOptions{
			Limit:     10,
			Threshold: 0.5,
		}

		results, err := store.Search(context.Background(), query, opts)
		if err != nil {
			t.Fatalf("Failed to search vectors: %v", err)
		}

		if len(results) == 0 {
			t.Errorf("Expected search results, got none")
		}

		// Verify that most similar vector is first
		if len(results) > 0 && results[0].Vector.ID != "vec1" {
			t.Errorf("Expected most similar vector to be vec1, got %s", results[0].Vector.ID)
		}
	})

	// Test Search with filter
	t.Run("SearchWithFilter", func(t *testing.T) {
		query := Vector{
			Values: []float32{0.5, 0.5, 0.0},
		}

		opts := VectorSearchOptions{
			Limit:     10,
			Threshold: 0.0,
			Filter: map[string]interface{}{
				"category": "test",
			},
		}

		results, err := store.Search(context.Background(), query, opts)
		if err != nil {
			t.Fatalf("Failed to search vectors with filter: %v", err)
		}

		for _, result := range results {
			if result.Vector.Metadata["category"] != "test" {
				t.Errorf("Expected only 'test' category, got %v", result.Vector.Metadata["category"])
			}
		}
	})

	// Test Search with comparison operator
	t.Run("SearchWithComparisonOperator", func(t *testing.T) {
		query := Vector{
			Values: []float32{0.0, 0.0, 0.0},
		}

		opts := VectorSearchOptions{
			Limit: 10,
			Filter: map[string]interface{}{
				"score": map[string]interface{}{
					"$gt": "20",
				},
			},
		}

		results, err := store.Search(context.Background(), query, opts)
		if err != nil {
			t.Fatalf("Failed to search vectors with comparison filter: %v", err)
		}

		for _, result := range results {
			score := result.Vector.Metadata["score"].(string)
			if score <= "20" {
				t.Errorf("Expected scores > 20, got %s", score)
			}
		}
	})

	// Test Search with nested path
	t.Run("SearchWithNestedPath", func(t *testing.T) {
		query := Vector{
			Values: []float32{0.0, 0.0, 0.0},
		}

		opts := VectorSearchOptions{
			Limit: 10,
			Filter: map[string]interface{}{
				"nested.id": map[string]interface{}{
					"$gt": "2",
				},
			},
		}

		results, err := store.Search(context.Background(), query, opts)
		if err != nil {
			t.Fatalf("Failed to search vectors with nested path filter: %v", err)
		}

		for _, result := range results {
			nestedMap := result.Vector.Metadata["nested"].(map[string]interface{})
			id := nestedMap["id"].(string)
			if id <= "2" {
				t.Errorf("Expected nested.id > 2, got %s", id)
			}
		}
	})

	// Test Search with regex
	t.Run("SearchWithRegex", func(t *testing.T) {
		query := Vector{
			Values: []float32{0.0, 0.0, 0.0},
		}

		opts := VectorSearchOptions{
			Limit: 10,
			Filter: map[string]interface{}{
				"nested.keywords": map[string]interface{}{
					"$regex": "vector",
				},
			},
		}

		results, err := store.Search(context.Background(), query, opts)
		if err != nil {
			t.Fatalf("Failed to search vectors with regex filter: %v", err)
		}

		foundKeyword := false
		for _, result := range results {
			nestedMap := result.Vector.Metadata["nested"].(map[string]interface{})
			keywords := nestedMap["keywords"].(string)
			if strings.Contains(keywords, "vector") {
				foundKeyword = true
				break
			}
		}

		if !foundKeyword {
			t.Errorf("Expected to find vectors with 'vector' in keywords")
		}
	})

	// Test Delete
	t.Run("Delete", func(t *testing.T) {
		err := store.Delete(context.Background(), []string{"vec1"})
		if err != nil {
			t.Fatalf("Failed to delete vector: %v", err)
		}

		// Verify vector is deleted
		query := Vector{
			Values: []float32{1.0, 0.0, 0.0},
		}

		opts := VectorSearchOptions{
			Limit: 10,
		}

		results, err := store.Search(context.Background(), query, opts)
		if err != nil {
			t.Fatalf("Failed to search vectors: %v", err)
		}

		for _, result := range results {
			if result.Vector.ID == "vec1" {
				t.Errorf("Vector vec1 should be deleted")
			}
		}
	})

	// Test Clear
	t.Run("Clear", func(t *testing.T) {
		err := store.Clear(context.Background())
		if err != nil {
			t.Fatalf("Failed to clear vectors: %v", err)
		}

		// Verify all vectors are cleared
		query := Vector{
			Values: []float32{0.0, 0.0, 0.0},
		}

		opts := VectorSearchOptions{
			Limit: 10,
		}

		results, err := store.Search(context.Background(), query, opts)
		if err != nil {
			t.Fatalf("Failed to search vectors: %v", err)
		}

		if len(results) > 0 {
			t.Errorf("Expected no vectors after clear, got %d", len(results))
		}
	})
}
