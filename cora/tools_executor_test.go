package cora

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestToolCache(t *testing.T) {
	cache := NewToolCache(1*time.Second, 10)

	args := map[string]any{"x": 5}
	
	// Miss on first call
	_, _, found := cache.Get("add", args)
	if found {
		t.Error("expected cache miss")
	}

	// Store result
	cache.Set("add", args, 10, nil)

	// Hit on second call
	result, err, found := cache.Get("add", args)
	if !found {
		t.Error("expected cache hit")
	}
	if result != 10 {
		t.Errorf("expected result 10, got %v", result)
	}
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Test expiration
	time.Sleep(1100 * time.Millisecond)
	_, _, found = cache.Get("add", args)
	if found {
		t.Error("expected cache miss after expiration")
	}
}

func TestToolValidator(t *testing.T) {
	tools := []CoraTool{
		{
			Name: "calculate",
			ParametersSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"x": map[string]any{"type": "number"},
					"y": map[string]any{"type": "number"},
				},
				"required": []string{"x", "y"},
			},
		},
	}

	validator := NewToolValidator(tools)

	// Valid call
	err := validator.ValidateCall("calculate", map[string]any{"x": 5.0, "y": 3.0})
	if err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}

	// Missing required field
	err = validator.ValidateCall("calculate", map[string]any{"x": 5.0})
	if err == nil {
		t.Error("expected validation error for missing required field")
	}

	// Wrong type
	err = validator.ValidateCall("calculate", map[string]any{"x": "not a number", "y": 3.0})
	if err == nil {
		t.Error("expected validation error for wrong type")
	}
}

func TestToolExecutorWithCache(t *testing.T) {
	callCount := 0
	handlers := map[string]CoraToolHandler{
		"expensive": func(ctx context.Context, args map[string]any) (any, error) {
			callCount++
			time.Sleep(100 * time.Millisecond)
			return "result", nil
		},
	}

	executor := NewToolExecutor(handlers).
		WithCache(1*time.Second, 10)

	ctx := context.Background()
	calls := []toolCallRequest{
		{name: "expensive", args: map[string]any{"id": 1}},
		{name: "expensive", args: map[string]any{"id": 1}}, // Same call
	}

	results, err := executor.executeBatch(ctx, calls)
	if err != nil {
		t.Fatalf("executeBatch failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// First call should execute, second should be cached
	if callCount != 1 {
		t.Errorf("expected handler to be called once, got %d calls", callCount)
	}

	if !results[1].cached {
		t.Error("second result should be marked as cached")
	}

	metrics := executor.Metrics()
	if metrics.CachedCalls != 1 {
		t.Errorf("expected 1 cached call, got %d", metrics.CachedCalls)
	}
}

func TestRetryableToolHandler(t *testing.T) {
	attempts := 0
	transientErr := errors.New("transient error")

	handler := func(ctx context.Context, args map[string]any) (any, error) {
		attempts++
		if attempts < 3 {
			return nil, transientErr
		}
		return "success", nil
	}

	config := RetryConfig{
		MaxAttempts:       3,
		InitialBackoff:    10 * time.Millisecond,
		MaxBackoff:        100 * time.Millisecond,
		BackoffMultiplier: 2.0,
		RetryableErrors:   []error{transientErr},
	}

	wrapped := RetryableToolHandler(handler, config)
	
	result, err := wrapped(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected success after retries, got error: %v", err)
	}
	if result != "success" {
		t.Errorf("unexpected result: %v", result)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}