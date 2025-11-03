package cora

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

func (so *streamOrchestrator) streamOpenAI(p *openAIProvider) error {
	msgs := make([]openai.ChatCompletionMessage, 0, 4)

	if strings.TrimSpace(so.req.System) != "" {
		msgs = append(msgs, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: so.req.System,
		})
	}

	msgs = append(msgs, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: so.req.Input,
	})

	req := openai.ChatCompletionRequest{
		Model:    so.model,
		Messages: msgs,
		Stream:   true,
	}

	if so.req.Temperature != nil {
		req.Temperature = *so.req.Temperature
	}
	if so.req.MaxOutputTokens != nil {
		req.MaxCompletionTokens = *so.req.MaxOutputTokens
	}

	// Add tools if provided
	if len(so.req.Tools) > 0 {
		req.Tools = make([]openai.Tool, len(so.req.Tools))
		for i, t := range so.req.Tools {
			req.Tools[i] = openai.Tool{
				Type: openai.ToolTypeFunction,
				Function: &openai.FunctionDefinition{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.ParametersSchema,
				},
			}
		}
	}

	// Include usage if requested
	if so.opts.IncludeUsage {
		req.StreamOptions = &openai.StreamOptions{IncludeUsage: true}
	}

	stream, err := p.client.CreateChatCompletionStream(so.ctx, req)
	if err != nil {
		return err
	}
	defer stream.Close()

	// Track tool calls being built incrementally
	toolCalls := make(map[int]*openai.ToolCall)

	for {
		response, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		if len(response.Choices) == 0 {
			continue
		}

		choice := response.Choices[0]
		delta := choice.Delta

		// Handle text chunks
		if delta.Content != "" {
			so.sendChunk(delta.Content)
		}

		// Handle tool calls (incremental)
		if len(delta.ToolCalls) > 0 {
			for _, tc := range delta.ToolCalls {
				idx := *tc.Index
				if _, exists := toolCalls[idx]; !exists {
					toolCalls[idx] = &openai.ToolCall{
						Index: tc.Index,
						ID:    tc.ID,
						Type:  tc.Type,
						Function: openai.FunctionCall{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					}
				} else {
					// Append arguments incrementally
					toolCalls[idx].Function.Arguments += tc.Function.Arguments
				}
			}
		}

		// Handle usage metadata
		if response.Usage != nil {
			so.sendUsage(&StreamUsage{
				PromptTokens:     response.Usage.PromptTokens,
				CompletionTokens: response.Usage.CompletionTokens,
				TotalTokens:      response.Usage.TotalTokens,
			})
		}

		// Check finish reason
		if choice.FinishReason == "tool_calls" && len(toolCalls) > 0 {
			if err := so.handleOpenAIToolCalls(p, toolCalls, &msgs); err != nil {
				return err
			}
			// Reset for next round
			toolCalls = make(map[int]*openai.ToolCall)
		}
	}

	return nil
}

func (so *streamOrchestrator) handleOpenAIToolCalls(
	p *openAIProvider,
	toolCalls map[int]*openai.ToolCall,
	msgs *[]openai.ChatCompletionMessage,
) error {
	// Convert map to slice
	calls := make([]openai.ToolCall, len(toolCalls))
	for idx, tc := range toolCalls {
		calls[idx] = *tc
	}

	// Parse and execute each tool call
	for _, tc := range calls {
		var args map[string]any
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			return fmt.Errorf("invalid tool call args for %s: %w", tc.Function.Name, err)
		}

		// Send tool call request event
		so.sendToolCallRequest(&StreamToolCall{
			ID:           tc.ID,
			Name:         tc.Function.Name,
			Arguments:    args,
			ArgumentsRaw: tc.Function.Arguments,
		})

		// Execute tool based on mode
		var result any
		var execErr error

		switch so.opts.ToolExecutionMode {
		case ToolExecutionAuto, ToolExecutionParallel:
			handler, ok := so.req.ToolHandlers[tc.Function.Name]
			if !ok {
				return fmt.Errorf("no handler for tool %s", tc.Function.Name)
			}
			result, execErr = handler(so.ctx, args)

		case ToolExecutionPause:
			result, execErr = so.waitForToolResult(tc.ID)
		}

		// Send tool result event
		so.sendToolCallResult(&StreamToolResult{
			ToolCallID: tc.ID,
			Name:       tc.Function.Name,
			Result:     result,
			Err:        execErr,
		})

		if execErr != nil && so.opts.ToolExecutionMode != ToolExecutionPause {
			return execErr
		}

		// Append tool result to conversation
		resultJSON, _ := json.Marshal(result)
		*msgs = append(*msgs, openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			Content:    string(resultJSON),
			ToolCallID: tc.ID,
		})
	}

	// Continue stream with tool results
	// (Simplified - would need to create a new stream request here)
	return nil
}