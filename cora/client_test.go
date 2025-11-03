package cora

import (
	"os"
	"testing"
)

func TestNew_OpenAIOnly_FromEnv(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test")
	// Ensure GOOGLE_API_KEY is unset to avoid accidental Google init.
	_ = os.Unsetenv("GOOGLE_API_KEY")

	c := New(CoraConfig{DetectEnv: true})
	if c == nil {
		t.Fatalf("New returned nil client")
	}
	if c.cfg.OpenAIAPIKey != "sk-test" {
		t.Fatalf("expected OpenAI key to be loaded from env, got %q", c.cfg.OpenAIAPIKey)
	}
	if c.cfg.GoogleAPIKey != "" {
		t.Fatalf("expected Google key to be empty, got %q", c.cfg.GoogleAPIKey)
	}
}

func TestNew_GoogleGeminiOnly_FromEnv(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "gsk-test")
	_ = os.Unsetenv("OPENAI_API_KEY")
	_ = os.Unsetenv("GOOGLE_CLOUD_PROJECT")
	_ = os.Unsetenv("GOOGLE_CLOUD_LOCATION")

	c := New(CoraConfig{DetectEnv: true, GoogleBackend: GoogleBackendGemini})
	if c == nil {
		t.Fatalf("New returned nil client")
	}
	if c.cfg.GoogleAPIKey != "gsk-test" {
		t.Fatalf("expected Google key to be loaded from env, got %q", c.cfg.GoogleAPIKey)
	}
	if c.cfg.OpenAIAPIKey != "" {
		t.Fatalf("expected OpenAI key to be empty, got %q", c.cfg.OpenAIAPIKey)
	}
}

func TestNew_GoogleVertex_FromExplicit(t *testing.T) {
	// Simulate Vertex AI usage without GOOGLE_API_KEY (ADC or other credentials may apply).
	_ = os.Unsetenv("GOOGLE_API_KEY")
	_ = os.Unsetenv("OPENAI_API_KEY")

	cfg := CoraConfig{
		Provider:       ProviderGoogle,
		GoogleBackend:  GoogleBackendVertex,
		GoogleProject:  "my-project",
		GoogleLocation: "us-central1",
		// API key is optional for Vertex if user uses ADC, we only check we can construct client.
	}
	c := New(cfg)
	if c == nil {
		t.Fatalf("New returned nil client")
	}
	// Just verify client is created; actual provider init happens on first Text() call
	if c.cfg.Provider != ProviderGoogle {
		t.Fatalf("expected ProviderGoogle, got %v", c.cfg.Provider)
	}
}

func TestNew_BothProviders_FromExplicit(t *testing.T) {
	cfg := CoraConfig{
		DetectEnv:     false,
		OpenAIAPIKey:  "sk-openai",
		GoogleAPIKey:  "gsk-google",
		GoogleBackend: GoogleBackendGemini,
	}
	c := New(cfg)
	if c == nil {
		t.Fatalf("New returned nil client")
	}
	if c.cfg.OpenAIAPIKey != "sk-openai" {
		t.Fatalf("expected OpenAI key to be set")
	}
	if c.cfg.GoogleAPIKey != "gsk-google" {
		t.Fatalf("expected Google key to be set")
	}
}

func TestNew_RequiresValidConfig(t *testing.T) {
	_ = os.Unsetenv("OPENAI_API_KEY")
	_ = os.Unsetenv("GOOGLE_API_KEY")
	c := New(CoraConfig{})
	if c == nil {
		t.Fatalf("New returned nil client even with empty config")
	}
}
