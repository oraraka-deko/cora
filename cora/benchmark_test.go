package cora

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestBenchmark_Google_Gemini tests Google Gemini API functionality
func TestBenchmark_Google_Gemini(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping benchmark test in short mode")
	}

	cfg := CoraConfig{
		GoogleAPIKey:       "",
		DefaultModelGoogle: "gemini-2.5-flash",
	}

	client := New(cfg)
	if client == nil {
		t.Fatal("Failed to create client")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	resp, err := client.Text(ctx, TextRequest{
		Provider: ProviderGoogle,
		Model:    "gemini-2.5-flash",
		Input:    "Say hello in exactly 5 words.",
		Mode:     ModeBasic,
	})
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Google Gemini request failed: %v", err)
	}

	if resp.Text == "" {
		t.Fatal("Expected non-empty response text")
	}

	t.Logf("✓ Google Gemini Response: %q", resp.Text)
	t.Logf("✓ Model: %s", resp.Model)
	t.Logf("✓ Response time: %v", duration)
	if resp.PromptTokens != nil {
		t.Logf("✓ Prompt tokens: %d", *resp.PromptTokens)
	}
	if resp.CompletionTokens != nil {
		t.Logf("✓ Completion tokens: %d", *resp.CompletionTokens)
	}
}

// TestBenchmark_Google_WithSystemPrompt tests Google with system message
func TestBenchmark_Google_WithSystemPrompt(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping benchmark test in short mode")
	}

	cfg := CoraConfig{
		GoogleAPIKey: "",
	}

	client := New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Text(ctx, TextRequest{
		Provider: ProviderGoogle,
		Model:    "gemini-2.5-flash",
		System:   "You are a helpful math tutor. Be concise.",
		Input:    "What is the Pythagorean theorem?",
		Mode:     ModeBasic,
	})

	if err != nil {
		t.Fatalf("Google request failed: %v", err)
	}

	if resp.Text == "" {
		t.Fatal("Expected non-empty response text")
	}

	t.Logf("✓ Google response with system prompt: %q", resp.Text)
}

// TestBenchmark_Google_StructuredJSON tests Google structured output
func TestBenchmark_Google_StructuredJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping benchmark test in short mode")
	}

	cfg := CoraConfig{
		GoogleAPIKey: "",
	}

	client := New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type": "string",
			},
			"capital": map[string]any{
				"type": "string",
			},
			"population": map[string]any{
				"type": "number",
			},
		},
		"required": []string{"name", "capital", "population"},
	}

	resp, err := client.Text(ctx, TextRequest{
		Provider:       ProviderGoogle,
		Model:          "gemini-2.5-flash",
		Input:          "Tell me about France: name, capital, and population (approximate)",
		Mode:           ModeStructuredJSON,
		ResponseSchema: schema,
	})

	if err != nil {
		t.Fatalf("Google structured JSON request failed: %v", err)
	}

	if resp.JSON == nil {
		t.Fatal("Expected JSON response")
	}

	t.Logf("✓ Google structured JSON response: %+v", resp.JSON)
}

// TestBenchmark_Parallel_BothProviders tests both providers in parallel
func TestBenchmark_Parallel_BothProviders(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping benchmark test in short mode")
	}

	// Create separate clients for each provider
	xaiClient := New(CoraConfig{
		OpenAIAPIKey:  "",
		OpenAIBaseURL: "https://api.x.ai/v1",
	})

	googleClient := New(CoraConfig{
		GoogleAPIKey: "",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	results := make(map[string]string)
	errors := make(map[string]error)
	timings := make(map[string]time.Duration)
	var mu sync.Mutex

	question := "What is artificial intelligence in one sentence?"

	// Test X.AI
	wg.Add(1)
	go func() {
		defer wg.Done()
		start := time.Now()
		resp, err := xaiClient.Text(ctx, TextRequest{
			Provider: ProviderOpenAI,
			Model:    "grok-4-fast-reasoning",
			Input:    question,
			Mode:     ModeBasic,
		})
		duration := time.Since(start)

		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			errors["X.AI"] = err
		} else {
			results["X.AI"] = resp.Text
			timings["X.AI"] = duration
		}
	}()

	// Test Google Gemini
	wg.Add(1)
	go func() {
		defer wg.Done()
		start := time.Now()
		resp, err := googleClient.Text(ctx, TextRequest{
			Provider: ProviderGoogle,
			Model:    "gemini-2.5-flash",
			Input:    question,
			Mode:     ModeBasic,
		})
		duration := time.Since(start)

		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			errors["Google"] = err
		} else {
			results["Google"] = resp.Text
			timings["Google"] = duration
		}
	}()

	wg.Wait()

	// Check results
	if len(errors) > 0 {
		for provider, err := range errors {
			t.Errorf("%s provider failed: %v", provider, err)
		}
	}

	if len(results) > 0 {
		t.Logf("✓ Parallel execution completed successfully")
		for provider, result := range results {
			t.Logf("✓ %s: %q (time: %v)", provider, result, timings[provider])
		}
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 successful responses, got %d", len(results))
	}
}

