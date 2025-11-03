package cora

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// ToolBuilder helps construct tools from Go functions with automatic schema generation.
type ToolBuilder struct {
	tools    []CoraTool
	handlers map[string]CoraToolHandler
}

// NewToolBuilder creates a new tool builder.
func NewToolBuilder() *ToolBuilder {
	return &ToolBuilder{
		tools:    make([]CoraTool, 0),
		handlers: make(map[string]CoraToolHandler, 0),
	}
}

// AddFunc registers a Go function as a tool with automatic schema generation.
// The function signature should be: func(ctx context.Context, params YourStructType) (result any, err error)
// The params struct's fields and tags are used to generate the JSON schema.
func (tb *ToolBuilder) AddFunc(name, description string, handlerFunc any) error {
	handler, schema, err := wrapFunction(handlerFunc)
	if err != nil {
		return fmt.Errorf("failed to wrap function %s: %w", name, err)
	}

	tb.tools = append(tb.tools, CoraTool{
		Name:             name,
		Description:      description,
		ParametersSchema: schema,
	})
	tb.handlers[name] = handler
	return nil
}

// AddTool manually adds a pre-configured tool and its handler.
func (tb *ToolBuilder) AddTool(tool CoraTool, handler CoraToolHandler) {
	tb.tools = append(tb.tools, tool)
	tb.handlers[tool.Name] = handler
}

// Build returns the finalized tools and handlers for use in a TextRequest.
func (tb *ToolBuilder) Build() ([]CoraTool, map[string]CoraToolHandler) {
	return tb.tools, tb.handlers
}

// wrapFunction inspects a Go function and generates a tool handler + JSON schema.
// Expected signature: func(ctx context.Context, input T) (output any, err error)
func wrapFunction(fn any) (CoraToolHandler, map[string]any, error) {
	fnVal := reflect.ValueOf(fn)
	fnType := fnVal.Type()

	// Validate function signature
	if fnType.Kind() != reflect.Func {
		return nil, nil, errors.New("handler must be a function")
	}
	if fnType.NumIn() != 2 {
		return nil, nil, errors.New("function must have exactly 2 parameters: (context.Context, ParamsStruct)")
	}
	if fnType.NumOut() != 2 {
		return nil, nil, errors.New("function must return exactly 2 values: (result any, error)")
	}

	// Check context.Context as first param
	ctxType := reflect.TypeOf((*context.Context)(nil)).Elem()
	if !fnType.In(0).Implements(ctxType) {
		return nil, nil, errors.New("first parameter must be context.Context")
	}

	// Check error as second return
	errType := reflect.TypeOf((*error)(nil)).Elem()
	if !fnType.Out(1).Implements(errType) {
		return nil, nil, errors.New("second return value must be error")
	}

	// Extract parameter struct type
	paramsType := fnType.In(1)
	if paramsType.Kind() != reflect.Struct {
		return nil, nil, errors.New("second parameter must be a struct")
	}

	// Generate JSON schema from struct
	schema, err := generateSchemaFromStruct(paramsType)
	if err != nil {
		return nil, nil, fmt.Errorf("schema generation failed: %w", err)
	}

	// Create handler wrapper that unmarshals map[string]any -> struct -> calls function
	handler := func(ctx context.Context, args map[string]any) (any, error) {
		// Marshal args back to JSON
		argsJSON, err := json.Marshal(args)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal args: %w", err)
		}

		// Create zero value of params struct
		paramsVal := reflect.New(paramsType).Interface()

		// Unmarshal into struct
		if err := json.Unmarshal(argsJSON, paramsVal); err != nil {
			return nil, fmt.Errorf("failed to unmarshal args into %s: %w", paramsType.Name(), err)
		}

		// Call the original function
		results := fnVal.Call([]reflect.Value{
			reflect.ValueOf(ctx),
			reflect.ValueOf(paramsVal).Elem(),
		})

		// Extract result and error
		resultVal := results[0].Interface()
		errVal := results[1].Interface()

		if errVal != nil {
			return nil, errVal.(error)
		}
		return resultVal, nil
	}

	return handler, schema, nil
}

// generateSchemaFromStruct creates a JSON schema object from a Go struct using reflection and tags.
func generateSchemaFromStruct(t reflect.Type) (map[string]any, error) {
	if t.Kind() != reflect.Struct {
		return nil, errors.New("type must be a struct")
	}

	properties := make(map[string]any)
	required := make([]string, 0)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Extract JSON tag name
		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue // Skip fields with json:"-"
		}
		fieldName := field.Name
		if jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" {
				fieldName = parts[0]
			}
			// Check for omitempty
			if !contains(parts, "omitempty") {
				required = append(required, fieldName)
			}
		} else {
			// No json tag, field is required by default
			required = append(required, fieldName)
		}

		// Extract description from `description` tag
		desc := field.Tag.Get("description")

		// Map Go type to JSON schema type
		fieldSchema := typeToSchema(field.Type)
		if desc != "" {
			fieldSchema["description"] = desc
		}

		properties[fieldName] = fieldSchema
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}

	return schema, nil
}

// typeToSchema maps a Go reflect.Type to a JSON schema primitive.
func typeToSchema(t reflect.Type) map[string]any {
	schema := make(map[string]any)

	// Handle pointers
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.String:
		schema["type"] = "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		schema["type"] = "integer"
	case reflect.Float32, reflect.Float64:
		schema["type"] = "number"
	case reflect.Bool:
		schema["type"] = "boolean"
	case reflect.Slice, reflect.Array:
		schema["type"] = "array"
		schema["items"] = typeToSchema(t.Elem())
	case reflect.Struct:
		// Recursively handle nested structs
		nested, _ := generateSchemaFromStruct(t)
		return nested
	case reflect.Map:
		schema["type"] = "object"
		// For simplicity, we don't enforce key/value types deeply
	default:
		schema["type"] = "string" // fallback
	}

	return schema
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}