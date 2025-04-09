package workflow

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// SchemaType represents the type of a schema field
type SchemaType string

const (
	TypeString  SchemaType = "string"
	TypeInteger SchemaType = "integer"
	TypeNumber  SchemaType = "number"
	TypeBoolean SchemaType = "boolean"
	TypeObject  SchemaType = "object"
	TypeArray   SchemaType = "array"
	TypeAny     SchemaType = "any"
)

// SchemaField represents a field in a schema
type SchemaField struct {
	Type        SchemaType             `json:"type"`
	Description string                 `json:"description,omitempty"`
	Required    bool                   `json:"required"`
	Pattern     string                 `json:"pattern,omitempty"`
	MinLength   int                    `json:"min_length,omitempty"`
	MaxLength   int                    `json:"max_length,omitempty"`
	Minimum     float64                `json:"minimum,omitempty"`
	Maximum     float64                `json:"maximum,omitempty"`
	Properties  map[string]SchemaField `json:"properties,omitempty"`
	Items       *SchemaField           `json:"items,omitempty"`
	Enum        []interface{}          `json:"enum,omitempty"`
}

// Schema represents a complete validation schema
type Schema struct {
	Type        SchemaType             `json:"type"`
	Description string                 `json:"description,omitempty"`
	Properties  map[string]SchemaField `json:"properties,omitempty"`
	Required    []string               `json:"required,omitempty"`
}

// ValidationError represents an error during schema validation
type ValidationError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// NewSchema creates a new Schema
func NewSchema(schemaType SchemaType, description string) *Schema {
	return &Schema{
		Type:        schemaType,
		Description: description,
		Properties:  make(map[string]SchemaField),
		Required:    []string{},
	}
}

// AddProperty adds a property to the schema
func (s *Schema) AddProperty(name string, field SchemaField, required bool) {
	s.Properties[name] = field
	if required {
		s.Required = append(s.Required, name)
	}
}

// Validate validates data against the schema
func (s *Schema) Validate(data interface{}) []ValidationError {
	return s.validateType("", s.Type, data, s.Properties, s.Required)
}

