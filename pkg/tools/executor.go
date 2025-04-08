package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// Executor 工具执行器，负责执行工具调用
type Executor struct {
	registry *Registry
}

// Registry 工具注册表，用于管理和查找工具
type Registry struct {
	tools map[string]Tool
}

// NewRegistry 创建一个新的工具注册表
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register 注册一个工具
func (r *Registry) Register(tool Tool) error {
	if tool == nil {
		return errors.New("tool cannot be nil")
	}

	id := tool.GetID()
	if id == "" {
		return errors.New("tool ID cannot be empty")
	}

	if _, exists := r.tools[id]; exists {
		return fmt.Errorf("tool with ID '%s' already registered", id)
	}

	r.tools[id] = tool
	return nil
}

// Get 获取一个工具
func (r *Registry) Get(id string) (Tool, error) {
	tool, exists := r.tools[id]
	if !exists {
		return nil, fmt.Errorf("tool with ID '%s' not found", id)
	}
	return tool, nil
}

// List 列出所有工具
func (r *Registry) List() []Tool {
	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// NewExecutor 创建一个新的工具执行器
func NewExecutor(registry *Registry) *Executor {
	if registry == nil {
		registry = NewRegistry()
	}
	return &Executor{
		registry: registry,
	}
}

// ExecuteByName 根据工具名称执行工具
func (e *Executor) ExecuteByName(ctx context.Context, toolName string, params map[string]interface{}, options *ExecuteOptions) (interface{}, error) {
	// 查找工具
	tool, err := e.registry.Get(toolName)
	if err != nil {
		return nil, err
	}

	// 执行工具
	return e.Execute(ctx, tool, params, options)
}

// Execute 执行工具
func (e *Executor) Execute(ctx context.Context, tool Tool, params map[string]interface{}, options *ExecuteOptions) (interface{}, error) {
	if tool == nil {
		return nil, errors.New("tool cannot be nil")
	}

	if options == nil {
		options = &ExecuteOptions{}
	}

	// 验证参数
	schema := tool.GetInputSchema()
	if schema != nil {
		if err := schema.Validate(params); err != nil {
			return nil, fmt.Errorf("validation error: %w", err)
		}
	}

	// 执行工具
	return tool.Execute(params, options)
}

// SimpleSchema 提供基本的Schema实现
type SimpleSchema struct {
	SchemaType    JSONSchemaType
	Properties    map[string]Schema
	Required      []string
	Description   string
	SchemaExample interface{}
}

// Validate 验证参数是否符合架构
func (s *SimpleSchema) Validate(params map[string]interface{}) error {
	if s.SchemaType == TypeObject && s.Properties != nil {
		// 检查必填字段
		for _, req := range s.Required {
			if _, ok := params[req]; !ok {
				return NewValidationError(fmt.Sprintf("missing required field: %s", req))
			}
		}

		// 验证每个属性
		for name, prop := range s.Properties {
			if val, ok := params[name]; ok {
				// 如果有值，验证它
				if propSchema, ok := prop.(*SimpleSchema); ok {
					// 对于对象或数组类型的属性，递归验证
					if propSchema.SchemaType == TypeObject {
						if objMap, ok := val.(map[string]interface{}); ok {
							if err := propSchema.Validate(objMap); err != nil {
								return NewValidationError(fmt.Sprintf("invalid field '%s': %v", name, err))
							}
						} else {
							return NewValidationError(fmt.Sprintf("field '%s' must be an object", name))
						}
					} else if propSchema.SchemaType == TypeArray {
						// 简单验证数组类型，这里可以扩展更复杂的验证
						if _, ok := val.([]interface{}); !ok {
							return NewValidationError(fmt.Sprintf("field '%s' must be an array", name))
						}
					} else {
						// 验证基本类型
						switch propSchema.SchemaType {
						case TypeString:
							if _, ok := val.(string); !ok {
								return NewValidationError(fmt.Sprintf("field '%s' must be a string", name))
							}
						case TypeNumber:
							switch val.(type) {
							case float64, float32, int, int64, int32:
								// 有效的数字类型
							default:
								return NewValidationError(fmt.Sprintf("field '%s' must be a number", name))
							}
						case TypeInteger:
							switch val.(type) {
							case int, int64, int32:
								// 有效的整数类型
							default:
								return NewValidationError(fmt.Sprintf("field '%s' must be an integer", name))
							}
						case TypeBoolean:
							if _, ok := val.(bool); !ok {
								return NewValidationError(fmt.Sprintf("field '%s' must be a boolean", name))
							}
						}
					}
				}
			}
		}
	}
	return nil
}

// JSONSchema 返回Schema的JSON表示
func (s *SimpleSchema) JSONSchema() (map[string]interface{}, error) {
	schema := map[string]interface{}{
		"type": s.SchemaType,
	}

	if s.Description != "" {
		schema["description"] = s.Description
	}

	if s.SchemaExample != nil {
		schema["example"] = s.SchemaExample
	}

	if s.SchemaType == TypeObject && s.Properties != nil {
		props := make(map[string]interface{})
		for name, prop := range s.Properties {
			propSchema, err := prop.JSONSchema()
			if err != nil {
				return nil, err
			}
			props[name] = propSchema
		}
		schema["properties"] = props

		if len(s.Required) > 0 {
			schema["required"] = s.Required
		}
	}

	return schema, nil
}

// NewSimpleSchema 创建一个新的简单Schema
func NewSimpleSchema(schemaType JSONSchemaType, description string) *SimpleSchema {
	return &SimpleSchema{
		SchemaType:  schemaType,
		Properties:  make(map[string]Schema),
		Description: description,
	}
}

// AddProperty 添加属性
func (s *SimpleSchema) AddProperty(name string, property Schema, required bool) *SimpleSchema {
	s.Properties[name] = property
	if required {
		s.Required = append(s.Required, name)
	}
	return s
}

// SetExample 设置示例
func (s *SimpleSchema) SetExample(example interface{}) *SimpleSchema {
	s.SchemaExample = example
	return s
}

// ParseToolCallJSON 从JSON字符串解析工具调用
func ParseToolCallJSON(jsonStr string) (toolName string, params map[string]interface{}, err error) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return "", nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	toolNameVal, ok := data["name"]
	if !ok {
		return "", nil, errors.New("missing 'name' field in tool call")
	}

	toolName, ok = toolNameVal.(string)
	if !ok {
		return "", nil, errors.New("'name' field must be a string")
	}

	paramsVal, ok := data["arguments"]
	if !ok {
		return toolName, make(map[string]interface{}), nil
	}

	params, ok = paramsVal.(map[string]interface{})
	if !ok {
		// 尝试解析参数字符串
		if paramsStr, ok := paramsVal.(string); ok {
			if err := json.Unmarshal([]byte(paramsStr), &params); err != nil {
				return toolName, make(map[string]interface{}), nil
			}
			return toolName, params, nil
		}
		return toolName, make(map[string]interface{}), nil
	}

	return toolName, params, nil
}
