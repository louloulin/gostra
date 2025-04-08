package document

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDocumentProcessor(t *testing.T) {
	// Create a temporary test file
	content := "This is a test document.\nIt has multiple lines.\nWe will use it to test chunking."
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create processor with test options
	processor := NewDocumentProcessor(ChunkOptions{
		Strategy: "fixed",
		Size:     20,
		Overlap:  5,
	})

	t.Run("LoadFromFile", func(t *testing.T) {
		doc, err := processor.LoadFromFile(tmpFile)
		if err != nil {
			t.Fatalf("LoadFromFile failed: %v", err)
		}

		if doc.Content != content {
			t.Errorf("Expected content %q, got %q", content, doc.Content)
		}

		if doc.Metadata["type"] != "file" {
			t.Errorf("Expected type 'file', got %v", doc.Metadata["type"])
		}
	})

	t.Run("Chunk", func(t *testing.T) {
		doc := &ProcessedDocument{
			ID:      "test",
			Content: content,
			Metadata: map[string]interface{}{
				"type": "test",
			},
		}

		err := processor.Chunk(doc)
		if err != nil {
			t.Fatalf("Chunk failed: %v", err)
		}

		if len(doc.Chunks) == 0 {
			t.Error("Expected chunks to be created")
		}

		// Verify chunk properties
		for i, chunk := range doc.Chunks {
			if chunk.ID == "" {
				t.Errorf("Chunk %d has empty ID", i)
			}
			if chunk.Content == "" {
				t.Errorf("Chunk %d has empty content", i)
			}
			if chunk.Metadata["docID"] != "test" {
				t.Errorf("Chunk %d has wrong docID", i)
			}
		}

		// Verify content coverage
		allContent := ""
		for _, chunk := range doc.Chunks {
			allContent += chunk.Content
		}
		if !containsSubstring(content, allContent) {
			t.Error("Chunks do not cover all content")
		}
	})

	t.Run("Process", func(t *testing.T) {
		doc, err := processor.Process(context.Background(), tmpFile)
		if err != nil {
			t.Fatalf("Process failed: %v", err)
		}

		if doc.Content != content {
			t.Errorf("Expected content %q, got %q", content, doc.Content)
		}

		if len(doc.Chunks) == 0 {
			t.Error("Expected chunks to be created")
		}
	})
}

// containsSubstring checks if all characters in substr appear in str in order
func containsSubstring(str, substr string) bool {
	i := 0
	for _, c := range substr {
		for i < len(str) && rune(str[i]) != c {
			i++
		}
		if i >= len(str) {
			return false
		}
		i++
	}
	return true
}
