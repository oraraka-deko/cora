package cora

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

// executeToolLoop handles multi-round tool calling for Google.
func (p *googleProvider) executeToolLoop(ctx context.Context, model string, contents any, cfg *genai.GenerateContentConfig, plan callPlan) (callResult, error) {
	executor := NewToolExecutor(plan.ToolHandlers).
		WithMaxRounds(5).
		WithParallel(false).
		WithStopOnError(true).
		WithValidator(plan.Tools)

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
