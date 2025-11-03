package cora

import (
	"context"
	"fmt"
	"time"
)

// ToolExecutor handles tool call execution with configurable behavior.
type ToolExecutor struct {
	handlers    map[string]CoraToolHandler
	maxRounds   int
	parallel    bool
	stopOnError bool
	cache       *ToolCache
	validator   *ToolValidator
	retryConfig *RetryConfig
	
	// Metrics
	totalCalls      int
	successfulCalls int
	failedCalls     int
	cachedCalls     int
}

// NewToolExecutor creates a tool executor with default settings.
func NewToolExecutor(handlers map[string]CoraToolHandler) *ToolExecutor {
	return &ToolExecutor{
		handlers:    handlers,
		maxRounds:   5,
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

// WithCache enables result caching with the specified TTL and max size.
func (te *ToolExecutor) WithCache(ttl time.Duration, maxSize int) *ToolExecutor {
	te.cache = NewToolCache(ttl, maxSize)
	return te
}

// WithValidator enables argument validation using tool schemas.
func (te *ToolExecutor) WithValidator(tools []CoraTool) *ToolExecutor {
	te.validator = NewToolValidator(tools)
	return te
}

// WithRetry enables retry logic for tool execution.
func (te *ToolExecutor) WithRetry(config RetryConfig) *ToolExecutor {
	te.retryConfig = &config
	
	// Wrap all handlers with retry logic
	for name, handler := range te.handlers {
		te.handlers[name] = RetryableToolHandler(handler, config)
	}
	return te
}

// toolCallResult holds the result of a single tool invocation.
type toolCallResult struct {
	name   string
	result any
	err    error
	cached bool
}

// executeBatch runs multiple tool calls, respecting parallel/serial execution mode.
func (te *ToolExecutor) executeBatch(ctx context.Context, calls []toolCallRequest) ([]toolCallResult, error) {
	if len(calls) == 0 {
		return nil, nil
	}

	// Update metrics
	te.totalCalls += len(calls)

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
		result, err := te.executeSingleCall(ctx, call)
		results[i] = result

		if err != nil {
			te.failedCalls++
			if te.stopOnError {
				return results, fmt.Errorf("tool %q failed: %w", call.name, err)
			}
		} else {
			te.successfulCalls++
		}
	}

	return results, nil
}

func (te *ToolExecutor) executeParallel(ctx context.Context, calls []toolCallRequest) ([]toolCallResult, error) {
	results := make([]toolCallResult, len(calls))
	errChan := make(chan error, len(calls))
	doneChan := make(chan struct{}, len(calls))

	for i, call := range calls {
		i, call := i, call
		go func() {
			result, err := te.executeSingleCall(ctx, call)
			results[i] = result
			
			if err != nil {
				te.failedCalls++
				errChan <- fmt.Errorf("tool %q failed: %w", call.name, err)
			} else {
				te.successfulCalls++
			}
			doneChan <- struct{}{}
		}()
	}

	// Wait for all to complete
	for i := 0; i < len(calls); i++ {
		<-doneChan
	}
	close(errChan)

	if te.stopOnError {
		for err := range errChan {
			return results, err
		}
	}

	return results, nil
}

func (te *ToolExecutor) executeSingleCall(ctx context.Context, call toolCallRequest) (toolCallResult, error) {
	// 1. Validate arguments if validator is configured
	if te.validator != nil {
		if err := te.validator.ValidateCall(call.name, call.args); err != nil {
			return toolCallResult{name: call.name, err: err}, err
		}
	}

	// 2. Check cache if enabled
	if te.cache != nil {
		if result, err, found := te.cache.Get(call.name, call.args); found {
			te.cachedCalls++
			return toolCallResult{name: call.name, result: result, err: err, cached: true}, err
		}
	}

	// 3. Execute handler
	handler, ok := te.handlers[call.name]
	if !ok {
		err := fmt.Errorf("no handler for tool %q", call.name)
		return toolCallResult{name: call.name, err: err}, err
	}

	result, err := handler(ctx, call.args)

	// 4. Store in cache if enabled
	if te.cache != nil {
		te.cache.Set(call.name, call.args, result, err)
	}

	return toolCallResult{name: call.name, result: result, err: err}, err
}

// Metrics returns execution statistics.
func (te *ToolExecutor) Metrics() ToolExecutorMetrics {
	metrics := ToolExecutorMetrics{
		TotalCalls:      te.totalCalls,
		SuccessfulCalls: te.successfulCalls,
		FailedCalls:     te.failedCalls,
		CachedCalls:     te.cachedCalls,
	}

	if te.cache != nil {
		hits, misses := te.cache.Stats()
		metrics.CacheHits = int(hits)
		metrics.CacheMisses = int(misses)
		if hits+misses > 0 {
			metrics.CacheHitRate = float64(hits) / float64(hits+misses)
		}
	}

	if metrics.TotalCalls > 0 {
		metrics.SuccessRate = float64(metrics.SuccessfulCalls) / float64(metrics.TotalCalls)
	}

	return metrics
}

type ToolExecutorMetrics struct {
	TotalCalls      int
	SuccessfulCalls int
	FailedCalls     int
	CachedCalls     int
	CacheHits       int
	CacheMisses     int
	CacheHitRate    float64
	SuccessRate     float64
}