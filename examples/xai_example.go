package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/oraraka-deko/cora/cora"
)

func main() {
	// Configure the client with X.AI API
	cfg := cora.CoraConfig{
		OpenAIAPIKey:       "",
		OpenAIBaseURL:      "",
		DefaultModelOpenAI: "grok-3",
	}

	client := cora.New(cfg)

	// Example 1: Basic chat
	fmt.Println("=== Example 1: Basic Chat ===")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Text(ctx, cora.TextRequest{
		Provider: cora.ProviderOpenAI,
		Model:    "grok-3",
		Input:    "What is the meaning of life in 10 words or less?",
		Mode:     cora.ModeBasic,
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	fmt.Printf("Response: %s\n\n", resp.Text)

	// Example 2: With system prompt
	fmt.Println("=== Example 2: With System Prompt ===")
	resp2, err := client.Text(context.Background(), cora.TextRequest{
		Provider: cora.ProviderOpenAI,
		Model:    "grok-3",
		System:   "You are a wise philosopher. Speak concisely.",
		Input:    "What is consciousness?",
		Mode:     cora.ModeBasic,
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	fmt.Printf("Response: %s\n\n", resp2.Text)

	// Example 3: Structured JSON
	fmt.Println("=== Example 3: Structured JSON Output ===")
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{
				"type": "string",
			},
			"author": map[string]any{
				"type": "string",
			},
			"year": map[string]any{
				"type": "number",
			},
		},
		"required": []string{"title", "author", "year"},
	}

	resp3, err := client.Text(context.Background(), cora.TextRequest{
		Provider:       cora.ProviderOpenAI,
		Model:          "grok-3",
		Input:          "Create a book: '1984' by George Orwell published in 1949",
		Mode:           cora.ModeStructuredJSON,
		ResponseSchema: schema,
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	fmt.Printf("JSON Response: %+v\n\n", resp3.JSON)

	// Example 4: With temperature control
	fmt.Println("=== Example 4: Temperature Control ===")
	temp := float32(0.3) // Low temperature for more deterministic output
	maxTokens := 30

	resp4, err := client.Text(context.Background(), cora.TextRequest{
		Provider:        cora.ProviderOpenAI,
		Model:           "grok-3",
		Input:           "Count from 1 to 5",
		Mode:            cora.ModeBasic,
		Temperature:     &temp,
		MaxOutputTokens: &maxTokens,
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	fmt.Printf("Response (temp=%.1f): %s\n", temp, resp4.Text)

	// Display token usage
	if resp4.TotalTokens != nil {
		fmt.Printf("Total tokens used: %d\n", *resp4.TotalTokens)
	}
}
