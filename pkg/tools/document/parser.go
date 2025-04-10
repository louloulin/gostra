package document

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/louloulin/gostra/pkg/tools"
)

// ParserOptions represents options for the document parser
type ParserOptions struct {
	// Supported document formats
	SupportedFormats []string
}

// Document represents a parsed document
type Document struct {
	// Original filename
	Filename string `json:"filename"`
	// Document content type
	ContentType string `json:"content_type"`
	// Extracted text content
	Content string `json:"content"`
	// Document metadata
	Metadata map[string]interface{} `json:"metadata"`
	// Page count (if applicable)
	PageCount int `json:"page_count,omitempty"`
}

// DocumentParser is a tool for parsing documents
type DocumentParser struct {
	options ParserOptions
	id      string
	desc    string
	schema  tools.Schema
}

// NewDocumentParser creates a new document parser tool
func NewDocumentParser(options ParserOptions) *DocumentParser {
	supportedFormats := options.SupportedFormats
	if len(supportedFormats) == 0 {
		// Default supported formats
		supportedFormats = []string{"text/plain", "application/pdf", "text/html"}
	}

	// Create schema using SimpleSchema
	schema := tools.NewSimpleSchema(tools.TypeObject, "Document parser parameters")

	// Add file_path property
	filePathSchema := tools.NewSimpleSchema(tools.TypeString, "Path to the document file")
	schema.AddProperty("file_path", filePathSchema, false)

	// Add url property
	urlSchema := tools.NewSimpleSchema(tools.TypeString, "URL to download the document from")
	schema.AddProperty("url", urlSchema, false)

	// Add extract_metadata property
	extractMetadataSchema := tools.NewSimpleSchema(tools.TypeBoolean, "Whether to extract metadata from the document")
	schema.AddProperty("extract_metadata", extractMetadataSchema, false)

	return &DocumentParser{
		options: ParserOptions{
			SupportedFormats: supportedFormats,
		},
		id:     "document_parser",
		desc:   "Parses documents and extracts text and metadata",
		schema: schema,
	}
}

// GetID returns the tool ID
func (p *DocumentParser) GetID() string {
	return p.id
}

// GetDescription returns the tool description
func (p *DocumentParser) GetDescription() string {
	return p.desc
}

// GetInputSchema returns the input schema
func (p *DocumentParser) GetInputSchema() tools.Schema {
	return p.schema
}

// Execute executes the tool
func (p *DocumentParser) Execute(params map[string]interface{}, options *tools.ExecuteOptions) (interface{}, error) {
	// Create context from options if available
	// Note: ctx could be used for cancellation in a more advanced implementation
	_ = context.Background()
	if options != nil && options.Context != nil {
		if _, ok := options.Context.(context.Context); ok {
			// ctx = ctxVal // We could use this for operations that support context
		}
	}

	filePath, hasFilePath := params["file_path"].(string)
	url, hasURL := params["url"].(string)
	extractMetadata := true
	if val, ok := params["extract_metadata"].(bool); ok {
		extractMetadata = val
	}

	var content []byte
	var filename string
	var contentType string
	var err error

	// Get content from file or URL
	if hasFilePath {
		content, contentType, err = p.readFromFile(filePath)
		filename = filepath.Base(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}
	} else if hasURL {
		content, contentType, filename, err = p.downloadFromURL(url)
		if err != nil {
			return nil, fmt.Errorf("failed to download from URL: %w", err)
		}
	} else {
		return nil, errors.New("either file_path or url must be provided")
	}

	// Check if content type is supported
	supported := false
	for _, format := range p.options.SupportedFormats {
		if format == contentType {
			supported = true
			break
		}
	}

	if !supported {
		return nil, fmt.Errorf("unsupported document format: %s", contentType)
	}

	// Parse document content
	doc, err := p.parseDocument(content, filename, contentType, extractMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to parse document: %w", err)
	}

	return doc, nil
}

// readFromFile reads content from a file and detects its content type
func (p *DocumentParser) readFromFile(filePath string) ([]byte, string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, "", err
	}

	// Detect content type
	contentType := http.DetectContentType(content)
	return content, contentType, nil
}

// downloadFromURL downloads content from a URL
func (p *DocumentParser) downloadFromURL(url string) ([]byte, string, string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", "", fmt.Errorf("failed to download: HTTP status %d", resp.StatusCode)
	}

	// Get content type
	contentType := resp.Header.Get("Content-Type")

	// Get filename from Content-Disposition or URL
	filename := ""
	if disposition := resp.Header.Get("Content-Disposition"); disposition != "" {
		if _, params, err := mime.ParseMediaType(disposition); err == nil {
			if fn, ok := params["filename"]; ok {
				filename = fn
			}
		}
	}

	if filename == "" {
		urlPath := strings.Split(url, "/")
		if len(urlPath) > 0 {
			filename = urlPath[len(urlPath)-1]
		} else {
			filename = "downloaded_document"
		}
	}

	// Read response body
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", "", err
	}

	return content, contentType, filename, nil
}

// parseDocument parses document content into a Document struct
func (p *DocumentParser) parseDocument(content []byte, filename, contentType string, extractMetadata bool) (*Document, error) {
	doc := &Document{
		Filename:    filename,
		ContentType: contentType,
		Metadata:    make(map[string]interface{}),
	}

	switch contentType {
	case "text/plain":
		doc.Content = string(content)
		doc.PageCount = 1
	case "application/pdf":
		// Basic PDF parsing - in a real implementation, use a PDF library
		doc.Content = string(content) // This is a simplification
		doc.PageCount = 1             // Placeholder
		if extractMetadata {
			// Extract PDF metadata (simplified)
			doc.Metadata["format"] = "PDF"
		}
	case "text/html":
		// Basic HTML parsing - in a real implementation, use an HTML parser
		doc.Content = string(content)
		doc.PageCount = 1
		if extractMetadata {
			doc.Metadata["format"] = "HTML"
		}
	default:
		return nil, fmt.Errorf("parsing not implemented for content type: %s", contentType)
	}

	return doc, nil
}
