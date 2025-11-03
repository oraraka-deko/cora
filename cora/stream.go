package cora

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Stream executes a streaming text generation request.
func (c *Client) Stream(ctx context.Context, req StreamRequest) (*StreamResponse, error) {
	if req.Provider != ProviderOpenAI && req.Provider != ProviderGoogle {
		return nil, fmt.Errorf("cora: unknown provider %q", req.Provider)
	}

	model := req.Model
	if model == "" {
		switch req.Provider {
		case ProviderOpenAI:
			model = c.cfg.DefaultModelOpenAI
		case ProviderGoogle:
			model = c.cfg.DefaultModelGoogle
		}
		if model == "" {
			return nil, fmt.Errorf("cora: model must be specified")
		}
	}

	// Apply defaults to stream options
	opts := req.StreamOptions
	if opts.BufferSize == 0 {
		opts.BufferSize = 100
	}

	// Create cancellable context
	streamCtx, cancel := context.WithCancel(ctx)

	// Create event channel
	events := make(chan StreamEvent, opts.BufferSize)

	// Create orchestrator
	orchestrator := &streamOrchestrator{
		ctx:      streamCtx,
		client:   c,
		req:      req,
		model:    model,
		opts:     opts,
		events:   events,
		cancel:   cancel,
		toolWait: make(map[string]chan any),
	}

	// Start streaming in background
	go orchestrator.run()

	return &StreamResponse{
		Events:           events,
		Cancel:           cancel,
		SubmitToolResult: orchestrator.submitToolResult,
	}, nil
}

// streamOrchestrator manages the lifecycle of a stream.
type streamOrchestrator struct {
	ctx    context.Context
	client *Client
	req    StreamRequest
	model  string
	opts   StreamOptions
	events chan StreamEvent
	cancel context.CancelFunc

	// Tool execution state
	toolWaitMu sync.Mutex
	toolWait   map[string]chan any
}

func (so *streamOrchestrator) run() {
	defer close(so.events)

	// Get provider client
	pc, err := so.client.ensureProvider(so.req.Provider)
	if err != nil {
		so.sendError(err)
		return
	}

	// Delegate to provider-specific streaming
	switch so.req.Provider {
	case ProviderOpenAI:
		err = so.streamOpenAI(pc.(*openAIProvider))
	case ProviderGoogle:
		err = so.streamGoogle(pc.(*googleProvider))
	}

	if err != nil {
		so.sendError(err)
		return
	}

	// Send completion event
	so.events <- StreamEvent{
		Type:      EventTypeDone,
		provider:  so.req.Provider,
		timestamp: time.Now(),
	}
}

func (so *streamOrchestrator) sendChunk(text string) {
	select {
	case <-so.ctx.Done():
		return
	case so.events <- StreamEvent{
		Type:      EventTypeChunk,
		Text:      text,
		provider:  so.req.Provider,
		timestamp: time.Now(),
	}:
	}
}

func (so *streamOrchestrator) sendToolCallRequest(tc *StreamToolCall) {
	select {
	case <-so.ctx.Done():
		return
	case so.events <- StreamEvent{
		Type:      EventTypeToolCallRequest,
		ToolCall:  tc,
		provider:  so.req.Provider,
		timestamp: time.Now(),
	}:
	}
}

func (so *streamOrchestrator) sendToolCallResult(tr *StreamToolResult) {
	select {
	case <-so.ctx.Done():
		return
	case so.events <- StreamEvent{
		Type:       EventTypeToolCallResult,
		ToolResult: tr,
		provider:   so.req.Provider,
		timestamp:  time.Now(),
	}:
	}
}

func (so *streamOrchestrator) sendUsage(usage *StreamUsage) {
	select {
	case <-so.ctx.Done():
		return
	case so.events <- StreamEvent{
		Type:      EventTypeUsage,
		Usage:     usage,
		provider:  so.req.Provider,
		timestamp: time.Now(),
	}:
	}
}

func (so *streamOrchestrator) sendError(err error) {
	select {
	case <-so.ctx.Done():
		return
	case so.events <- StreamEvent{
		Type:      EventTypeError,
		Err:       err,
		provider:  so.req.Provider,
		timestamp: time.Now(),
	}:
	}
}

// submitToolResult is called by user to manually submit tool results (pause mode)
func (so *streamOrchestrator) submitToolResult(toolCallID string, result any) error {
	so.toolWaitMu.Lock()
	ch, exists := so.toolWait[toolCallID]
	so.toolWaitMu.Unlock()

	if !exists {
		return fmt.Errorf("no pending tool call with ID %s", toolCallID)
	}

	select {
	case ch <- result:
		return nil
	case <-so.ctx.Done():
		return so.ctx.Err()
	}
}

// waitForToolResult blocks until tool result is submitted (pause mode)
func (so *streamOrchestrator) waitForToolResult(toolCallID string) (any, error) {
	ch := make(chan any, 1)
	so.toolWaitMu.Lock()
	so.toolWait[toolCallID] = ch
	so.toolWaitMu.Unlock()

	defer func() {
		so.toolWaitMu.Lock()
		delete(so.toolWait, toolCallID)
		so.toolWaitMu.Unlock()
	}()

	select {
	case result := <-ch:
		return result, nil
	case <-so.ctx.Done():
		return nil, so.ctx.Err()
	}
}