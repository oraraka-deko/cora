package cora

import (
	"net/http"
	"time"
)

// CoraConfig contains client-wide configuration.
// In this step we require explicit provider selection at call-time (TextRequest.Provider).
// Config holds secrets and HTTP knobs.
type CoraConfig struct {


	// Default model per provider if not set per-call.
	DefaultModelOpenAI string
	DefaultModelGoogle string

		Provider Provider

	// OpenAI configuration.
	OpenAIAPIKey     string // falls back to env OPENAI_API_KEY if empty and DetectEnv is true
	OpenAIBaseURL    string // optional; supports custom or Azure endpoint
	OpenAIOrgID      string // optional; also supports env OPENAI_ORG_ID
	OpenAIAPIType    string // "openai" (default) or "azure"
	OpenAIAPIVersion string // required for Azure

	// Google/GenAI configuration.
	GoogleAPIKey   string // falls back to env GOOGLE_API_KEY if empty and DetectEnv is true
	GoogleProject  string // required for Vertex AI
	GoogleLocation string // required for Vertex AI
	GoogleBaseURL  string // optional custom endpoint
	GoogleBackend  GoogleBackend

	// Shared client options.
	HTTPClient *http.Client
	Timeout    time.Duration // applied to HTTPOptions.Timeout (genai) and HTTP client (OpenAI) when possible

	// Auto-detection.
	DetectEnv bool // when true, pull missing values from environment
}