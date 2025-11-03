package cora

import (
	"context"
	"fmt"
)

// ToolExecutor handles tool call execution with configurable behavior.
type ToolExecutor struct {
	handlers    map[string]CoraToolHandler
	maxRounds   int  // Maximum tool call rounds (prevents infinite loops)
	parallel    bool // Execute multiple tool calls in parallel
	stopOnError bool // Stop execution on first error
}

// NewToolExecutor creates a tool executor with default settings.
func NewToolExecutor(handlers map[string]CoraToolHandler) *ToolExecutor {
	return &ToolExecutor{
		handlers:    handlers,
		maxRounds:   5, // Default max rounds
		parallel:    false,
		stopOnError: true,
	}
}

// WithMaxRounds sets the maximum number of tool call rounds.
func (te *ToolExecutor) WithMaxRounds(max int) *ToolExecutor {
	te.maxRounds = max
	return te
}

// WithParallel enables parallel execution of multiple tool calls.
func (te *ToolExecutor) WithParallel(parallel bool) *ToolExecutor {
	te.parallel = parallel
	return te
}

// WithStopOnError controls whether to stop on first error.
func (te *ToolExecutor) WithStopOnError(stop bool) *ToolExecutor {
	te.stopOnError = stop
	return te
}

// toolCallResult holds the result of a single tool invocation.
type toolCallResult struct {
	name   string
	result any
	err    error
}

// executeBatch runs multiple tool calls, respecting parallel/serial execution mode.
func (te *ToolExecutor) executeBatch(ctx context.Context, calls []toolCallRequest) ([]toolCallResult, error) {
	if len(calls) == 0 {
		return nil, nil
	}

	if te.parallel {
		return te.executeParallel(ctx, calls)
	}
	return te.executeSerial(ctx, calls)
}

type toolCallRequest struct {
	name string
	args map[string]any
}

func (te *ToolExecutor) executeSerial(ctx context.Context, calls []toolCallRequest) ([]toolCallResult, error) {
	results := make([]toolCallResult, len(calls))

	for i, call := range calls {
		handler, ok := te.handlers[call.name]
		if !ok {
			err := fmt.Errorf("no handler for tool %q", call.name)
			results[i] = toolCallResult{name: call.name, err: err}
			if te.stopOnError {
				return results, err
			}
			continue
		}

		result, err := handler(ctx, call.args)
		results[i] = toolCallResult{name: call.name, result: result, err: err}

		if err != nil && te.stopOnError {
			return results, fmt.Errorf("tool %q failed: %w", call.name, err)
		}
	}

	return results, nil
}

func (te *ToolExecutor) executeParallel(ctx context.Context, calls []toolCallRequest) ([]toolCallResult, error) {
	results := make([]toolCallResult, len(calls))
	errChan := make(chan error, len(calls))
	doneChan := make(chan struct{}, len(calls))

	for i, call := range calls {
		i, call := i, call // capture loop variables
		go func() {
			handler, ok := te.handlers[call.name]
			if !ok {
				err := fmt.Errorf("no handler for tool %q", call.name)
				results[i] = toolCallResult{name: call.name, err: err}
				errChan <- err
				doneChan <- struct{}{}
				return
			}

			result, err := handler(ctx, call.args)
			results[i] = toolCallResult{name: call.name, result: result, err: err}
			if err != nil {
				errChan <- fmt.Errorf("tool %q failed: %w", call.name, err)
			}
			doneChan <- struct{}{}
		}()
	}

	// Wait for all to complete
	for i := 0; i < len(calls); i++ {
		<-doneChan
	}
	close(errChan)

	// Check for errors
	if te.stopOnError {
		for err := range errChan {
			return results, err
		}
	}

	return results, nil
}
