package document

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/yourusername/gostra/pkg/tools"
)

// ChunkStrategy 定义文档分块策略
type ChunkStrategy string

const (
	// StrategyRecursive 递归分块策略，根据分隔符分割
	StrategyRecursive ChunkStrategy = "recursive"
	// StrategyFixed 固定大小分块策略
	StrategyFixed ChunkStrategy = "fixed"
	// StrategySentence 句子分块策略
	StrategySentence ChunkStrategy = "sentence"
	// StrategyParagraph 段落分块策略
	StrategyParagraph ChunkStrategy = "paragraph"
)

// ChunkParams 表示文档分块参数
type ChunkParams struct {
	// 分块策略
	Strategy ChunkStrategy `json:"strategy"`
	// 块大小（字符数）
	Size int `json:"size"`
	// 块重叠（字符数）
	Overlap int `json:"overlap"`
	// 分隔符（用于递归策略）
	Separator string `json:"separator"`
}

// DocumentChunk 表示文档的一个块
type DocumentChunk struct {
	// 块内容
	Content string `json:"content"`
	// 块在原文中的位置
	Position int `json:"position"`
	// 所属文档ID（如果有）
	DocumentID string `json:"document_id,omitempty"`
	// 块元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// DocumentChunkerOptions 定义文档分块工具的选项
type DocumentChunkerOptions struct {
	// 默认分块参数
	DefaultParams ChunkParams
}

// DocumentChunker 文档分块工具
type DocumentChunker struct {
	options DocumentChunkerOptions
	id      string
	desc    string
	schema  tools.Schema
}

// NewDocumentChunker 创建新的文档分块工具
func NewDocumentChunker(options DocumentChunkerOptions) *DocumentChunker {
	// 设置默认参数
	if options.DefaultParams.Strategy == "" {
		options.DefaultParams.Strategy = StrategyRecursive
	}
	if options.DefaultParams.Size == 0 {
		options.DefaultParams.Size = 1000
	}
	if options.DefaultParams.Separator == "" {
		options.DefaultParams.Separator = "\n"
	}

	// 创建输入模式
	schema := tools.NewSimpleSchema(tools.TypeObject, "文档分块参数")

	contentSchema := tools.NewSimpleSchema(tools.TypeString, "要分块的文档内容")
	schema.AddProperty("content", contentSchema, true)

	strategySchema := tools.NewSimpleSchema(tools.TypeString, "分块策略：recursive, fixed, sentence, paragraph")
	schema.AddProperty("strategy", strategySchema, false)

	sizeSchema := tools.NewSimpleSchema(tools.TypeInteger, "块大小（字符数）")
	schema.AddProperty("size", sizeSchema, false)

	overlapSchema := tools.NewSimpleSchema(tools.TypeInteger, "块重叠（字符数）")
	schema.AddProperty("overlap", overlapSchema, false)

	separatorSchema := tools.NewSimpleSchema(tools.TypeString, "分隔符（用于递归策略）")
	schema.AddProperty("separator", separatorSchema, false)

	return &DocumentChunker{
		options: options,
		id:      "document_chunker",
		desc:    "将文档分割成较小的块，用于进一步处理或嵌入",
		schema:  schema,
	}
}

// GetID 返回工具ID
func (c *DocumentChunker) GetID() string {
	return c.id
}

// GetDescription 返回工具描述
func (c *DocumentChunker) GetDescription() string {
	return c.desc
}

// GetInputSchema 返回输入架构
func (c *DocumentChunker) GetInputSchema() tools.Schema {
	return c.schema
}

// Execute 执行工具
func (c *DocumentChunker) Execute(params map[string]interface{}, options *tools.ExecuteOptions) (interface{}, error) {
	// 获取文档内容
	content, ok := params["content"].(string)
	if !ok || content == "" {
		return nil, errors.New("必须提供非空的文档内容")
	}

	// 设置分块参数
	chunkParams := c.options.DefaultParams

	// 覆盖默认参数
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

	// 根据策略分块
	chunks, err := c.chunkDocument(content, chunkParams)
	if err != nil {
		return nil, fmt.Errorf("分块文档失败: %w", err)
	}

	return chunks, nil
}

// chunkDocument 根据策略将文档分块
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
		return nil, fmt.Errorf("不支持的分块策略: %s", params.Strategy)
	}
}

// chunkRecursive 递归分块，先按分隔符分割，如果块太大则继续分割
func (c *DocumentChunker) chunkRecursive(content string, size int, overlap int, separator string) ([]*DocumentChunk, error) {
	if content == "" {
		return []*DocumentChunk{}, nil
	}

	// 定义分隔符列表，优先级从高到低
	separators := []string{separator}
	if separator != "\n" {
		separators = append(separators, "\n")
	}
	separators = append(separators, ". ", "! ", "? ", "；", "，", " ", "")

	return c.splitRecursive(content, size, overlap, separators, 0), nil
}

