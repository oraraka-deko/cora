package cora

import (
	"context"
	"time"
)

// StreamRequest configures a streaming text generation.
type StreamRequest struct {
	Provider Provider
	Model    string

	Input  string
	System string

	// Generation parameters
	Temperature     *float32
	MaxOutputTokens *int

	// Tool support in streams
	Tools        []CoraTool
	ToolHandlers map[string]CoraToolHandler

	// Stream-specific options
	StreamOptions StreamOptions
}

// StreamOptions controls streaming behavior.
type StreamOptions struct {
	// BufferSize sets the channel buffer size for events (default: 100)
	BufferSize int

	// IncludeUsage requests usage metadata in the final event
	IncludeUsage bool

	// FlushInterval sets minimum time between chunk deliveries (rate limiting)
	FlushInterval time.Duration

	// EnableToolExecution allows automatic tool execution within the stream
	EnableToolExecution bool

	// ToolExecutionMode controls how tools are executed
	ToolExecutionMode ToolExecutionMode
}

// ToolExecutionMode determines tool execution strategy during streaming.
type ToolExecutionMode int

const (
	// ToolExecutionAuto automatically executes tools and continues streaming
	ToolExecutionAuto ToolExecutionMode = iota
	// ToolExecutionPause pauses streaming, waits for manual tool result submission
	ToolExecutionPause
	// ToolExecutionParallel executes multiple tool calls in parallel during stream
	ToolExecutionParallel
)

// StreamEvent represents a single event in the stream.
type StreamEvent struct {
	Type StreamEventType

	// Text content (for EventTypeChunk)
	Text string

	// Tool call request (for EventTypeToolCallRequest)
	ToolCall *StreamToolCall

	// Tool call result (for EventTypeToolCallResult)
	ToolResult *StreamToolResult

	// Metadata (for EventTypeDone)
	Usage *StreamUsage
	Model string

	// Error (for EventTypeError)
	Err error

	// Internal fields
	provider  Provider
	timestamp time.Time
}

// StreamEventType identifies the event kind.
type StreamEventType int

const (
	// EventTypeChunk is a text token chunk
	EventTypeChunk StreamEventType = iota
	// EventTypeToolCallRequest indicates model wants to call a tool
	EventTypeToolCallRequest
	// EventTypeToolCallResult contains a completed tool execution result
	EventTypeToolCallResult
	// EventTypeUsage contains token usage metadata
	EventTypeUsage
	// EventTypeDone signals stream completion
	EventTypeDone
	// EventTypeError signals an error occurred
	EventTypeError
)

// StreamToolCall represents a tool invocation request from the model.
type StreamToolCall struct {
	ID          string
	Name        string
	Arguments   map[string]any
	ArgumentsRaw string // Raw JSON before parsing
}

// StreamToolResult contains the result of a tool execution.
type StreamToolResult struct {
	ToolCallID string
	Name       string
	Result     any
	Err        error
}

// StreamUsage contains token usage information.
type StreamUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// StreamResponse provides control over an active stream.
type StreamResponse struct {
	Events <-chan StreamEvent
	Cancel context.CancelFunc

	// SubmitToolResult manually submits a tool result (for ToolExecutionPause mode)
	SubmitToolResult func(toolCallID string, result any) error
}