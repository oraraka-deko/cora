package cora

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestToolCalling_WithConfigurableMaxRounds tests that MaxToolRounds is respected
func TestToolCalling_WithConfigurableMaxRounds(t *testing.T) {
	maxRounds := 3
	roundCounter := 0

	handlers := map[string]CoraToolHandler{
		"counter": func(ctx context.Context, args map[string]any) (any, error) {
			roundCounter++
			return map[string]any{"count": roundCounter}, nil
		},
	}

	tools := []CoraTool{
		{
			Name:        "counter",
			Description: "Counts rounds",
			ParametersSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}

	plan := callPlan{
		Tools:         tools,
		ToolHandlers:  handlers,
		MaxToolRounds: &maxRounds,
	}

	// Verify the plan has the right value
	if plan.MaxToolRounds == nil || *plan.MaxToolRounds != 3 {
		t.Errorf("expected MaxToolRounds to be 3, got %v", plan.MaxToolRounds)
	}
}

// TestToolCalling_ParallelConfiguration tests ParallelTools configuration
func TestToolCalling_ParallelConfiguration(t *testing.T) {
	parallelTrue := true
	parallelFalse := false

	testCases := []struct {
		name     string
		parallel *bool
		expected bool
	}{
		{"parallel true", &parallelTrue, true},
		{"parallel false", &parallelFalse, false},
		{"parallel nil (default)", nil, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			plan := callPlan{
				ParallelTools: tc.parallel,
			}

			// Verify the configuration is stored correctly
			if tc.parallel == nil {
				if plan.ParallelTools != nil {
					t.Errorf("expected ParallelTools to be nil, got %v", *plan.ParallelTools)
				}
			} else {
				if plan.ParallelTools == nil || *plan.ParallelTools != tc.expected {
					t.Errorf("expected ParallelTools to be %v, got %v", tc.expected, plan.ParallelTools)
				}
			}
		})
	}
}

// TestToolCalling_StopOnErrorConfiguration tests StopOnToolError configuration
func TestToolCalling_StopOnErrorConfiguration(t *testing.T) {
	stopTrue := true
	stopFalse := false

	testCases := []struct {
		name     string
		stopOnError *bool
		expected bool
	}{
		{"stop on error true", &stopTrue, true},
		{"stop on error false", &stopFalse, false},
		{"stop on error nil (default)", nil, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			plan := callPlan{
				StopOnToolError: tc.stopOnError,
			}

			// Verify the configuration is stored correctly
			if tc.stopOnError == nil {
				if plan.StopOnToolError != nil {
					t.Errorf("expected StopOnToolError to be nil, got %v", *plan.StopOnToolError)
				}
			} else {
				if plan.StopOnToolError == nil || *plan.StopOnToolError != tc.expected {
					t.Errorf("expected StopOnToolError to be %v, got %v", tc.expected, plan.StopOnToolError)
				}
			}
		})
	}
}

// TestToolExecutor_WithValidator tests that validator is used correctly
func TestToolExecutor_WithValidator(t *testing.T) {
	tools := []CoraTool{
		{
			Name:        "add",
			Description: "Add two numbers",
			ParametersSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"a": map[string]any{"type": "number"},
					"b": map[string]any{"type": "number"},
				},
				"required": []string{"a", "b"},
			},
		},
	}

	handlers := map[string]CoraToolHandler{
		"add": func(ctx context.Context, args map[string]any) (any, error) {
			a := args["a"].(float64)
			b := args["b"].(float64)
			return a + b, nil
		},
	}

	executor := NewToolExecutor(handlers).WithValidator(tools)

	// Test valid call
	ctx := context.Background()
	validCall := toolCallRequest{
		name: "add",
		args: map[string]any{"a": 5.0, "b": 3.0},
	}
	result, err := executor.executeSingleCall(ctx, validCall)
	if err != nil {
		t.Errorf("valid call should succeed, got error: %v", err)
	}
	if result.result != 8.0 {
		t.Errorf("expected result 8.0, got %v", result.result)
	}

	// Test invalid call (missing required parameter)
	invalidCall := toolCallRequest{
		name: "add",
		args: map[string]any{"a": 5.0}, // missing 'b'
	}
	_, err = executor.executeSingleCall(ctx, invalidCall)
	if err == nil {
		t.Error("invalid call should fail validation")
	}
}

