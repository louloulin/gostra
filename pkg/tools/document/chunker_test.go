package document

import (
	"testing"
)

func TestDocumentChunkerGetID(t *testing.T) {
	chunker := NewDocumentChunker(DocumentChunkerOptions{})
	if id := chunker.GetID(); id != "document_chunker" {
		t.Errorf("Expected tool ID 'document_chunker', got '%s'", id)
	}
}

func TestDocumentChunkerGetDescription(t *testing.T) {
	chunker := NewDocumentChunker(DocumentChunkerOptions{})
	if desc := chunker.GetDescription(); desc == "" {
		t.Error("Tool description should not be empty")
	}
}

func TestDocumentChunkerGetInputSchema(t *testing.T) {
	chunker := NewDocumentChunker(DocumentChunkerOptions{})
	schema := chunker.GetInputSchema()

	if schema == nil {
		t.Fatal("Input schema should not be nil")
	}

	// Verify schema
	jsonSchema, err := schema.JSONSchema()
	if err != nil {
		t.Fatalf("Failed to get JSON schema: %v", err)
	}

	// Check properties
	properties, ok := jsonSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Properties should be a map")
	}

	// Check if schema contains necessary properties
	requiredProps := []string{"content", "strategy", "size", "overlap", "separator"}
	for _, prop := range requiredProps {
		if _, exists := properties[prop]; !exists {
			t.Errorf("Schema should have '%s' property", prop)
		}
	}
}

