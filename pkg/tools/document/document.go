package document

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/yourusername/gostra/pkg/tools/common"
)

// ProcessedDocument represents a processed document
type ProcessedDocument struct {
	ID       string                 `json:"id"`
	Content  string                 `json:"content"`
	Metadata map[string]interface{} `json:"metadata"`
	Chunks   []Chunk                `json:"chunks"`
}

// Chunk represents a segment of a document
type Chunk struct {
	ID       string                 `json:"id"`
	Content  string                 `json:"content"`
	Metadata map[string]interface{} `json:"metadata"`
}

// ChunkOptions defines options for document chunking
type ChunkOptions struct {
	Strategy common.ChunkStrategy `json:"strategy"` // recursive, fixed, sentence, etc.
	Size     int                  `json:"size"`     // chunk size in tokens/chars
	Overlap  int                  `json:"overlap"`  // overlap between chunks
}

// DocumentProcessor handles document processing operations
type DocumentProcessor struct {
	options ChunkOptions
}

// NewDocumentProcessor creates a new document processor
func NewDocumentProcessor(options ChunkOptions) *DocumentProcessor {
	if options.Size == 0 {
		options.Size = 512
	}
	return &DocumentProcessor{
		options: options,
	}
}

// LoadFromFile loads a document from a file
func (p *DocumentProcessor) LoadFromFile(path string) (*ProcessedDocument, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	doc := &ProcessedDocument{
		ID:      filepath.Base(path),
		Content: string(content),
		Metadata: map[string]interface{}{
			"source": path,
			"type":   "file",
		},
	}

	return doc, nil
}

// LoadFromURL loads a document from a URL
func (p *DocumentProcessor) LoadFromURL(ctx context.Context, url string) (*ProcessedDocument, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	doc := &ProcessedDocument{
		ID:      url,
		Content: string(content),
		Metadata: map[string]interface{}{
			"source": url,
			"type":   "url",
		},
	}

	return doc, nil
}

// Chunk splits a document into chunks based on the configured options
func (p *DocumentProcessor) Chunk(doc *ProcessedDocument) error {
	if doc == nil || doc.Content == "" {
		return fmt.Errorf("invalid document or empty content")
	}

	var chunks []Chunk
	content := doc.Content
	size := p.options.Size
	overlap := p.options.Overlap

	// Simple fixed-size chunking with overlap
	for i := 0; i < len(content); i += size - overlap {
		end := i + size
		if end > len(content) {
			end = len(content)
		}

		chunk := Chunk{
			ID:      fmt.Sprintf("%s-chunk-%d", doc.ID, len(chunks)),
			Content: content[i:end],
			Metadata: map[string]interface{}{
				"index":    len(chunks),
				"start":    i,
				"end":      end,
				"docID":    doc.ID,
				"strategy": p.options.Strategy,
			},
		}

		chunks = append(chunks, chunk)

		if end == len(content) {
			break
		}
	}

	doc.Chunks = chunks
	return nil
}

// Process processes a document from either a file or URL
func (p *DocumentProcessor) Process(ctx context.Context, source string) (*ProcessedDocument, error) {
	var doc *ProcessedDocument
	var err error

	if isURL(source) {
		doc, err = p.LoadFromURL(ctx, source)
	} else {
		doc, err = p.LoadFromFile(source)
	}

	if err != nil {
		return nil, err
	}

	err = p.Chunk(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to chunk document: %w", err)
	}

	return doc, nil
}

// isURL checks if a string is a URL
func isURL(str string) bool {
	return len(str) > 7 && (str[:7] == "http://" || str[:8] == "https://")
}