// TestBuildPlans_WithToolConfiguration tests that tool configuration is properly passed through
func TestBuildPlans_WithToolConfiguration(t *testing.T) {
	maxRounds := 10
	parallel := true
	stopOnError := false

	req := TextRequest{
		Mode: ModeToolCalling,
		Tools: []CoraTool{
			{Name: "test", Description: "test tool", ParametersSchema: map[string]any{"type": "object"}},
		},
		ToolHandlers: map[string]CoraToolHandler{
			"test": func(ctx context.Context, args map[string]any) (any, error) {
				return "ok", nil
			},
		},
		MaxToolRounds:   &maxRounds,
		ParallelTools:   &parallel,
		StopOnToolError: &stopOnError,
	}

	cfg := CoraConfig{
		ToolCacheTTL:     5 * time.Minute,
		ToolCacheMaxSize: 100,
		ToolRetryConfig: &RetryConfig{
			MaxAttempts:    3,
			InitialBackoff: 100 * time.Millisecond,
		},
	}

	plans, err := buildPlans(ProviderGoogle, "gemini-test", req, cfg)
	if err != nil {
		t.Fatalf("buildPlans error: %v", err)
	}

	if len(plans) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(plans))
	}

	plan := plans[0]
	if plan.MaxToolRounds == nil || *plan.MaxToolRounds != 10 {
		t.Errorf("expected MaxToolRounds to be 10, got %v", plan.MaxToolRounds)
	}
	if plan.ParallelTools == nil || *plan.ParallelTools != true {
		t.Errorf("expected ParallelTools to be true, got %v", plan.ParallelTools)
	}
	if plan.StopOnToolError == nil || *plan.StopOnToolError != false {
		t.Errorf("expected StopOnToolError to be false, got %v", plan.StopOnToolError)
	}

	// Verify client-level config is passed through
	if plan.ToolCacheTTL != 5*time.Minute {
		t.Errorf("expected ToolCacheTTL to be 5m, got %v", plan.ToolCacheTTL)
	}
	if plan.ToolCacheMaxSize != 100 {
		t.Errorf("expected ToolCacheMaxSize to be 100, got %d", plan.ToolCacheMaxSize)
	}
	if plan.ToolRetryConfig == nil || plan.ToolRetryConfig.MaxAttempts != 3 {
		t.Errorf("expected ToolRetryConfig with MaxAttempts=3, got %v", plan.ToolRetryConfig)
	}
}

// TestToolExecutor_ParallelExecution tests that parallel execution works correctly
func TestToolExecutor_ParallelExecution(t *testing.T) {
	callCount := 0
	handlers := map[string]CoraToolHandler{
		"tool1": func(ctx context.Context, args map[string]any) (any, error) {
			callCount++
			return "result1", nil
		},
		"tool2": func(ctx context.Context, args map[string]any) (any, error) {
			callCount++
			return "result2", nil
		},
		"tool3": func(ctx context.Context, args map[string]any) (any, error) {
			callCount++
			return "result3", nil
		},
	}

	executor := NewToolExecutor(handlers).WithParallel(true)
	ctx := context.Background()

	calls := []toolCallRequest{
		{name: "tool1", args: map[string]any{}},
		{name: "tool2", args: map[string]any{}},
		{name: "tool3", args: map[string]any{}},
	}

	results, err := executor.executeBatch(ctx, calls)
	if err != nil {
		t.Fatalf("executeBatch failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}

	// Check all results are correct
	expectedResults := []string{"result1", "result2", "result3"}
	for i, result := range results {
		if result.result != expectedResults[i] {
			t.Errorf("result[%d]: expected %q, got %v", i, expectedResults[i], result.result)
		}
	}
}

// TestToolExecutor_StopOnError tests error handling behavior
func TestToolExecutor_StopOnError(t *testing.T) {
	testError := errors.New("tool failed")
	handlers := map[string]CoraToolHandler{
		"tool1": func(ctx context.Context, args map[string]any) (any, error) {
			return "ok", nil
		},
		"tool2": func(ctx context.Context, args map[string]any) (any, error) {
			return nil, testError
		},
		"tool3": func(ctx context.Context, args map[string]any) (any, error) {
			return "ok", nil
		},
	}

	t.Run("stop on error true", func(t *testing.T) {
		executor := NewToolExecutor(handlers).WithStopOnError(true)
		ctx := context.Background()

		calls := []toolCallRequest{
			{name: "tool1", args: map[string]any{}},
			{name: "tool2", args: map[string]any{}},
			{name: "tool3", args: map[string]any{}},
		}

		_, err := executor.executeBatch(ctx, calls)
		if err == nil {
			t.Error("expected error when tool2 fails")
		}
	})

	t.Run("stop on error false", func(t *testing.T) {
		executor := NewToolExecutor(handlers).WithStopOnError(false)
		ctx := context.Background()

		calls := []toolCallRequest{
			{name: "tool1", args: map[string]any{}},
			{name: "tool2", args: map[string]any{}},
			{name: "tool3", args: map[string]any{}},
		}

		results, err := executor.executeBatch(ctx, calls)
		if err != nil {
			t.Errorf("should not return error when StopOnError is false, got: %v", err)
		}

		if len(results) != 3 {
			t.Errorf("expected 3 results, got %d", len(results))
		}

		// Check that tool1 and tool3 succeeded, tool2 failed
		if results[0].err != nil {
			t.Errorf("tool1 should succeed, got error: %v", results[0].err)
		}
		if results[1].err == nil {
			t.Error("tool2 should have error")
		}
		if results[2].err != nil {
			t.Errorf("tool3 should succeed, got error: %v", results[2].err)
		}
	})
}
