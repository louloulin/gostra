package document

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDocumentParserGetID(t *testing.T) {
	parser := NewDocumentParser(ParserOptions{})
	if id := parser.GetID(); id != "document_parser" {
		t.Errorf("Expected tool ID 'document_parser', got '%s'", id)
	}
}

func TestDocumentParserGetDescription(t *testing.T) {
	parser := NewDocumentParser(ParserOptions{})
	if desc := parser.GetDescription(); desc == "" {
		t.Error("Tool description should not be empty")
	}
}

func TestDocumentParserGetInputSchema(t *testing.T) {
	parser := NewDocumentParser(ParserOptions{})
	schema := parser.GetInputSchema()

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
	requiredProps := []string{"file_path", "url", "extract_metadata"}
	for _, prop := range requiredProps {
		if _, exists := properties[prop]; !exists {
			t.Errorf("Schema should have '%s' property", prop)
		}
	}
}

func TestDocumentParserExecuteWithFilePath(t *testing.T) {
	// Create a temporary text file
	content := "This is a test document."
	tmpDir, err := os.MkdirTemp("", "document_parser_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	// Create parser
	parser := NewDocumentParser(ParserOptions{})

	// Execute with file path
	params := map[string]interface{}{
		"file_path": tmpFile,
	}

	result, err := parser.Execute(params, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Check result
	doc, ok := result.(*Document)
	if !ok {
		t.Fatal("Result should be of type *Document")
	}

	if doc.Content != content {
		t.Errorf("Expected content '%s', got '%s'", content, doc.Content)
	}

	if doc.ContentType != "text/plain; charset=utf-8" {
		t.Errorf("Expected content type 'text/plain; charset=utf-8', got '%s'", doc.ContentType)
	}

	if doc.PageCount != 1 {
		t.Errorf("Expected page count 1, got %d", doc.PageCount)
	}
}

func TestDocumentParserExecuteWithURL(t *testing.T) {
	// Create a test server
	content := "This is a test document from URL."
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Disposition", "attachment; filename=test.txt")
		w.Write([]byte(content))
	}))
	defer server.Close()

	// Create parser
	parser := NewDocumentParser(ParserOptions{})

	// Execute with URL
	params := map[string]interface{}{
		"url": server.URL,
	}

	result, err := parser.Execute(params, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Check result
	doc, ok := result.(*Document)
	if !ok {
		t.Fatal("Result should be of type *Document")
	}

	if doc.Content != content {
		t.Errorf("Expected content '%s', got '%s'", content, doc.Content)
	}

	if doc.Filename != "test.txt" {
		t.Errorf("Expected filename 'test.txt', got '%s'", doc.Filename)
	}
}

func TestDocumentParserUnsupportedFormat(t *testing.T) {
	// Create parser with limited formats
	parser := NewDocumentParser(ParserOptions{
		SupportedFormats: []string{"application/pdf"},
	})

	// Create a temporary text file
	content := "This is a test document."
	tmpDir, err := os.MkdirTemp("", "document_parser_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	// Execute with file path
	params := map[string]interface{}{
		"file_path": tmpFile,
	}

	_, err = parser.Execute(params, nil)
	if err == nil {
		t.Fatal("Expected error for unsupported format, got nil")
	}
}

func TestDocumentParserMissingParams(t *testing.T) {
	parser := NewDocumentParser(ParserOptions{})

	// Execute without file_path or url
	params := map[string]interface{}{}

	_, err := parser.Execute(params, nil)
	if err == nil {
		t.Fatal("Expected error for missing parameters, got nil")
	}
}

func TestDocumentParserInvalidFilePath(t *testing.T) {
	parser := NewDocumentParser(ParserOptions{})

	// Execute with non-existent file
	params := map[string]interface{}{
		"file_path": "/path/to/nonexistent/file.txt",
	}

	_, err := parser.Execute(params, nil)
	if err == nil {
		t.Fatal("Expected error for invalid file path, got nil")
	}
}
