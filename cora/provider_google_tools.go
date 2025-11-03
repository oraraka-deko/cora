package cora

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

// executeToolLoop handles multi-round tool calling for Google.
func (p *googleProvider) executeToolLoop(ctx context.Context, model string, contents any, cfg *genai.GenerateContentConfig, plan callPlan) (callResult, error) {
	// Configure executor with plan settings or defaults
	maxRounds := 5
	if plan.MaxToolRounds != nil {
		maxRounds = *plan.MaxToolRounds
	}

	parallelTools := false
	if plan.ParallelTools != nil {
		parallelTools = *plan.ParallelTools
	}

	stopOnError := true
	if plan.StopOnToolError != nil {
		stopOnError = *plan.StopOnToolError
	}

	executor := NewToolExecutor(plan.ToolHandlers).
		WithMaxRounds(maxRounds).
		WithParallel(parallelTools).
		WithStopOnError(stopOnError).
		WithValidator(plan.Tools)

	// Apply cache if configured
	if plan.ToolCacheTTL > 0 && plan.ToolCacheMaxSize > 0 {
		executor = executor.WithCache(plan.ToolCacheTTL, plan.ToolCacheMaxSize)
	}

	// Apply retry if configured
	if plan.ToolRetryConfig != nil {
		executor = executor.WithRetry(*plan.ToolRetryConfig)
	}

	roundCount := 0

	// Convert initial contents to proper type
	var currentContents []*genai.Content
	if c, ok := contents.([]*genai.Content); ok {
		currentContents = c
	} else {
		// If contents is not already the right type, wrap it
		currentContents = []*genai.Content{
			{Parts: []*genai.Part{{Text: fmt.Sprintf("%v", contents)}}},
		}
	}

	for {
		roundCount++
		if roundCount > executor.maxRounds {
			return callResult{}, fmt.Errorf("exceeded maximum tool call rounds (%d)", executor.maxRounds)
		}

		res, err := p.client.Models.GenerateContent(ctx, model, currentContents, cfg)
		if err != nil {
			return callResult{}, err
		}

		fcs := res.FunctionCalls()
		if len(fcs) == 0 {
			return toCallResultFromGenAI(res), nil
		}

		// Execute function calls
		calls := make([]toolCallRequest, len(fcs))
		for i, fc := range fcs {
			calls[i] = toolCallRequest{name: fc.Name, args: fc.Args}
		}

		results, err := executor.executeBatch(ctx, calls)
		if err != nil {
			return callResult{}, err
		}

		// Build function response content
		respContent := &genai.Content{Role: "user"}
		for i, result := range results {
			payload, _ := normalizeJSON(result.result)
			respContent.Parts = append(respContent.Parts, &genai.Part{
				FunctionResponse: &genai.FunctionResponse{
					Name:     fcs[i].Name,
					Response: payload,
				},
			})
		}

		// Append model response and function response to history
		// Google requires: [previous messages, model's function call, user's function response]
		currentContents = append(currentContents, res.Candidates[0].Content, respContent)
	}
}
