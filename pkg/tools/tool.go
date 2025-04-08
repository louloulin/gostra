package tools

import (
	"errors"
)

// ExecuteOptions 定义工具执行选项
type ExecuteOptions struct {
	ThreadID   string      `json:"thread_id,omitempty"`
	ResourceID string      `json:"resource_id,omitempty"`
	CallID     string      `json:"call_id,omitempty"`
	Context    interface{} `json:"context,omitempty"`
}

// Schema 表示工具输入架构的接口
type Schema interface {
	// Validate 验证参数是否符合架构
	Validate(params map[string]interface{}) error
	// JSONSchema 返回Schema的JSON表示
	JSONSchema() (map[string]interface{}, error)
}

// Tool 定义工具的基本接口
type Tool interface {
	// GetID 返回工具ID
	GetID() string
	// GetDescription 返回工具描述
	GetDescription() string
	// GetInputSchema 返回输入架构
	GetInputSchema() Schema
	// Execute 执行工具
	Execute(params map[string]interface{}, options *ExecuteOptions) (interface{}, error)
}

// BasicTool 提供Tool接口的基本实现
type BasicTool struct {
	ID          string
	Description string
	InputSchema Schema
	ExecuteFunc func(params map[string]interface{}, options *ExecuteOptions) (interface{}, error)
}

// GetID 返回工具ID
func (t *BasicTool) GetID() string {
	return t.ID
}

// GetDescription 返回工具描述
func (t *BasicTool) GetDescription() string {
	return t.Description
}

// GetInputSchema 返回输入架构
func (t *BasicTool) GetInputSchema() Schema {
	return t.InputSchema
}

// Execute 执行工具
func (t *BasicTool) Execute(params map[string]interface{}, options *ExecuteOptions) (interface{}, error) {
	if t.ExecuteFunc == nil {
		return nil, errors.New("execute function not set")
	}

	// 验证参数
	if t.InputSchema != nil {
		if err := t.InputSchema.Validate(params); err != nil {
			return nil, err
		}
	}

	return t.ExecuteFunc(params, options)
}

// NewBasicTool 创建一个新的基本工具
func NewBasicTool(id, description string, schema Schema, execFunc func(params map[string]interface{}, options *ExecuteOptions) (interface{}, error)) *BasicTool {
	return &BasicTool{
		ID:          id,
		Description: description,
		InputSchema: schema,
		ExecuteFunc: execFunc,
	}
}

// JSONSchemaType 表示JSON Schema中的类型
type JSONSchemaType string

const (
	TypeString  JSONSchemaType = "string"
	TypeNumber  JSONSchemaType = "number"
	TypeInteger JSONSchemaType = "integer"
	TypeBoolean JSONSchemaType = "boolean"
	TypeObject  JSONSchemaType = "object"
	TypeArray   JSONSchemaType = "array"
	TypeNull    JSONSchemaType = "null"
)

// ValidationError 表示验证错误
type ValidationError struct {
	Message string
}

// Error 实现error接口
func (e *ValidationError) Error() string {
	return e.Message
}

// NewValidationError 创建一个新的验证错误
func NewValidationError(message string) *ValidationError {
	return &ValidationError{
		Message: message,
	}
}
