package cora

import (
	"fmt"
	"strings"

	"google.golang.org/genai"
)

func (so *streamOrchestrator) streamGoogle(p *googleProvider) error {
	cfg := &genai.GenerateContentConfig{}

	if strings.TrimSpace(so.req.System) != "" {
		cfg.SystemInstruction = &genai.Content{
			Parts: []*genai.Part{{Text: so.req.System}},
		}
	}

	if so.req.Temperature != nil {
		cfg.Temperature = genai.Ptr(*so.req.Temperature)
	}
	if so.req.MaxOutputTokens != nil {
		cfg.MaxOutputTokens = int32(*so.req.MaxOutputTokens)
	}

	// Add tools
	if len(so.req.Tools) > 0 {
		cfg.Tools = toGenAITools(so.req.Tools)
		cfg.ToolConfig = &genai.ToolConfig{
			FunctionCallingConfig: &genai.FunctionCallingConfig{
				Mode: genai.FunctionCallingConfigModeAny,
			},
		}
	}

	// Start streaming
	for result, err := range p.client.Models.GenerateContentStream(
		so.ctx,
		so.model,
		genai.Text(so.req.Input),
		cfg,
	) {
		if err != nil {
			return err
		}

		// Send text chunks
		if text := result.Text(); text != "" {
			so.sendChunk(text)
		}

		// Handle tool calls
		if fcs := result.FunctionCalls(); len(fcs) > 0 {
			if err := so.handleGoogleToolCalls(p, fcs); err != nil {
				return err
			}
		}

		// Send usage
		if result.UsageMetadata != nil {
			so.sendUsage(&StreamUsage{
				PromptTokens:     int(result.UsageMetadata.PromptTokenCount),
				CompletionTokens: int(result.UsageMetadata.CandidatesTokenCount),
				TotalTokens:      int(result.UsageMetadata.TotalTokenCount),
			})
		}
	}

	return nil
}

func (so *streamOrchestrator) handleGoogleToolCalls(
	p *googleProvider,
	calls []*genai.FunctionCall,
) error {
	for _, fc := range calls {
		// Send tool call request
		so.sendToolCallRequest(&StreamToolCall{
			ID:        fc.Name, // Google doesn't provide ID in stream
			Name:      fc.Name,
			Arguments: fc.Args,
		})

		// Execute tool
		var result any
		var execErr error

		switch so.opts.ToolExecutionMode {
		case ToolExecutionAuto, ToolExecutionParallel:
			handler, ok := so.req.ToolHandlers[fc.Name]
			if !ok {
				return fmt.Errorf("no handler for tool %s", fc.Name)
			}
			result, execErr = handler(so.ctx, fc.Args)

		case ToolExecutionPause:
			result, execErr = so.waitForToolResult(fc.Name)
		}

		// Send tool result
		so.sendToolCallResult(&StreamToolResult{
			ToolCallID: fc.Name,
			Name:       fc.Name,
			Result:     result,
			Err:        execErr,
		})

		if execErr != nil {
			return execErr
		}
	}

	return nil
}