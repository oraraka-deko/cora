package cora

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"
)

// RetryConfig configures retry behavior for tool execution.
type RetryConfig struct {
	MaxAttempts     int
	InitialBackoff  time.Duration
	MaxBackoff      time.Duration
	BackoffMultiplier float64
	RetryableErrors []error // Specific errors that should trigger retry
}

var DefaultRetryConfig = RetryConfig{
	MaxAttempts:       3,
	InitialBackoff:    100 * time.Millisecond,
	MaxBackoff:        10 * time.Second,
	BackoffMultiplier: 2.0,
}

// RetryableToolHandler wraps a tool handler with retry logic.
func RetryableToolHandler(handler CoraToolHandler, config RetryConfig) CoraToolHandler {
	return func(ctx context.Context, args map[string]any) (any, error) {
		var lastErr error

		for attempt := 0; attempt < config.MaxAttempts; attempt++ {
			result, err := handler(ctx, args)
			if err == nil {
				return result, nil
			}

			lastErr = err

			// Check if error is retryable
			if !isRetryable(err, config.RetryableErrors) {
				return nil, fmt.Errorf("non-retryable error: %w", err)
			}

			// Check if we have more attempts
			if attempt < config.MaxAttempts-1 {
				backoff := calculateBackoff(attempt, config)
				
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(backoff):
					// Continue to next attempt
				}
			}
		}

		return nil, fmt.Errorf("max retry attempts (%d) exceeded: %w", config.MaxAttempts, lastErr)
	}
}

func isRetryable(err error, retryableErrors []error) bool {
	if len(retryableErrors) == 0 {
		// Default: retry on common transient errors
		return errors.Is(err, context.DeadlineExceeded) || 
			   errors.Is(err, context.Canceled)
	}

	for _, retryableErr := range retryableErrors {
		if errors.Is(err, retryableErr) {
			return true
		}
	}
	return false
}

func calculateBackoff(attempt int, config RetryConfig) time.Duration {
	backoff := float64(config.InitialBackoff) * math.Pow(config.BackoffMultiplier, float64(attempt))
	if backoff > float64(config.MaxBackoff) {
		backoff = float64(config.MaxBackoff)
	}
	return time.Duration(backoff)
}