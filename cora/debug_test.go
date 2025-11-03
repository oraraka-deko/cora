package cora

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestDebug_GoogleToolResponse(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping debug test in short mode")
	}

	cfg := CoraConfig{
		GoogleAPIKey: "",
	}

	client := New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Simple tool
	tools := []CoraTool{
		{
			Name:        "get_time",
			Description: "Get current time",
			ParametersSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}

	handlers := map[string]CoraToolHandler{
		"get_time": func(ctx context.Context, args map[string]any) (any, error) {
			return map[string]any{
				"time": time.Now().Format("15:04:05"),
			}, nil
		},
	}

	resp, err := client.Text(ctx, TextRequest{
		Provider:     ProviderGoogle,
		Model:        "gemini-2.5-flash",
		Input:        "Use the get_time tool to find out what time it is, then tell me the result.",
		Mode:         ModeToolCalling,
		Tools:        tools,
		ToolHandlers: handlers,
	})

	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	t.Logf("Response Text: %q", resp.Text)
	if resp.JSON != nil {
		jsonBytes, _ := json.MarshalIndent(resp.JSON, "", "  ")
		t.Logf("Response JSON:\n%s", jsonBytes)
	}
	t.Logf("Tokens: Prompt=%v, Completion=%v, Total=%v",
		ptrInt(resp.PromptTokens),
		ptrInt(resp.CompletionTokens),
		ptrInt(resp.TotalTokens))
}

func ptrInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
