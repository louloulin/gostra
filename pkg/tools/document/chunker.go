package document

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/louloulin/gostra/pkg/tools"
)

// ChunkStrategy defines the strategy for chunking documents
type ChunkStrategy string

const (
	// StrategyFixed chunks text into fixed-size segments
	StrategyFixed ChunkStrategy = "fixed"

	// StrategyRecursive uses a recursive algorithm to split text
	StrategyRecursive ChunkStrategy = "recursive"

	// StrategyParagraph splits by paragraphs
	StrategyParagraph ChunkStrategy = "paragraph"

	// StrategySentence splits by sentences
	StrategySentence ChunkStrategy = "sentence"
)

// ChunkParams represents the parameters for chunking a document
type ChunkParams struct {
	// Strategy is the chunking strategy
	Strategy ChunkStrategy `json:"strategy"`
	// Size is the size of each chunk
	Size int `json:"size"`
	// Overlap is the overlap between chunks
	Overlap int `json:"overlap"`
	// Separator is the separator used for recursive strategy
	Separator string `json:"separator"`
}

// DocumentChunk represents a chunk of a document
type DocumentChunk struct {
	// Content is the text content of the chunk
	Content string `json:"content"`

	// DocumentID is the ID of the original document
	DocumentID string `json:"document_id,omitempty"`

	// ChunkIndex is the index of this chunk in the original document
	ChunkIndex int `json:"chunk_index"`

	// Position is the character position in the original document
	Position int `json:"position"`

	// Metadata contains additional information about the chunk
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// DocumentChunkerOptions defines the options for the document chunker
type DocumentChunkerOptions struct {
	// DefaultParams is the default chunking parameters
	DefaultParams ChunkParams
}

// DocumentChunker is the tool for chunking documents
type DocumentChunker struct {
	options DocumentChunkerOptions
	id      string
	desc    string
	schema  tools.Schema
}

// NewDocumentChunker creates a new document chunker
func NewDocumentChunker(options DocumentChunkerOptions) *DocumentChunker {
	// Set default parameters
	if options.DefaultParams.Strategy == "" {
		options.DefaultParams.Strategy = StrategyRecursive
	}
	if options.DefaultParams.Size == 0 {
		options.DefaultParams.Size = 1000
	}
	if options.DefaultParams.Separator == "" {
		options.DefaultParams.Separator = "\n"
	}

	// Create input schema
	schema := tools.NewSimpleSchema(tools.TypeObject, "Document chunking parameters")

	contentSchema := tools.NewSimpleSchema(tools.TypeString, "Content of the document to be chunked")
	schema.AddProperty("content", contentSchema, true)

	strategySchema := tools.NewSimpleSchema(tools.TypeString, "Chunking strategy: recursive, fixed, sentence, paragraph")
	schema.AddProperty("strategy", strategySchema, false)

	sizeSchema := tools.NewSimpleSchema(tools.TypeInteger, "Chunk size (in characters)")
	schema.AddProperty("size", sizeSchema, false)

	overlapSchema := tools.NewSimpleSchema(tools.TypeInteger, "Chunk overlap (in characters)")
	schema.AddProperty("overlap", overlapSchema, false)

	separatorSchema := tools.NewSimpleSchema(tools.TypeString, "Separator (for recursive strategy)")
	schema.AddProperty("separator", separatorSchema, false)

	return &DocumentChunker{
		options: options,
		id:      "document_chunker",
		desc:    "Splits a document into smaller chunks for further processing or embedding",
		schema:  schema,
	}
}

// GetID returns the tool ID
func (c *DocumentChunker) GetID() string {
	return c.id
}

// GetDescription returns the tool description
func (c *DocumentChunker) GetDescription() string {
	return c.desc
}

// GetInputSchema returns the input schema
func (c *DocumentChunker) GetInputSchema() tools.Schema {
	return c.schema
}

// Execute executes the tool
func (c *DocumentChunker) Execute(params map[string]interface{}, options *tools.ExecuteOptions) (interface{}, error) {
	// Get document content
	content, ok := params["content"].(string)
	if !ok || content == "" {
		return nil, errors.New("must provide non-empty document content")
	}

	// Set chunking parameters
	chunkParams := c.options.DefaultParams

	// Override default parameters
	if strategy, ok := params["strategy"].(string); ok && strategy != "" {
		chunkParams.Strategy = ChunkStrategy(strategy)
	}
	if size, ok := params["size"].(float64); ok && size > 0 {
		chunkParams.Size = int(size)
	}
	if overlap, ok := params["overlap"].(float64); ok && overlap >= 0 {
		chunkParams.Overlap = int(overlap)
	}
	if separator, ok := params["separator"].(string); ok {
		chunkParams.Separator = separator
	}

	// Chunk document based on strategy
	chunks, err := c.chunkDocument(content, chunkParams)
	if err != nil {
		return nil, fmt.Errorf("chunking document failed: %w", err)
	}

	return chunks, nil
}

// chunkDocument chunks the document based on the strategy
func (c *DocumentChunker) chunkDocument(content string, params ChunkParams) ([]*DocumentChunk, error) {
	switch params.Strategy {
	case StrategyRecursive:
		return c.chunkRecursive(content, params.Size, params.Overlap, params.Separator)
	case StrategyFixed:
		return c.chunkFixed(content, params.Size, params.Overlap)
	case StrategySentence:
		return c.chunkBySentence(content, params.Size, params.Overlap)
	case StrategyParagraph:
		return c.chunkByParagraph(content, params.Size, params.Overlap)
	default:
		return nil, fmt.Errorf("unsupported chunking strategy: %s", params.Strategy)
	}
}

// chunkRecursive recursively chunks text, splitting by separator
func (c *DocumentChunker) chunkRecursive(content string, size int, overlap int, separator string) ([]*DocumentChunk, error) {
	if content == "" {
		return []*DocumentChunk{}, nil
	}

	// Define separator list, with priority from high to low
	separators := []string{separator}
	if separator != "\n" {
		separators = append(separators, "\n")
	}
	separators = append(separators, ". ", "! ", "? ", "；", "，", " ", "")

	return c.splitRecursive(content, size, overlap, separators, 0), nil
}

// splitRecursive recursively splits text, trying each separator
func (c *DocumentChunker) splitRecursive(content string, size int, overlap int, separators []string, position int) []*DocumentChunk {
	// If content is already smaller than chunk size, return
	if len(content) <= size {
		return []*DocumentChunk{
			{
				Content:  content,
				Position: position,
			},
		}
	}

	// If no more separators, split by size
	if len(separators) == 0 || separators[0] == "" {
		chunks, _ := c.chunkFixed(content, size, overlap)
		return chunks
	}

	// Try current separator
	separator := separators[0]
	parts := strings.Split(content, separator)

	// If separator cannot split text, try next separator
	if len(parts) == 1 {
		return c.splitRecursive(content, size, overlap, separators[1:], position)
	}

	chunks := []*DocumentChunk{}
	currentChunk := ""
	currentPosition := position

	for _, part := range parts {
		if part == "" {
			continue
		}

		// Add separator (except empty string)
		partWithSep := part
		if separator != "" {
			partWithSep = part + separator
		}

		// If current chunk plus new part exceeds size, need further processing
		if len(currentChunk)+len(partWithSep) > size {
			// If current chunk is not empty, add to result
			if currentChunk != "" {
				chunks = append(chunks, &DocumentChunk{
					Content:  currentChunk,
					Position: currentPosition,
				})
				currentPosition += len(currentChunk) - overlap
			}

			// If single part exceeds size, recursively process
			if len(partWithSep) > size {
				subChunks := c.splitRecursive(partWithSep, size, overlap, separators[1:], currentPosition+overlap)
				chunks = append(chunks, subChunks...)
				currentPosition = subChunks[len(subChunks)-1].Position + len(subChunks[len(subChunks)-1].Content)
			} else {
				// Start new chunk
				currentChunk = partWithSep
			}
		} else {
			// Add to current chunk
			currentChunk += partWithSep
		}
	}

	// Add last chunk
	if currentChunk != "" {
		chunks = append(chunks, &DocumentChunk{
			Content:  currentChunk,
			Position: currentPosition,
		})
	}

	return chunks
}

// chunkFixed chunks text into fixed-size segments
func (c *DocumentChunker) chunkFixed(content string, size int, overlap int) ([]*DocumentChunk, error) {
	if content == "" {
		return []*DocumentChunk{}, nil
	}

	chunks := []*DocumentChunk{}
	contentLength := len(content)
	stride := size - overlap

	for i := 0; i < contentLength; i += stride {
		end := int(math.Min(float64(i+size), float64(contentLength)))
		chunkContent := content[i:end]

		chunks = append(chunks, &DocumentChunk{
			Content:  chunkContent,
			Position: i,
		})

		// If already processed to end, break loop
		if end == contentLength {
			break
		}
	}

	return chunks, nil
}

// chunkBySentence chunks text by sentences
func (c *DocumentChunker) chunkBySentence(content string, size int, overlap int) ([]*DocumentChunk, error) {
	// Sentence end regex
	re := regexp.MustCompile(`[.!?。！？]+\s*`)
	sentences := re.Split(content, -1)

	// Build chunks based on sentences
	chunks := []*DocumentChunk{}
	currentChunk := ""
	currentPosition := 0

	for _, sentence := range sentences {
		if sentence == "" {
			continue
		}

		// Add sentence end
		sentenceWithEnd := sentence + ". "

		// If current chunk plus new sentence exceeds size, start new chunk
		if len(currentChunk)+len(sentenceWithEnd) > size && currentChunk != "" {
			chunks = append(chunks, &DocumentChunk{
				Content:  currentChunk,
				Position: currentPosition,
			})

			// Consider overlap, calculate new chunk start position
			if overlap > 0 && len(currentChunk) > overlap {
				// Find sentence start at overlap
				currentPosition += len(currentChunk) - overlap
				overlapText := currentChunk[len(currentChunk)-overlap:]

				// If overlap is not sentence start, find closest sentence start
				sentenceStart := re.Split(overlapText, -1)
				if len(sentenceStart) > 1 {
					// Use last complete sentence
					currentPosition += len(overlapText) - len(sentenceStart[len(sentenceStart)-1])
					currentChunk = sentenceStart[len(sentenceStart)-1]
				} else {
					currentChunk = ""
				}
			} else {
				currentPosition += len(currentChunk)
				currentChunk = ""
			}
		}

		// Add to current chunk
		currentChunk += sentenceWithEnd
	}

	// Add last chunk
	if currentChunk != "" {
		chunks = append(chunks, &DocumentChunk{
			Content:  currentChunk,
			Position: currentPosition,
		})
	}

	return chunks, nil
}

// chunkByParagraph chunks text by paragraphs
func (c *DocumentChunker) chunkByParagraph(content string, size int, overlap int) ([]*DocumentChunk, error) {
	// Paragraph separator
	paragraphs := strings.Split(content, "\n\n")

	// Build chunks based on paragraphs
	chunks := []*DocumentChunk{}
	currentChunk := ""
	currentPosition := 0

	for _, paragraph := range paragraphs {
		if paragraph == "" {
			continue
		}

		// Add paragraph separator
		paragraphWithEnd := paragraph + "\n\n"

		// If current chunk plus new paragraph exceeds size, start new chunk
		if len(currentChunk)+len(paragraphWithEnd) > size && currentChunk != "" {
			chunks = append(chunks, &DocumentChunk{
				Content:  currentChunk,
				Position: currentPosition,
			})

			// Consider overlap
			if overlap > 0 && len(currentChunk) > overlap {
				currentPosition += len(currentChunk) - overlap

				// Find paragraph start at overlap
				overlapText := currentChunk[len(currentChunk)-overlap:]
				paragraphStart := strings.Split(overlapText, "\n\n")

				if len(paragraphStart) > 1 {
					// Use last complete paragraph
					currentPosition += len(overlapText) - len(paragraphStart[len(paragraphStart)-1])
					currentChunk = paragraphStart[len(paragraphStart)-1]
				} else {
					currentChunk = ""
				}
			} else {
				currentPosition += len(currentChunk)
				currentChunk = ""
			}
		}

		// Large paragraph may need further splitting
		if len(paragraphWithEnd) > size {
			// If current chunk is not empty, add first
			if currentChunk != "" {
				chunks = append(chunks, &DocumentChunk{
					Content:  currentChunk,
					Position: currentPosition,
				})
				currentPosition += len(currentChunk)
				currentChunk = ""
			}

			// Process large paragraph by fixed-size chunks
			innerChunks, err := c.chunkFixed(paragraphWithEnd, size, overlap)
			if err != nil {
				return nil, err
			}

			for _, innerChunk := range innerChunks {
				innerChunk.Position += currentPosition
			}
			chunks = append(chunks, innerChunks...)

			// Update position
			if len(innerChunks) > 0 {
				lastChunk := innerChunks[len(innerChunks)-1]
				currentPosition = lastChunk.Position + len(lastChunk.Content)
			}
		} else {
			// Normal add to current chunk
			currentChunk += paragraphWithEnd
		}
	}

	// Add last chunk
	if currentChunk != "" {
		chunks = append(chunks, &DocumentChunk{
			Content:  currentChunk,
			Position: currentPosition,
		})
	}

	return chunks, nil
}
