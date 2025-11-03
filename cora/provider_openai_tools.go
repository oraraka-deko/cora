package cora

import (
	"context"
	"encoding/json"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

// executeToolLoop handles multi-round tool calling for OpenAI.
func (p *openAIProvider) executeToolLoop(ctx context.Context, req openai.ChatCompletionRequest, plan callPlan) (callResult, error) {
	executor := NewToolExecutor(plan.ToolHandlers).
		WithMaxRounds(5).
		WithParallel(false).
		WithStopOnError(true)

	msgs := req.Messages
	roundCount := 0

	for {
		roundCount++
		if roundCount > executor.maxRounds {
			return callResult{}, fmt.Errorf("exceeded maximum tool call rounds (%d)", executor.maxRounds)
		}

		// Make API call
		resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model:    req.Model,
			Messages: msgs,
			Tools:    req.Tools,
		})
		if err != nil {
			return callResult{}, err
		}

		if len(resp.Choices) == 0 {
			return callResult{}, fmt.Errorf("no choices in response")
		}

		choice := resp.Choices[0]

		// No tool calls, return final answer
		if len(choice.Message.ToolCalls) == 0 {
			return p.toCallResult(resp), nil
		}

		// Append assistant message with tool calls
		msgs = append(msgs, choice.Message)

		// Execute tool calls
		calls := make([]toolCallRequest, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			var args map[string]any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				return callResult{}, fmt.Errorf("invalid tool call args for %s: %w", tc.Function.Name, err)
			}
			calls[i] = toolCallRequest{name: tc.Function.Name, args: args}
		}

		results, err := executor.executeBatch(ctx, calls)
		if err != nil {
			return callResult{}, err
		}

		// Append tool results as messages
		for i, result := range results {
			resultJSON, _ := json.Marshal(result.result)
			msgs = append(msgs, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    string(resultJSON),
				Name:       result.name,
				ToolCallID: choice.Message.ToolCalls[i].ID,
			})
		}
	}
}