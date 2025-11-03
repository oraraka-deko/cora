package cora

import (
	"fmt"
	"reflect"
)

// ToolValidator validates tool call arguments against the tool's schema.
type ToolValidator struct {
	tools map[string]CoraTool
}

// NewToolValidator creates a validator from a list of tools.
func NewToolValidator(tools []CoraTool) *ToolValidator {
	toolMap := make(map[string]CoraTool)
	for _, t := range tools {
		toolMap[t.Name] = t
	}
	return &ToolValidator{tools: toolMap}
}

// ValidateCall checks if a tool call has valid arguments according to its schema.
func (tv *ToolValidator) ValidateCall(name string, args map[string]any) error {
	tool, exists := tv.tools[name]
	if !exists {
		return fmt.Errorf("unknown tool: %s", name)
	}

	if len(tool.ParametersSchema) == 0 {
		return nil // No schema to validate against
	}

	// Validate required fields
	required, ok := tool.ParametersSchema["required"].([]string)
	if !ok {
		// Try []any (from JSON unmarshal)
		if reqAny, ok := tool.ParametersSchema["required"].([]any); ok {
			required = make([]string, len(reqAny))
			for i, v := range reqAny {
				if s, ok := v.(string); ok {
					required[i] = s
				}
			}
		}
	}

	for _, fieldName := range required {
		if _, exists := args[fieldName]; !exists {
			return fmt.Errorf("missing required parameter: %s", fieldName)
		}
	}

	// Validate types of provided arguments
	properties, ok := tool.ParametersSchema["properties"].(map[string]any)
	if !ok {
		return nil // No property definitions
	}

	for argName, argValue := range args {
		propSchema, exists := properties[argName]
		if !exists {
			continue // Extra args are allowed
		}

		propMap, ok := propSchema.(map[string]any)
		if !ok {
			continue
		}

		expectedType, ok := propMap["type"].(string)
		if !ok {
			continue
		}

		if err := validateType(argName, argValue, expectedType); err != nil {
			return err
		}
	}

	return nil
}

func validateType(name string, value any, expectedType string) error {
	if value == nil {
		return nil // Null values pass (handled by required check)
	}

	actualType := reflect.TypeOf(value).Kind()

	switch expectedType {
	case "string":
		if actualType != reflect.String {
			return fmt.Errorf("parameter %s: expected string, got %v", name, actualType)
		}
	case "number":
		if actualType != reflect.Float64 && actualType != reflect.Float32 {
			return fmt.Errorf("parameter %s: expected number, got %v", name, actualType)
		}
	case "integer":
		// JSON numbers are float64; check if it's a whole number
		if f, ok := value.(float64); ok {
			if f != float64(int(f)) {
				return fmt.Errorf("parameter %s: expected integer, got float %v", name, f)
			}
		} else {
			return fmt.Errorf("parameter %s: expected integer, got %v", name, actualType)
		}
	case "boolean":
		if actualType != reflect.Bool {
			return fmt.Errorf("parameter %s: expected boolean, got %v", name, actualType)
		}
	case "array":
		if actualType != reflect.Slice && actualType != reflect.Array {
			return fmt.Errorf("parameter %s: expected array, got %v", name, actualType)
		}
	case "object":
		if actualType != reflect.Map {
			return fmt.Errorf("parameter %s: expected object, got %v", name, actualType)
		}
	}

	return nil
}