// validateType validates a value against a specific type
func (s *Schema) validateType(path string, schemaType SchemaType, value interface{}, properties map[string]SchemaField, required []string) []ValidationError {
	errors := []ValidationError{}

	// Nil check
	if value == nil {
		if len(required) > 0 && containsString(required, strings.TrimPrefix(path, ".")) {
			errors = append(errors, ValidationError{
				Path:    path,
				Message: "Value is required but got nil",
			})
		}
		return errors
	}

	// Type checks
	switch schemaType {
	case TypeString:
		if _, ok := value.(string); !ok {
			errors = append(errors, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("Expected string but got %T", value),
			})
		} else {
			// Additional string validations
			strValue := value.(string)
			if field, ok := properties[strings.TrimPrefix(path, ".")]; ok {
				// Pattern check
				if field.Pattern != "" {
					pattern := regexp.MustCompile(field.Pattern)
					if !pattern.MatchString(strValue) {
						errors = append(errors, ValidationError{
							Path:    path,
							Message: fmt.Sprintf("String '%s' does not match pattern '%s'", strValue, field.Pattern),
						})
					}
				}

				// Length checks
				if field.MinLength > 0 && len(strValue) < field.MinLength {
					errors = append(errors, ValidationError{
						Path:    path,
						Message: fmt.Sprintf("String length %d is less than minimum %d", len(strValue), field.MinLength),
					})
				}

				if field.MaxLength > 0 && len(strValue) > field.MaxLength {
					errors = append(errors, ValidationError{
						Path:    path,
						Message: fmt.Sprintf("String length %d is greater than maximum %d", len(strValue), field.MaxLength),
					})
				}
			}
		}

	case TypeInteger:
		if reflect.TypeOf(value).Kind() != reflect.Int &&
			reflect.TypeOf(value).Kind() != reflect.Int8 &&
			reflect.TypeOf(value).Kind() != reflect.Int16 &&
			reflect.TypeOf(value).Kind() != reflect.Int32 &&
			reflect.TypeOf(value).Kind() != reflect.Int64 {
			errors = append(errors, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("Expected integer but got %T", value),
			})
		}

	case TypeNumber:
		if reflect.TypeOf(value).Kind() != reflect.Float32 &&
			reflect.TypeOf(value).Kind() != reflect.Float64 &&
			reflect.TypeOf(value).Kind() != reflect.Int &&
			reflect.TypeOf(value).Kind() != reflect.Int8 &&
			reflect.TypeOf(value).Kind() != reflect.Int16 &&
			reflect.TypeOf(value).Kind() != reflect.Int32 &&
			reflect.TypeOf(value).Kind() != reflect.Int64 {
			errors = append(errors, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("Expected number but got %T", value),
			})
		}

	case TypeBoolean:
		if _, ok := value.(bool); !ok {
			errors = append(errors, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("Expected boolean but got %T", value),
			})
		}

	case TypeObject:
		// Check if it's a map
		if reflect.TypeOf(value).Kind() != reflect.Map {
			errors = append(errors, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("Expected object but got %T", value),
			})
			return errors
		}

		// Convert to map[string]interface{} for validation
		var obj map[string]interface{}

		// Handle different map types
		valueMap := reflect.ValueOf(value)
		if valueMap.Type().Key().Kind() == reflect.String {
			// If it's already a map with string keys, convert directly
			obj = make(map[string]interface{})
			iter := valueMap.MapRange()
			for iter.Next() {
				obj[iter.Key().String()] = iter.Value().Interface()
			}
		} else {
			// Try JSON marshaling/unmarshaling for complex types
			bytes, err := json.Marshal(value)
			if err != nil {
				errors = append(errors, ValidationError{
					Path:    path,
					Message: fmt.Sprintf("Failed to marshal object: %v", err),
				})
				return errors
			}

			err = json.Unmarshal(bytes, &obj)
			if err != nil {
				errors = append(errors, ValidationError{
					Path:    path,
					Message: fmt.Sprintf("Failed to unmarshal object: %v", err),
				})
				return errors
			}
		}

		// Check required properties
		for _, req := range required {
			if _, ok := obj[req]; !ok {
				errors = append(errors, ValidationError{
					Path:    fmt.Sprintf("%s.%s", path, req),
					Message: fmt.Sprintf("Required property '%s' is missing", req),
				})
			}
		}

		// Validate each property
		for propName, propSchema := range properties {
			propPath := path
			if path != "" {
				propPath = fmt.Sprintf("%s.%s", path, propName)
			} else {
				propPath = propName
			}

			if propValue, ok := obj[propName]; ok {
				propErrors := s.validateType(propPath, propSchema.Type, propValue, propSchema.Properties, []string{})
				errors = append(errors, propErrors...)
			}
		}

	case TypeArray:
		// Check if it's an array/slice
		if reflect.TypeOf(value).Kind() != reflect.Slice && reflect.TypeOf(value).Kind() != reflect.Array {
			errors = append(errors, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("Expected array but got %T", value),
			})
			return errors
		}

		// Validate each item in the array
		arrayValue := reflect.ValueOf(value)
		for i := 0; i < arrayValue.Len(); i++ {
			itemPath := fmt.Sprintf("%s[%d]", path, i)
			itemValue := arrayValue.Index(i).Interface()

			// If we have an item schema, validate against it
			if field, ok := properties[strings.TrimPrefix(path, ".")]; ok && field.Items != nil {
				itemErrors := s.validateType(itemPath, field.Items.Type, itemValue, field.Items.Properties, []string{})
				errors = append(errors, itemErrors...)
			}
		}

	case TypeAny:
		// TypeAny accepts any type, no validation needed
		return errors

	default:
		errors = append(errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("Unknown schema type: %s", schemaType),
		})
	}

	return errors
}

// containsString checks if a string is in a slice of strings
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// NewSimpleSchema creates a simple schema for a given type
func NewSimpleSchema(schemaType SchemaType, description string) SchemaField {
	return SchemaField{
		Type:        schemaType,
		Description: description,
		Required:    false,
		Properties:  make(map[string]SchemaField),
	}
}

// ValidateWorkflowInput validates the input against a schema and returns a formatted error if invalid
func ValidateWorkflowInput(schema *Schema, input map[string]interface{}) error {
	validationErrors := schema.Validate(input)
	if len(validationErrors) > 0 {
		errorMessages := make([]string, len(validationErrors))
		for i, err := range validationErrors {
			errorMessages[i] = fmt.Sprintf("%s: %s", err.Path, err.Message)
		}
		return fmt.Errorf("workflow input validation failed: %s", strings.Join(errorMessages, "; "))
	}
	return nil
}