// TestBenchmark_MultipleRequests_Performance tests performance with multiple requests
func TestBenchmark_MultipleRequests_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping benchmark test in short mode")
	}

	providers := []struct {
		name     string
		client   *Client
		provider Provider
		model    string
	}{
		{
			name: "X.AI",
			client: New(CoraConfig{
				OpenAIAPIKey:  "",
				OpenAIBaseURL: "https://api.x.ai/v1",
			}),
			provider: ProviderOpenAI,
			model:    "grok-4-fast-reasoning",
		},
		{
			name: "Google",
			client: New(CoraConfig{
				GoogleAPIKey: "",
			}),
			provider: ProviderGoogle,
			model:    "gemini-2.5-flash",
		},
	}

	questions := []string{
		"What is 10+10?",
		"Name a fruit.",
		"What color is the sky?",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	for _, p := range providers {
		t.Run(p.name, func(t *testing.T) {
			start := time.Now()
			successCount := 0

			for i, question := range questions {
				resp, err := p.client.Text(ctx, TextRequest{
					Provider: p.provider,
					Model:    p.model,
					Input:    question,
					Mode:     ModeBasic,
				})

				if err != nil {
					t.Logf("✗ Request %d failed: %v", i+1, err)
					continue
				}

				if resp.Text != "" {
					successCount++
					t.Logf("✓ Q%d: %q → %q", i+1, question, resp.Text)
				}
			}

			totalDuration := time.Since(start)
			avgDuration := totalDuration / time.Duration(len(questions))

			t.Logf("✓ %s: %d/%d successful, total time: %v, avg: %v",
				p.name, successCount, len(questions), totalDuration, avgDuration)

			if successCount != len(questions) {
				t.Errorf("Expected %d successful requests, got %d", len(questions), successCount)
			}
		})
	}
}

// TestBenchmark_ToolCalling_XAI tests tool calling with X.AI
func TestBenchmark_ToolCalling_XAI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping benchmark test in short mode")
	}

	cfg := CoraConfig{
		OpenAIAPIKey:  "",
		OpenAIBaseURL: "https://api.x.ai/v1",
	}

	client := New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	// Define tools
	tools := []CoraTool{
		{
			Name:        "get_weather",
			Description: "Get the current weather for a location",
			ParametersSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"location": map[string]any{
						"type":        "string",
						"description": "The city name",
					},
				},
				"required": []string{"location"},
			},
		},
		{
			Name:        "calculate",
			Description: "Perform a mathematical calculation",
			ParametersSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"operation": map[string]any{
						"type":        "string",
						"description": "The operation to perform",
					},
					"a": map[string]any{
						"type":        "number",
						"description": "First number",
					},
					"b": map[string]any{
						"type":        "number",
						"description": "Second number",
					},
				},
				"required": []string{"operation", "a", "b"},
			},
		},
	}

	// Define handlers
	handlers := map[string]CoraToolHandler{
		"get_weather": func(ctx context.Context, args map[string]any) (any, error) {
			location := args["location"].(string)
			return map[string]any{
				"location":    location,
				"temperature": 72,
				"condition":   "sunny",
			}, nil
		},
		"calculate": func(ctx context.Context, args map[string]any) (any, error) {
			op := args["operation"].(string)
			a := args["a"].(float64)
			b := args["b"].(float64)

			var result float64
			switch op {
			case "add":
				result = a + b
			case "subtract":
				result = a - b
			case "multiply":
				result = a * b
			case "divide":
				if b != 0 {
					result = a / b
				}
			}

			return map[string]any{
				"result": result,
			}, nil
		},
	}

	start := time.Now()
	resp, err := client.Text(ctx, TextRequest{
		Provider:     ProviderOpenAI,
		Model:        "grok-4-fast-reasoning",
		Input:        "What's the weather in Paris and also calculate 15 + 27?",
		Mode:         ModeToolCalling,
		Tools:        tools,
		ToolHandlers: handlers,
	})
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Tool calling request failed: %v", err)
	}

	t.Logf("✓ Tool calling response: %q", resp.Text)
	t.Logf("✓ Response time: %v", duration)
}

