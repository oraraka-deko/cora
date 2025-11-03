package cora

import (
	"context"
	"testing"
	"time"
)

func TestStream_BasicChunks(t *testing.T) {
	fp := &fakeProvider{finalOut: "Hello streaming world"}
	c := &Client{cfg: CoraConfig{}}
	c.openai = fp

	ctx := context.Background()
	resp, err := c.Stream(ctx, StreamRequest{
		Provider: ProviderOpenAI,
		Model:    "gpt-test",
		Input:    "Say hello",
	})
	if err != nil {
		t.Fatalf("Stream error: %v", err)
	}
	defer resp.Cancel()

	var chunks []string
	for event := range resp.Events {
		if event.Type == EventTypeChunk {
			chunks = append(chunks, event.Text)
		}
		if event.Type == EventTypeDone {
			break
		}
	}

	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}
}

func TestStream_ToolCalls(t *testing.T) {
	// Test will be implementation-specific
	// Would mock tool call events in stream
}

func TestStream_Cancel(t *testing.T) {
	c := &Client{cfg: CoraConfig{}}
	c.openai = &fakeProvider{finalOut: "infinite"}

	ctx := context.Background()
	resp, err := c.Stream(ctx, StreamRequest{
		Provider: ProviderOpenAI,
		Model:    "gpt-test",
		Input:    "Count to infinity",
	})
	if err != nil {
		t.Fatalf("Stream error: %v", err)
	}

	// Cancel after 10ms
	go func() {
		time.Sleep(10 * time.Millisecond)
		resp.Cancel()
	}()

	eventCount := 0
	for range resp.Events {
		eventCount++
	}

	if eventCount == 0 {
		t.Error("expected some events before cancel")
	}
}