func TestDocumentChunkerExecuteWithDefaultParams(t *testing.T) {
	chunker := NewDocumentChunker(DocumentChunkerOptions{})

	// Create test content
	content := `这是第一段。这是一个测试文档。这是第一段的最后一句。

这是第二段。它包含了多个句子。这是另一个句子。这是最后一句。

这是第三段内容，比较长。文档分块是将长文本分割成更小的片段，以便进行处理、分析或存储。
分块的大小和策略取决于具体的应用场景。例如，在搜索引擎中，文档可能被分成段落或句子级别的块；
在自然语言处理中，文档可能按句子或词组分块；在向量搜索中，块的大小则需要平衡上下文信息和精确匹配之间的关系。`

	// Execute with default parameters
	params := map[string]interface{}{
		"content": content,
	}

	result, err := chunker.Execute(params, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Check result
	chunks, ok := result.([]*DocumentChunk)
	if !ok {
		t.Fatal("Result should be of type []*DocumentChunk")
	}

	// Default should split by newline and create appropriate chunks
	if len(chunks) == 0 {
		t.Fatal("Should have at least one chunk")
	}

	// Check that chunks contain the content
	allContent := ""
	for _, chunk := range chunks {
		allContent += chunk.Content
	}

	// Check if all the original content is included across all chunks
	// Note: This is simplified as chunk overlaps make it hard to exactly match
	for _, line := range []string{"这是第一段", "这是第二段", "这是第三段"} {
		if !containsString(allContent, line) {
			t.Errorf("Chunked content should contain '%s'", line)
		}
	}
}

func TestDocumentChunkerWithFixedStrategy(t *testing.T) {
	chunker := NewDocumentChunker(DocumentChunkerOptions{})

	content := "This is a test document. This should be split into multiple chunks of fixed size."

	params := map[string]interface{}{
		"content":  content,
		"strategy": "fixed",
		"size":     20.0, // Values come in as float64
		"overlap":  5.0,
	}

	result, err := chunker.Execute(params, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	chunks, ok := result.([]*DocumentChunk)
	if !ok {
		t.Fatal("Result should be of type []*DocumentChunk")
	}

	// With size 20 and overlap 5, expect several chunks
	if len(chunks) < 3 {
		t.Fatalf("Expected at least 3 chunks with size 20, got %d", len(chunks))
	}

	// Check chunk sizes
	for i, chunk := range chunks {
		// Last chunk might be smaller
		if i < len(chunks)-1 && len(chunk.Content) != 20 {
			t.Errorf("Chunk %d should have size 20, got %d", i, len(chunk.Content))
		}

		// Check overlap
		if i > 0 {
			prevChunk := chunks[i-1]
			overlap := calculateOverlap(prevChunk.Content, chunk.Content)
			if overlap != 5 && i < len(chunks)-1 {
				t.Errorf("Expected overlap of 5 between chunks %d and %d, got %d", i-1, i, overlap)
			}
		}
	}
}

func TestDocumentChunkerWithSentenceStrategy(t *testing.T) {
	chunker := NewDocumentChunker(DocumentChunkerOptions{})

	content := "This is sentence one. This is sentence two! This is sentence three? This is sentence four. This is a longer sentence that contains multiple clauses and should be treated as a single unit by the sentence chunker."

	params := map[string]interface{}{
		"content":  content,
		"strategy": "sentence",
		"size":     50,
		"overlap":  0,
	}

	result, err := chunker.Execute(params, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	chunks, ok := result.([]*DocumentChunk)
	if !ok {
		t.Fatal("Result should be of type []*DocumentChunk")
	}

	if len(chunks) < 2 {
		t.Fatalf("Expected at least 2 chunks with sentence strategy, got %d", len(chunks))
	}

	// Each chunk should end with a sentence terminator
	for i, chunk := range chunks {
		if i < len(chunks)-1 && !endsWithSentenceTerminator(chunk.Content) {
			t.Errorf("Chunk %d should end with a sentence terminator: '%s'", i, chunk.Content)
		}
	}
}

func TestDocumentChunkerWithParagraphStrategy(t *testing.T) {
	chunker := NewDocumentChunker(DocumentChunkerOptions{})

	content := "Paragraph one.\nStill paragraph one.\n\nParagraph two.\n\nParagraph three.\nStill paragraph three."

	params := map[string]interface{}{
		"content":  content,
		"strategy": "paragraph",
		"size":     100,
		"overlap":  0,
	}

	result, err := chunker.Execute(params, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	chunks, ok := result.([]*DocumentChunk)
	if !ok {
		t.Fatal("Result should be of type []*DocumentChunk")
	}

	if len(chunks) != 3 {
		t.Fatalf("Expected 3 chunks with paragraph strategy, got %d", len(chunks))
	}

	// Check that each chunk represents a paragraph
	if !containsString(chunks[0].Content, "Paragraph one") {
		t.Errorf("First chunk should contain paragraph one: '%s'", chunks[0].Content)
	}

	if !containsString(chunks[1].Content, "Paragraph two") {
		t.Errorf("Second chunk should contain paragraph two: '%s'", chunks[1].Content)
	}

	if !containsString(chunks[2].Content, "Paragraph three") {
		t.Errorf("Third chunk should contain paragraph three: '%s'", chunks[2].Content)
	}
}

func TestDocumentChunkerRecursiveStrategy(t *testing.T) {
	chunker := NewDocumentChunker(DocumentChunkerOptions{})

	content := "Line 1: This is a test.\nLine 2: Another test.\nLine 3: Yet another test.\nLine 4: Final test."

	params := map[string]interface{}{
		"content":   content,
		"strategy":  "recursive",
		"size":      30,
		"overlap":   0,
		"separator": "\n",
	}

	result, err := chunker.Execute(params, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	chunks, ok := result.([]*DocumentChunk)
	if !ok {
		t.Fatal("Result should be of type []*DocumentChunk")
	}

	if len(chunks) < 2 {
		t.Fatalf("Expected at least 2 chunks with recursive strategy, got %d", len(chunks))
	}

	// Check if the chunks were split by the separator
	for _, chunk := range chunks {
		// Count the occurrences of newlines in each chunk - should be reasonable
		newlines := countOccurrences(chunk.Content, "\n")
		if newlines > 2 {
			t.Errorf("Chunk contains too many newlines, doesn't seem to be split correctly: '%s'", chunk.Content)
		}
	}
}

func TestDocumentChunkerEmptyContent(t *testing.T) {
	chunker := NewDocumentChunker(DocumentChunkerOptions{})

	params := map[string]interface{}{
		"content": "",
	}

	_, err := chunker.Execute(params, nil)
	if err == nil {
		t.Fatal("Expected error for empty content, got nil")
	}
}

func TestDocumentChunkerMissingContent(t *testing.T) {
	chunker := NewDocumentChunker(DocumentChunkerOptions{})

	params := map[string]interface{}{}

	_, err := chunker.Execute(params, nil)
	if err == nil {
		t.Fatal("Expected error for missing content, got nil")
	}
}

// Helper functions

func containsString(haystack, needle string) bool {
	return haystack != "" && needle != "" && len(haystack) >= len(needle) && haystack != needle && haystack != "" && needle != "" && len(haystack) >= len(needle) && haystack != needle && contains(haystack, needle)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func calculateOverlap(s1, s2 string) int {
	if len(s1) == 0 || len(s2) == 0 {
		return 0
	}

	// Find the maximum suffix of s1 that is a prefix of s2
	maxOverlap := 0
	for i := 1; i <= minInt(len(s1), len(s2)); i++ {
		if s1[len(s1)-i:] == s2[:i] {
			maxOverlap = i
		}
	}

	return maxOverlap
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func endsWithSentenceTerminator(s string) bool {
	if len(s) == 0 {
		return false
	}

	terminators := []rune{'.', '!', '?', '。', '！', '？'}
	lastChar := rune(s[len(s)-1])

	for _, term := range terminators {
		if lastChar == term {
			return true
		}
	}

	return false
}

func countOccurrences(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}

	count := 0
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			count++
			i += len(substr) - 1
		}
	}

	return count
}