// TestBenchmark_ToolCalling_Google tests tool calling with Google Gemini
func TestBenchmark_ToolCalling_Google(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping benchmark test in short mode")
	}

	cfg := CoraConfig{
		GoogleAPIKey: "",
	}

	client := New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	// Define tools
	tools := []CoraTool{
		{
			Name:        "get_current_time",
			Description: "Get the current time in a timezone",
			ParametersSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"timezone": map[string]any{
						"type":        "string",
						"description": "The timezone name",
					},
				},
				"required": []string{"timezone"},
			},
		},
	}

	// Define handlers
	handlers := map[string]CoraToolHandler{
		"get_current_time": func(ctx context.Context, args map[string]any) (any, error) {
			timezone := args["timezone"].(string)
			return map[string]any{
				"timezone": timezone,
				"time":     time.Now().Format("15:04:05"),
			}, nil
		},
	}

	start := time.Now()
	resp, err := client.Text(ctx, TextRequest{
		Provider:     ProviderGoogle,
		Model:        "gemini-2.5-flash",
		Input:        "What time is it in UTC timezone?",
		Mode:         ModeToolCalling,
		Tools:        tools,
		ToolHandlers: handlers,
	})
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Google tool calling request failed: %v", err)
	}

	responseText := resp.Text
	if responseText == "" && resp.JSON != nil {
		// If text is empty but JSON is present, try to extract from JSON
		if jsonBytes, err := json.Marshal(resp.JSON); err == nil {
			responseText = string(jsonBytes)
		}
	}

	t.Logf("✓ Google tool calling response: %q", responseText)
	t.Logf("✓ Response time: %v", duration)
	if resp.PromptTokens != nil || resp.CompletionTokens != nil {
		promptTokens := 0
		completionTokens := 0
		if resp.PromptTokens != nil {
			promptTokens = *resp.PromptTokens
		}
		if resp.CompletionTokens != nil {
			completionTokens = *resp.CompletionTokens
		}
		t.Logf("✓ Tokens - Prompt: %d, Completion: %d", promptTokens, completionTokens)
	}
}

// TestBenchmark_ComparisonReport generates a comparison report
func TestBenchmark_ComparisonReport(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping benchmark test in short mode")
	}

	type BenchmarkResult struct {
		Provider     string
		Model        string
		TestType     string
		Duration     time.Duration
		Success      bool
		ResponseLen  int
		TokensUsed   int
		ErrorMessage string
	}

	results := []BenchmarkResult{}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Test configurations
	tests := []struct {
		name     string
		client   *Client
		provider Provider
		model    string
	}{
		{
			name: "X.AI Grok",
			client: New(CoraConfig{
				OpenAIAPIKey:  "",
				OpenAIBaseURL: "https://api.x.ai/v1",
			}),
			provider: ProviderOpenAI,
			model:    "grok-4-fast-reasoning",
		},
		{
			name: "Google Gemini",
			client: New(CoraConfig{
				GoogleAPIKey: "",
			}),
			provider: ProviderGoogle,
			model:    "gemini-2.5-flash",
		},
	}

	testPrompts := []struct {
		name  string
		input string
	}{
		{"Simple Math", "What is 2+2?"},
		{"Explanation", "Explain gravity briefly."},
		{"Creative", "Write a haiku about coding."},
	}

	// Run tests
	for _, test := range tests {
		for _, prompt := range testPrompts {
			start := time.Now()
			resp, err := test.client.Text(ctx, TextRequest{
				Provider: test.provider,
				Model:    test.model,
				Input:    prompt.input,
				Mode:     ModeBasic,
			})
			duration := time.Since(start)

			result := BenchmarkResult{
				Provider: test.name,
				Model:    test.model,
				TestType: prompt.name,
				Duration: duration,
				Success:  err == nil,
			}

			if err != nil {
				result.ErrorMessage = err.Error()
			} else {
				result.ResponseLen = len(resp.Text)
				if resp.TotalTokens != nil {
					result.TokensUsed = *resp.TotalTokens
				}
			}

			results = append(results, result)
		}
	}

	// Generate report
	separator := strings.Repeat("=", 80)
	t.Logf("\n%s", separator)
	t.Logf("BENCHMARK COMPARISON REPORT")
	t.Logf("%s\n", separator)

	for _, r := range results {
		status := "✓"
		if !r.Success {
			status = "✗"
		}
		t.Logf("%s %s | %s | %s: %v (len=%d, tokens=%d)",
			status, r.Provider, r.Model, r.TestType, r.Duration, r.ResponseLen, r.TokensUsed)
		if r.ErrorMessage != "" {
			t.Logf("   Error: %s", r.ErrorMessage)
		}
	}

	t.Logf("\n%s", separator)

	// Calculate statistics
	var xaiTotalDuration, googleTotalDuration time.Duration
	xaiSuccess, googleSuccess := 0, 0
	xaiTotal, googleTotal := 0, 0

	for _, r := range results {
		if r.Provider == "X.AI Grok" {
			xaiTotal++
			xaiTotalDuration += r.Duration
			if r.Success {
				xaiSuccess++
			}
		} else {
			googleTotal++
			googleTotalDuration += r.Duration
			if r.Success {
				googleSuccess++
			}
		}
	}

	t.Logf("SUMMARY:")
	if xaiTotal > 0 {
		t.Logf("  X.AI Grok: %d/%d success, avg time: %v",
			xaiSuccess, xaiTotal, xaiTotalDuration/time.Duration(xaiTotal))
	}
	if googleTotal > 0 {
		t.Logf("  Google Gemini: %d/%d success, avg time: %v",
			googleSuccess, googleTotal, googleTotalDuration/time.Duration(googleTotal))
	}
	t.Logf("%s\n", separator)
}
