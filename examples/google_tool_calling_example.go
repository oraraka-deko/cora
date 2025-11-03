package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/oraraka-deko/cora/cora"
)

// Example: Multi-round tool calling with Google Gemini
// This demonstrates the fixed tool calling implementation with configurable options

func main() {
	// Configure the client with Google API and tool execution settings
	cfg := cora.CoraConfig{
		// Use environment variable: export GOOGLE_API_KEY=your_key_here
		DefaultModelGoogle: "gemini-1.5-flash",
		DetectEnv:          true, // Auto-detect API key from GOOGLE_API_KEY env var
		
		// Client-level tool configuration (applies to all tool calls)
		ToolCacheTTL:     5 * time.Minute, // Cache tool results for 5 minutes
		ToolCacheMaxSize: 100,              // Cache up to 100 results
		ToolRetryConfig: &cora.RetryConfig{
			MaxAttempts:       3,
			InitialBackoff:    100 * time.Millisecond,
			MaxBackoff:        10 * time.Second,
			BackoffMultiplier: 2.0,
		},
	}

	client := cora.New(cfg)
	ctx := context.Background()

	// Define tools using ToolBuilder for automatic schema generation
	tb := cora.NewToolBuilder()

	// Tool 1: Get weather for a location
	err := tb.AddFunc("get_weather", "Get current weather for a location", 
		func(ctx context.Context, params struct {
			Location string `json:"location" description:"City name, e.g., Boston, Tokyo"`
			Unit     string `json:"unit,omitempty" description:"Temperature unit: celsius or fahrenheit"`
		}) (any, error) {
			// Simulate API call
			fmt.Printf("🌤️  Calling get_weather(%s, %s)\n", params.Location, params.Unit)
			time.Sleep(100 * time.Millisecond) // Simulate network delay
			
			return map[string]any{
				"location":    params.Location,
				"temperature": 72,
				"unit":        params.Unit,
				"condition":   "sunny",
				"humidity":    65,
			}, nil
		})
	if err != nil {
		log.Fatal(err)
	}

	// Tool 2: Get flight information
	err = tb.AddFunc("get_flights", "Get available flights between cities",
		func(ctx context.Context, params struct {
			From string `json:"from" description:"Departure city"`
			To   string `json:"to" description:"Destination city"`
			Date string `json:"date,omitempty" description:"Travel date in YYYY-MM-DD format"`
		}) (any, error) {
			fmt.Printf("✈️  Calling get_flights(%s -> %s, %s)\n", params.From, params.To, params.Date)
			time.Sleep(150 * time.Millisecond) // Simulate network delay
			
			return map[string]any{
				"flights": []map[string]any{
					{"airline": "United", "departure": "08:00", "arrival": "12:00", "price": 250},
					{"airline": "Delta", "departure": "14:00", "arrival": "18:00", "price": 275},
				},
				"count": 2,
			}, nil
		})
	if err != nil {
		log.Fatal(err)
	}

	// Tool 3: Book a flight
	err = tb.AddFunc("book_flight", "Book a specific flight",
		func(ctx context.Context, params struct {
			Airline   string `json:"airline" description:"Airline name"`
			From      string `json:"from" description:"Departure city"`
			To        string `json:"to" description:"Destination city"`
			Departure string `json:"departure" description:"Departure time"`
		}) (any, error) {
			fmt.Printf("🎫 Calling book_flight(%s, %s->%s at %s)\n", 
				params.Airline, params.From, params.To, params.Departure)
			time.Sleep(200 * time.Millisecond) // Simulate booking delay
			
			return map[string]any{
				"confirmation": "ABC123XYZ",
				"status":       "confirmed",
				"message":      fmt.Sprintf("Flight booked on %s from %s to %s", params.Airline, params.From, params.To),
			}, nil
		})
	if err != nil {
		log.Fatal(err)
	}

	tools, handlers := tb.Build()

	// Example 1: Multi-round tool calling with default settings
	fmt.Println("=== Example 1: Multi-Round Tool Calling ===")
	fmt.Println("Query: I need to travel from Boston to San Francisco. What's the weather there and what flights are available?")
	fmt.Println()

	resp, err := client.Text(ctx, cora.TextRequest{
		Provider:     cora.ProviderGoogle,
		Model:        "gemini-1.5-flash",
		Input:        "I need to travel from Boston to San Francisco. What's the weather there and what flights are available?",
		Mode:         cora.ModeToolCalling,
		Tools:        tools,
		ToolHandlers: handlers,
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	fmt.Printf("\n✅ Final Response:\n%s\n\n", resp.Text)

	// Example 2: Parallel tool execution
	fmt.Println("=== Example 2: Parallel Tool Execution ===")
	fmt.Println("Query: What's the weather in New York, Tokyo, and London?")
	fmt.Println()

	parallelTrue := true
	resp2, err := client.Text(ctx, cora.TextRequest{
		Provider:      cora.ProviderGoogle,
		Model:         "gemini-1.5-flash",
		Input:         "What's the weather in New York, Tokyo, and London? Just give me the temperatures.",
		Mode:          cora.ModeToolCalling,
		Tools:         tools,
		ToolHandlers:  handlers,
		ParallelTools: &parallelTrue, // Enable parallel execution
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	fmt.Printf("\n✅ Final Response:\n%s\n\n", resp2.Text)

	// Example 3: Limited rounds and error handling
	fmt.Println("=== Example 3: Custom Configuration ===")
	fmt.Println("Query: I want to fly from Boston to San Francisco, check the weather, and book the earliest flight.")
	fmt.Println()

	maxRounds := 10
	stopOnError := false
	resp3, err := client.Text(ctx, cora.TextRequest{
		Provider:        cora.ProviderGoogle,
		Model:           "gemini-1.5-flash",
		Input:           "I want to fly from Boston to San Francisco, check the weather, and book the earliest flight.",
		Mode:            cora.ModeToolCalling,
		Tools:           tools,
		ToolHandlers:    handlers,
		MaxToolRounds:   &maxRounds,   // Allow up to 10 rounds
		StopOnToolError: &stopOnError, // Continue on errors
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	fmt.Printf("\n✅ Final Response:\n%s\n\n", resp3.Text)

	// Example 4: Demonstrate caching (second call should be faster)
	fmt.Println("=== Example 4: Tool Result Caching ===")
	fmt.Println("Query: What's the weather in Paris? (First call)")
	fmt.Println()

	start := time.Now()
	resp4, err := client.Text(ctx, cora.TextRequest{
		Provider:     cora.ProviderGoogle,
		Model:        "gemini-1.5-flash",
		Input:        "What's the weather in Paris?",
		Mode:         cora.ModeToolCalling,
		Tools:        tools,
		ToolHandlers: handlers,
	})
	elapsed1 := time.Since(start)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	fmt.Printf("\n✅ Response: %s\n", resp4.Text)
	fmt.Printf("⏱️  Time: %v\n\n", elapsed1)

	// Same query again - should hit cache
	fmt.Println("Query: What's the weather in Paris? (Cached call)")
	fmt.Println()

	start = time.Now()
	resp5, err := client.Text(ctx, cora.TextRequest{
		Provider:     cora.ProviderGoogle,
		Model:        "gemini-1.5-flash",
		Input:        "What's the weather in Paris?", // Same query for cache hit
		Mode:         cora.ModeToolCalling,
		Tools:        tools,
		ToolHandlers: handlers,
	})
	elapsed2 := time.Since(start)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	fmt.Printf("\n✅ Response: %s\n", resp5.Text)
	fmt.Printf("⏱️  Time: %v (should be faster due to caching)\n\n", elapsed2)

	fmt.Println("=== Summary ===")
	fmt.Println("✅ All examples completed successfully!")
	fmt.Println("Key features demonstrated:")
	fmt.Println("  • Multi-round tool calling (model can call tools multiple times)")
	fmt.Println("  • Parallel tool execution for independent calls")
	fmt.Println("  • Configurable max rounds and error handling")
	fmt.Println("  • Automatic argument validation")
	fmt.Println("  • Tool result caching for performance")
	fmt.Println("  • Automatic retry on transient errors")
}
