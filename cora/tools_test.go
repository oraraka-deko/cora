package cora

import (
	"context"
	"errors"
	"testing"
)

// Example tool function
type WeatherParams struct {
	Location string `json:"location" description:"City and state, e.g. San Francisco, CA"`
	Unit     string `json:"unit,omitempty" description:"Temperature unit (celsius or fahrenheit)"`
}

func getWeather(ctx context.Context, params WeatherParams) (any, error) {
	if params.Location == "" {
		return nil, errors.New("location is required")
	}
	return map[string]any{
		"location":    params.Location,
		"temperature": 72,
		"unit":        params.Unit,
		"condition":   "sunny",
	}, nil
}

func TestToolBuilder_AddFunc(t *testing.T) {
	tb := NewToolBuilder()
	err := tb.AddFunc("get_weather", "Get current weather", getWeather)
	if err != nil {
		t.Fatalf("AddFunc failed: %v", err)
	}

	tools, handlers := tb.Build()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if len(handlers) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(handlers))
	}

	tool := tools[0]
	if tool.Name != "get_weather" {
		t.Errorf("expected name 'get_weather', got %q", tool.Name)
	}

	// Verify schema structure
	schema := tool.ParametersSchema
	if schema["type"] != "object" {
		t.Errorf("expected type 'object', got %v", schema["type"])
	}

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties is not a map")
	}

	if _, ok := props["location"]; !ok {
		t.Errorf("schema missing 'location' property")
	}

	required, _ := schema["required"].([]string)
	if !contains(required, "location") {
		t.Errorf("schema should require 'location'")
	}
}

func TestToolBuilder_ExecuteHandler(t *testing.T) {
	tb := NewToolBuilder()
	_ = tb.AddFunc("get_weather", "Get current weather", getWeather)
	_, handlers := tb.Build()

	handler := handlers["get_weather"]
	ctx := context.Background()

	result, err := handler(ctx, map[string]any{
		"location": "Boston, MA",
		"unit":     "fahrenheit",
	})
	if err != nil {
		t.Fatalf("handler execution failed: %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result is not a map")
	}

	if resultMap["location"] != "Boston, MA" {
		t.Errorf("unexpected location: %v", resultMap["location"])
	}
	if resultMap["temperature"] != 72 {
		t.Errorf("unexpected temperature: %v", resultMap["temperature"])
	}
}

func TestToolExecutor_Serial(t *testing.T) {
	handlers := map[string]CoraToolHandler{
		"add": func(ctx context.Context, args map[string]any) (any, error) {
			a := int(args["a"].(float64))
			b := int(args["b"].(float64))
			return a + b, nil
		},
		"multiply": func(ctx context.Context, args map[string]any) (any, error) {
			a := int(args["a"].(float64))
			b := int(args["b"].(float64))
			return a * b, nil
		},
	}

	executor := NewToolExecutor(handlers)
	ctx := context.Background()

	calls := []toolCallRequest{
		{name: "add", args: map[string]any{"a": 2.0, "b": 3.0}},
		{name: "multiply", args: map[string]any{"a": 4.0, "b": 5.0}},
	}

	results, err := executor.executeBatch(ctx, calls)
	if err != nil {
		t.Fatalf("executeBatch failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].result != 5 {
		t.Errorf("add result: expected 5, got %v", results[0].result)
	}
	if results[1].result != 20 {
		t.Errorf("multiply result: expected 20, got %v", results[1].result)
	}
}