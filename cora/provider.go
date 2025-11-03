package cora

import "context"

// providerClient is the internal interface each backend implements.
type providerClient interface {
	// Text executes a single-turn request according to the given call plan.
	Text(ctx context.Context, plan callPlan) (callResult, error)
}

// callPlan is a normalized, provider-agnostic instruction set produced by the
// high-level Text() method based on TextRequest and Mode orchestration.
type callPlan struct {
	Provider Provider
	Model    string
	// Input/system for this call. Two-step mode may generate multiple plans.
	System string
	Input  string

	// Options
	Temperature     *float32
	MaxOutputTokens *int
	Labels          map[string]string

	// Structured JSON
	ResponseSchema map[string]any
	Structured     bool

	// Tool calling
	Tools        []CoraTool
	ToolHandlers map[string]CoraToolHandler

	// Two-step specific flag to apply proofreading prompt for this call
	Proofread bool
}

// callResult is the provider-agnostic result of one call execution.
type callResult struct {
	Text string
	JSON map[string]any

	PromptTokens     *int
	CompletionTokens *int
	TotalTokens      *int

	// toolLoop indicates provider detected tool calls and cora executed one follow-up round.
	toolLoop bool
}