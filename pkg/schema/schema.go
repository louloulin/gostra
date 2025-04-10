// Package schema provides data schema definitions for Gostra
package schema

// PropertyType represents the data type of a schema property
type PropertyType string

const (
	// TypeString represents string type
	TypeString PropertyType = "string"
	// TypeNumber represents number type
	TypeNumber PropertyType = "number"
	// TypeBoolean represents boolean type
	TypeBoolean PropertyType = "boolean"
	// TypeObject represents object type
	TypeObject PropertyType = "object"
	// TypeArray represents array type
	TypeArray PropertyType = "array"
	// TypeNull represents null type
	TypeNull PropertyType = "null"
)

// Property represents a schema property
type Property struct {
	Type        PropertyType         `json:"type"`
	Description string               `json:"description,omitempty"`
	Required    bool                 `json:"required,omitempty"`
	Default     interface{}          `json:"default,omitempty"`
	Enum        []interface{}        `json:"enum,omitempty"`
	Properties  map[string]*Property `json:"properties,omitempty"`
	Items       *Property            `json:"items,omitempty"`
}

// Schema represents a data schema
type Schema struct {
	Title       string               `json:"title,omitempty"`
	Description string               `json:"description,omitempty"`
	Type        PropertyType         `json:"type,omitempty"`
	Properties  map[string]*Property `json:"properties,omitempty"`
	Required    []string             `json:"required,omitempty"`
}

// NewSchema creates a new schema
func NewSchema() *Schema {
	return &Schema{
		Type:       TypeObject,
		Properties: make(map[string]*Property),
	}
}

// Validate validates data against the schema
func (s *Schema) Validate(data interface{}) (bool, error) {
	// 简化实现，未来可以添加实际验证逻辑
	return true, nil
}
