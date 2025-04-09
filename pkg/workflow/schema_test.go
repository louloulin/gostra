package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSchemaValidation(t *testing.T) {
	// Create a schema for testing
	schema := NewSchema(TypeObject, "Test schema")

	// Add properties to the schema
	schema.AddProperty("name", NewSimpleSchema(TypeString, "The name field"), true)
	schema.AddProperty("age", NewSimpleSchema(TypeInteger, "The age field"), true)

	// Add an object property
	addressSchema := NewSimpleSchema(TypeObject, "Address information")
	addressSchema.Properties["street"] = NewSimpleSchema(TypeString, "Street name")
	addressSchema.Properties["city"] = NewSimpleSchema(TypeString, "City name")
	schema.AddProperty("address", addressSchema, false)

	// Add an array property
	itemsSchema := NewSimpleSchema(TypeString, "Item name")
	arraySchema := NewSimpleSchema(TypeArray, "List of items")
	arraySchema.Items = &itemsSchema
	schema.AddProperty("items", arraySchema, false)

	// Test valid data
	validData := map[string]interface{}{
		"name": "John Doe",
		"age":  30,
		"address": map[string]interface{}{
			"street": "123 Main St",
			"city":   "Anytown",
		},
		"items": []string{"item1", "item2", "item3"},
	}

	errors := schema.Validate(validData)
	assert.Empty(t, errors, "Expected no validation errors for valid data")

	// Test missing required field
	missingRequired := map[string]interface{}{
		"name": "John Doe",
		// Missing age field
		"address": map[string]interface{}{
			"street": "123 Main St",
			"city":   "Anytown",
		},
	}

	errors = schema.Validate(missingRequired)
	assert.NotEmpty(t, errors, "Expected validation errors for missing required field")
	assert.Contains(t, errors[0].Message, "required")

	// Test wrong type
	wrongType := map[string]interface{}{
		"name": "John Doe",
		"age":  "thirty", // String instead of integer
	}

	errors = schema.Validate(wrongType)
	assert.NotEmpty(t, errors, "Expected validation errors for wrong type")
	assert.Contains(t, errors[0].Message, "Expected integer")

	// Test ValidateWorkflowInput convenience function
	err := ValidateWorkflowInput(schema, validData)
	assert.NoError(t, err, "Expected no error for valid data")

	err = ValidateWorkflowInput(schema, wrongType)
	assert.Error(t, err, "Expected error for invalid data")
	assert.Contains(t, err.Error(), "workflow input validation failed")
}

func TestComplexSchemaValidation(t *testing.T) {
	// Create a schema that mimics Mastra's workflow trigger schema
	schema := NewSchema(TypeObject, "Workflow trigger schema")

	// Add query property
	querySchema := NewSimpleSchema(TypeString, "The search query")
	querySchema.MinLength = 3
	querySchema.MaxLength = 500
	schema.AddProperty("query", querySchema, true)

	// Add options property
	optionsSchema := NewSimpleSchema(TypeObject, "Search options")
	optionsSchema.Properties["top_k"] = NewSimpleSchema(TypeInteger, "Number of results to return")
	optionsSchema.Properties["threshold"] = NewSimpleSchema(TypeNumber, "Similarity threshold")

	// Add filters property
	filtersSchema := NewSimpleSchema(TypeObject, "Search filters")
	filtersSchema.Properties["category"] = NewSimpleSchema(TypeString, "Category filter")
	filtersSchema.Properties["date_range"] = NewSimpleSchema(TypeString, "Date range filter")
	optionsSchema.Properties["filters"] = filtersSchema

	// Test with valid data
	validData := map[string]interface{}{
		"query": "What are the key insights from this document?",
		"options": map[string]interface{}{
			"top_k":     5,
			"threshold": 0.75,
		},
	}

	errors := schema.Validate(validData)
	assert.Empty(t, errors, "Expected no validation errors for valid complex data")

	// Test with invalid query (too short)
	invalidQuery := map[string]interface{}{
		"query": "Hi",
		"options": map[string]interface{}{
			"top_k":     5,
			"threshold": 0.75,
		},
	}

	errors = schema.Validate(invalidQuery)
	assert.NotEmpty(t, errors, "Expected validation errors for short query")
	assert.Contains(t, errors[0].Message, "minimum")

	// Test with invalid option types
	invalidOptions := map[string]interface{}{
		"query": "What are the key insights from this document?",
		"options": map[string]interface{}{
			"top_k":     "five", // String instead of integer
			"threshold": "high", // String instead of number
		},
	}

	errors = schema.Validate(invalidOptions)
	assert.NotEmpty(t, errors, "Expected validation errors for invalid option types")
}

func TestSchemaWithPattern(t *testing.T) {
	// Create a schema with pattern validation
	schema := NewSchema(TypeObject, "Pattern test schema")

	// Add email field with pattern
	emailSchema := NewSimpleSchema(TypeString, "Email address")
	emailSchema.Pattern = "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"
	schema.AddProperty("email", emailSchema, true)

	// Test with valid email
	validData := map[string]interface{}{
		"email": "test@example.com",
	}

	errors := schema.Validate(validData)
	assert.Empty(t, errors, "Expected no validation errors for valid email")

	// Test with invalid email
	invalidData := map[string]interface{}{
		"email": "not-an-email",
	}

	errors = schema.Validate(invalidData)
	assert.NotEmpty(t, errors, "Expected validation errors for invalid email")
	assert.Contains(t, errors[0].Message, "pattern")
}
