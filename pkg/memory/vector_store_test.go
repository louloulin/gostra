package memory

import (
	"context"
	"testing"
)

func TestBaseVectorStore(t *testing.T) {
	store := NewBaseVectorStore()
	ctx := context.Background()

	// Test vectors
	vectors := []Vector{
		{
			ID:     "1",
			Values: []float32{1.0, 0.0, 0.0},
			Metadata: map[string]interface{}{
				"type": "test",
				"tag":  "a",
			},
		},
		{
			ID:     "2",
			Values: []float32{0.0, 1.0, 0.0},
			Metadata: map[string]interface{}{
				"type": "test",
				"tag":  "b",
			},
		},
		{
			ID:     "3",
			Values: []float32{0.0, 0.0, 1.0},
			Metadata: map[string]interface{}{
				"type": "test",
				"tag":  "c",
			},
		},
	}

	t.Run("Store", func(t *testing.T) {
		err := store.Store(ctx, vectors)
		if err != nil {
			t.Fatalf("Store failed: %v", err)
		}

		// Test storing invalid vectors
		err = store.Store(ctx, []Vector{{ID: "", Values: []float32{1.0}}})
		if err == nil {
			t.Error("Expected error for empty ID")
		}

		err = store.Store(ctx, []Vector{{ID: "test", Values: []float32{}}})
		if err == nil {
			t.Error("Expected error for empty values")
		}
	})

	t.Run("Search", func(t *testing.T) {
		query := Vector{
			Values: []float32{1.0, 0.0, 0.0},
		}
		opts := VectorSearchOptions{
			Limit:     2,
			Threshold: 0.5,
		}

		results, err := store.Search(ctx, query, opts)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) == 0 {
			t.Error("Expected search results")
		}

		if results[0].Vector.ID != "1" {
			t.Errorf("Expected first result to be vector 1, got %s", results[0].Vector.ID)
		}

		if results[0].Score != 1.0 {
			t.Errorf("Expected perfect score for identical vector, got %f", results[0].Score)
		}
	})

	t.Run("SearchWithFilter", func(t *testing.T) {
		query := Vector{
			Values: []float32{1.0, 0.0, 0.0},
		}
		opts := VectorSearchOptions{
			Limit:     2,
			Threshold: 0.5,
			Filter: map[string]interface{}{
				"tag": "b",
			},
		}

		results, err := store.Search(ctx, query, opts)
		if err != nil {
			t.Fatalf("Search with filter failed: %v", err)
		}

		for _, result := range results {
			if result.Vector.Metadata["tag"] != "b" {
				t.Errorf("Filter not applied correctly, got tag %v", result.Vector.Metadata["tag"])
			}
		}
	})

	t.Run("Delete", func(t *testing.T) {
		err := store.Delete(ctx, []string{"1"})
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		// Verify deletion
		query := Vector{
			Values: []float32{1.0, 0.0, 0.0},
		}
		results, err := store.Search(ctx, query, VectorSearchOptions{Limit: 1})
		if err != nil {
			t.Fatalf("Search after delete failed: %v", err)
		}

		for _, result := range results {
			if result.Vector.ID == "1" {
				t.Error("Vector 1 should have been deleted")
			}
		}
	})

	t.Run("Clear", func(t *testing.T) {
		err := store.Clear(ctx)
		if err != nil {
			t.Fatalf("Clear failed: %v", err)
		}

		// Verify clear
		query := Vector{
			Values: []float32{1.0, 0.0, 0.0},
		}
		results, err := store.Search(ctx, query, VectorSearchOptions{Limit: 10})
		if err != nil {
			t.Fatalf("Search after clear failed: %v", err)
		}

		if len(results) != 0 {
			t.Error("Store should be empty after clear")
		}
	})
}

func TestComputeCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        []float32
		b        []float32
		expected float32
	}{
		{
			name:     "identical vectors",
			a:        []float32{1.0, 0.0, 0.0},
			b:        []float32{1.0, 0.0, 0.0},
			expected: 1.0,
		},
		{
			name:     "orthogonal vectors",
			a:        []float32{1.0, 0.0, 0.0},
			b:        []float32{0.0, 1.0, 0.0},
			expected: 0.0,
		},
		{
			name:     "opposite vectors",
			a:        []float32{1.0, 0.0, 0.0},
			b:        []float32{-1.0, 0.0, 0.0},
			expected: -1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computeCosineSimilarity(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("Expected %f, got %f", tt.expected, result)
			}
		})
	}
}
