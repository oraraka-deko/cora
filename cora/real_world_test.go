package cora

import (
	"context"
	"testing"
	"time"
)

// TestRealWorld_XAI_BasicChat tests basic chat functionality with X.AI API
func TestRealWorld_XAI_BasicChat(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real-world test in short mode")
	}

	cfg := CoraConfig{
		OpenAIAPIKey:       "",
		OpenAIBaseURL:      "https://api.x.ai/v1",
		DefaultModelOpenAI: "grok-4-fast-reasoning",
	}

	client := New(cfg)
	if client == nil {
		t.Fatal("Failed to create client")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Text(ctx, TextRequest{
		Provider: ProviderOpenAI,
		Model:    "grok-4-fast-reasoning",
		Input:    "Say hello in exactly 5 words.",
		Mode:     ModeBasic,
	})

	if err != nil {
		t.Fatalf("Text request failed: %v", err)
	}

	if resp.Text == "" {
		t.Fatal("Expected non-empty response text")
	}

	t.Logf("✓ Response received: %q", resp.Text)
	t.Logf("✓ Model: %s", resp.Model)
	if resp.PromptTokens != nil {
		t.Logf("✓ Prompt tokens: %d", *resp.PromptTokens)
	}
	if resp.CompletionTokens != nil {
		t.Logf("✓ Completion tokens: %d", *resp.CompletionTokens)
	}
	if resp.TotalTokens != nil {
		t.Logf("✓ Total tokens: %d", *resp.TotalTokens)
	}
}

// TestRealWorld_XAI_WithTemperature tests temperature parameter
func TestRealWorld_XAI_WithTemperature(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real-world test in short mode")
	}

	cfg := CoraConfig{
		OpenAIAPIKey:  "",
		OpenAIBaseURL: "https://api.x.ai/v1",
	}

	client := New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	temp := float32(0.7)
	maxTokens := 50

	resp, err := client.Text(ctx, TextRequest{
		Provider:        ProviderOpenAI,
		Model:           "grok-4-fast-reasoning",
		Input:           "Explain quantum computing in one sentence.",
		Mode:            ModeBasic,
		Temperature:     &temp,
		MaxOutputTokens: &maxTokens,
	})

	if err != nil {
		t.Fatalf("Text request failed: %v", err)
	}

	if resp.Text == "" {
		t.Fatal("Expected non-empty response text")
	}

	t.Logf("✓ Response with temperature %.1f: %q", temp, resp.Text)
}

// TestRealWorld_XAI_WithSystemPrompt tests system message functionality
func TestRealWorld_XAI_WithSystemPrompt(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real-world test in short mode")
	}

	cfg := CoraConfig{
		OpenAIAPIKey:  "",
		OpenAIBaseURL: "",
	}

	client := New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Text(ctx, TextRequest{
		Provider: ProviderOpenAI,
		Model:    "grok-4-fast-reasoning",
		System:   "You are a helpful assistant that speaks like a pirate. Keep responses brief.",
		Input:    "What is 2+2?",
		Mode:     ModeBasic,
	})

	if err != nil {
		t.Fatalf("Text request failed: %v", err)
	}

	if resp.Text == "" {
		t.Fatal("Expected non-empty response text")
	}

	t.Logf("✓ Response with pirate system prompt: %q", resp.Text)
}

// TestRealWorld_XAI_StructuredJSON tests structured JSON output
func TestRealWorld_XAI_StructuredJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real-world test in short mode")
	}

	cfg := CoraConfig{
		OpenAIAPIKey:  "",
		OpenAIBaseURL: "https://api.x.ai/v1",
	}

	client := New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Name of the person",
			},
			"age": map[string]any{
				"type":        "number",
				"description": "Age in years",
			},
			"city": map[string]any{
				"type":        "string",
				"description": "City where they live",
			},
		},
		"required": []string{"name", "age", "city"},
	}

	resp, err := client.Text(ctx, TextRequest{
		Provider:       ProviderOpenAI,
		Model:          "grok-4-fast-reasoning",
		Input:          "Generate a fictional person: John who is 30 years old and lives in New York",
		Mode:           ModeStructuredJSON,
		ResponseSchema: schema,
	})

	if err != nil {
		t.Fatalf("Structured JSON request failed: %v", err)
	}

	if resp.JSON == nil {
		t.Fatal("Expected JSON response")
	}

	t.Logf("✓ Structured JSON response: %+v", resp.JSON)

	// Verify expected fields
	if name, ok := resp.JSON["name"].(string); ok {
		t.Logf("✓ Name: %s", name)
	} else {
		t.Error("Expected 'name' field in JSON response")
	}

	if age, ok := resp.JSON["age"].(float64); ok {
		t.Logf("✓ Age: %.0f", age)
	} else {
		t.Error("Expected 'age' field in JSON response")
	}

	if city, ok := resp.JSON["city"].(string); ok {
		t.Logf("✓ City: %s", city)
	} else {
		t.Error("Expected 'city' field in JSON response")
	}
}

// TestRealWorld_XAI_MultipleRequests tests multiple sequential requests
func TestRealWorld_XAI_MultipleRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real-world test in short mode")
	}

	cfg := CoraConfig{
		OpenAIAPIKey:  "",
		OpenAIBaseURL: "https://api.x.ai/v1",
	}

	client := New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	questions := []string{
		"What is 5+5?",
		"Name a color.",
		"What day comes after Monday?",
	}

	for i, question := range questions {
		resp, err := client.Text(ctx, TextRequest{
			Provider: ProviderOpenAI,
			Model:    "grok-4-fast-reasoning",
			Input:    question,
			Mode:     ModeBasic,
		})

		if err != nil {
			t.Fatalf("Request %d failed: %v", i+1, err)
		}

		if resp.Text == "" {
			t.Fatalf("Request %d returned empty response", i+1)
		}

		t.Logf("✓ Q%d: %q → A: %q", i+1, question, resp.Text)
	}

	t.Logf("✓ All %d requests completed successfully", len(questions))
}

// TestRealWorld_XAI_ErrorHandling tests error handling with invalid model
func TestRealWorld_XAI_ErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real-world test in short mode")
	}

	cfg := CoraConfig{
		OpenAIAPIKey:  "",
		OpenAIBaseURL: "https://api.x.ai/v1",
	}

	client := New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := client.Text(ctx, TextRequest{
		Provider: ProviderOpenAI,
		Model:    "nonexistent-model-xyz",
		Input:    "This should fail",
		Mode:     ModeBasic,
	})

	if err == nil {
		t.Fatal("Expected error for invalid model, but got none")
	}

	t.Logf("✓ Error handling works correctly: %v", err)
}
