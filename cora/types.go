package cora

import (
	"context"
	"encoding/json"
)

// Provider identifies which backend to use. No auto-detection in this step.
type Provider string

const (
	ProviderOpenAI Provider = "openai"
	ProviderGoogle Provider = "google"
)

// TextMode selects orchestration/preset behavior for Text().
type TextMode int

const (
	// ModeBasic sends the input as-is to the provider and returns the assistant's text.
	ModeBasic TextMode = iota
	// ModeStructuredJSON requests a structured JSON response conforming to ResponseSchema.
	ModeStructuredJSON
	// ModeToolCalling enables tool/function calling. The model can request tool invocations;
	// cora will call your handlers and return a final model answer automatically.
	ModeToolCalling
	// ModeTwoStepEnhance first rewrites/cleans the user's input (spelling/grammar/clarity),
	// then sends the improved text for the main response.
	ModeTwoStepEnhance
)

// CoraTool declares a callable function the model may request.
type CoraTool struct {
	// Name is the unique function name referenced by the model.
	Name string
	// Description helps the model understand when to call this tool.
	Description string
	// ParametersSchema is a JSON Schema Object (draft subset).
	// Keep it provider-agnostic; cora maps it to each provider's format.
	ParametersSchema map[string]any
}

// CoraToolHandler is invoked when the model requests a tool call.
type CoraToolHandler func(ctx context.Context, args map[string]any) (any, error)

// TextRequest is the unified request for text-style generations.
type TextRequest struct {
	// Provider and Model must be set explicitly in this step.
	Provider Provider
	Model    string

	// Input and optional system instruction.
	Input  string
	System string

	// Mode selects orchestration behavior (see TextMode).
	Mode TextMode

	// Optional response shaping.
	Temperature     *float32
	MaxOutputTokens *int

	// Structured outputs (ModeStructuredJSON).
	// Provide a JSON schema that defines the shape of the response object.
	ResponseSchema map[string]any

	// Tool calling (ModeToolCalling).
	Tools        []CoraTool
	ToolHandlers map[string]CoraToolHandler

	// Tool execution configuration (optional, used with ModeToolCalling).
	MaxToolRounds  *int  // Maximum number of tool call rounds (default: 5)
	ParallelTools  *bool // Execute multiple tool calls in parallel (default: false)
	StopOnToolError *bool // Stop execution on first tool error (default: true)

	// Arbitrary per-call labels/metadata (carried provider-side if supported).
	Labels map[string]string
}

// TextResponse is a provider-agnostic result from Text().
type TextResponse struct {
	Provider Provider
	Model    string
	Mode     TextMode

	// Primary text content (if available).
	Text string

	// If ModeStructuredJSON and the provider returned a structured object,
	// JSON contains the parsed object.
	JSON map[string]any

	// Token usage, if available.
	PromptTokens     *int
	CompletionTokens *int
	TotalTokens      *int
}

// rawJSONSchema is a thin json.Marshaler wrapper to pass generic schemas
// into providers that take custom types implementing MarshalJSON.
type rawJSONSchema struct {
	m map[string]any
}

func (r rawJSONSchema) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.m)
}