// splitRecursive 递归分割，如果当前分隔符无法满足大小要求，则尝试下一个分隔符
func (c *DocumentChunker) splitRecursive(content string, size int, overlap int, separators []string, position int) []*DocumentChunk {
	// 如果内容已经小于块大小，直接返回
	if len(content) <= size {
		return []*DocumentChunk{
			{
				Content:  content,
				Position: position,
			},
		}
	}

	// 如果没有更多分隔符，直接按大小分割
	if len(separators) == 0 || separators[0] == "" {
		chunks, _ := c.chunkFixed(content, size, overlap)
		return chunks
	}

	// 尝试当前分隔符
	separator := separators[0]
	parts := strings.Split(content, separator)

	// 如果分隔符不能分割文本，尝试下一个分隔符
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

		// 添加分隔符（除了空字符串）
		partWithSep := part
		if separator != "" {
			partWithSep = part + separator
		}

		// 如果当前块加上新部分超过大小，则需要进一步处理
		if len(currentChunk)+len(partWithSep) > size {
			// 如果当前块非空，添加到结果
			if currentChunk != "" {
				chunks = append(chunks, &DocumentChunk{
					Content:  currentChunk,
					Position: currentPosition,
				})
				currentPosition += len(currentChunk) - overlap
			}

			// 如果单个部分超过大小，递归处理
			if len(partWithSep) > size {
				subChunks := c.splitRecursive(partWithSep, size, overlap, separators[1:], currentPosition+overlap)
				chunks = append(chunks, subChunks...)
				currentPosition = subChunks[len(subChunks)-1].Position + len(subChunks[len(subChunks)-1].Content)
			} else {
				// 开始新块
				currentChunk = partWithSep
			}
		} else {
			// 添加到当前块
			currentChunk += partWithSep
		}
	}

	// 添加最后一个块
	if currentChunk != "" {
		chunks = append(chunks, &DocumentChunk{
			Content:  currentChunk,
			Position: currentPosition,
		})
	}

	return chunks
}

// chunkFixed 按固定大小分块
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

		// 如果已经处理到末尾，跳出循环
		if end == contentLength {
			break
		}
	}

	return chunks, nil
}

// chunkBySentence 按句子分块
func (c *DocumentChunker) chunkBySentence(content string, size int, overlap int) ([]*DocumentChunk, error) {
	// 句子结束符正则表达式
	re := regexp.MustCompile(`[.!?。！？]+\s*`)
	sentences := re.Split(content, -1)

	// 根据句子构建块
	chunks := []*DocumentChunk{}
	currentChunk := ""
	currentPosition := 0

	for _, sentence := range sentences {
		if sentence == "" {
			continue
		}

		// 添加句子结束符
		sentenceWithEnd := sentence + ". "

		// 如果当前块加上新句子超过大小，开始新块
		if len(currentChunk)+len(sentenceWithEnd) > size && currentChunk != "" {
			chunks = append(chunks, &DocumentChunk{
				Content:  currentChunk,
				Position: currentPosition,
			})

			// 考虑重叠，计算新块的起始位置
			if overlap > 0 && len(currentChunk) > overlap {
				// 找到重叠处的句子开始
				currentPosition += len(currentChunk) - overlap
				overlapText := currentChunk[len(currentChunk)-overlap:]

				// 如果重叠处不是句子开始，寻找最接近的句子开始
				sentenceStart := re.Split(overlapText, -1)
				if len(sentenceStart) > 1 {
					// 使用最后一个完整的句子
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

		// 添加到当前块
		currentChunk += sentenceWithEnd
	}

	// 添加最后一个块
	if currentChunk != "" {
		chunks = append(chunks, &DocumentChunk{
			Content:  currentChunk,
			Position: currentPosition,
		})
	}

	return chunks, nil
}

// chunkByParagraph 按段落分块
func (c *DocumentChunker) chunkByParagraph(content string, size int, overlap int) ([]*DocumentChunk, error) {
	// 段落分隔符
	paragraphs := strings.Split(content, "\n\n")

	// 根据段落构建块
	chunks := []*DocumentChunk{}
	currentChunk := ""
	currentPosition := 0

	for _, paragraph := range paragraphs {
		if paragraph == "" {
			continue
		}

		// 添加段落分隔符
		paragraphWithEnd := paragraph + "\n\n"

		// 如果当前块加上新段落超过大小，开始新块
		if len(currentChunk)+len(paragraphWithEnd) > size && currentChunk != "" {
			chunks = append(chunks, &DocumentChunk{
				Content:  currentChunk,
				Position: currentPosition,
			})

			// 考虑重叠
			if overlap > 0 && len(currentChunk) > overlap {
				currentPosition += len(currentChunk) - overlap

				// 找到重叠处的段落开始
				overlapText := currentChunk[len(currentChunk)-overlap:]
				paragraphStart := strings.Split(overlapText, "\n\n")

				if len(paragraphStart) > 1 {
					// 使用最后一个完整的段落
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

		// 大段落可能需要进一步拆分
		if len(paragraphWithEnd) > size {
			// 如果当前块不为空，先添加
			if currentChunk != "" {
				chunks = append(chunks, &DocumentChunk{
					Content:  currentChunk,
					Position: currentPosition,
				})
				currentPosition += len(currentChunk)
				currentChunk = ""
			}

			// 按固定大小分块处理大段落
			innerChunks, err := c.chunkFixed(paragraphWithEnd, size, overlap)
			if err != nil {
				return nil, err
			}

			for _, innerChunk := range innerChunks {
				innerChunk.Position += currentPosition
			}
			chunks = append(chunks, innerChunks...)

			// 更新位置
			if len(innerChunks) > 0 {
				lastChunk := innerChunks[len(innerChunks)-1]
				currentPosition = lastChunk.Position + len(lastChunk.Content)
			}
		} else {
			// 正常添加到当前块
			currentChunk += paragraphWithEnd
		}
	}

	// 添加最后一个块
	if currentChunk != "" {
		chunks = append(chunks, &DocumentChunk{
			Content:  currentChunk,
			Position: currentPosition,
		})
	}

	return chunks, nil
}
