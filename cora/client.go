package cora

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// GoogleBackend selects the underlying Google backend.
type GoogleBackend int

const (
	// GoogleBackendAuto chooses based on presence of Project/Location (Vertex) or not (Gemini API).
	GoogleBackendAuto GoogleBackend = iota
	// GoogleBackendGemini uses Gemini Developer API.
	GoogleBackendGemini
	// GoogleBackendVertex uses Vertex AI (requires Project and Location).
	GoogleBackendVertex
)

// Client is the unified, minimal public client.
type Client struct {
	cfg    CoraConfig
	openai providerClient // lazily init
	google providerClient // lazily init

}

// New creates a Client with the given config.
// If DetectEnv is true, it pulls missing API keys from environment variables.
func New(cfg CoraConfig) *Client {
	if cfg.DetectEnv {
		if cfg.OpenAIAPIKey == "" {
			cfg.OpenAIAPIKey = os.Getenv("OPENAI_API_KEY")
		}
		if cfg.GoogleAPIKey == "" {
			cfg.GoogleAPIKey = os.Getenv("GOOGLE_API_KEY")
		}
	}
	return &Client{cfg: cfg}
}

// Text executes a text request using the requested provider/model and the selected Mode orchestration.
func (c *Client) Text(ctx context.Context, req TextRequest) (TextResponse, error) {
	if req.Provider != ProviderOpenAI && req.Provider != ProviderGoogle {
		return TextResponse{}, fmt.Errorf("cora: unknown provider %q", req.Provider)
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
			return TextResponse{}, errors.New("cora: model must be specified")
		}
	}

	// 1) Build call plans based on Mode.
	plans, err := buildPlans(req.Provider, model, req, c.cfg)
	if err != nil {
		return TextResponse{}, err
	}

	// 2) Execute plans sequentially; later plans may depend on earlier outputs.
	var finalRes callResult
	for i, p := range plans {
		pc, err := c.ensureProvider(p.Provider)
		if err != nil {
			return TextResponse{}, err
		}
		res, err := pc.Text(ctx, p)
		if err != nil {
			return TextResponse{}, err
		}
		finalRes = res

		// Two-step chaining: feed previous improved text into next plan.
		if i+1 < len(plans) {
			plans[i+1].Input = resultPreferredInput(res)
		}
	}

	out := TextResponse{
		Provider: req.Provider,
		Model:    model,
		Mode:     req.Mode,
		Text:     finalRes.Text,
		JSON:     finalRes.JSON,
	}
	out.PromptTokens = finalRes.PromptTokens
	out.CompletionTokens = finalRes.CompletionTokens
	out.TotalTokens = finalRes.TotalTokens
	return out, nil
}

func (c *Client) ensureProvider(p Provider) (providerClient, error) {
	switch p {
	case ProviderOpenAI:
		if c.openai == nil {
			pc, err := newOpenAIProvider(c.cfg)
			if err != nil {
				return nil, err
			}
			c.openai = pc
		}
		return c.openai, nil
	case ProviderGoogle:
		if c.google == nil {
			pc, err := newGoogleProvider(c.cfg)
			if err != nil {
				return nil, err
			}
			c.google = pc
		}
		return c.google, nil
	default:
		return nil, fmt.Errorf("cora: unsupported provider %q", p)
	}
}

// buildPlans converts a TextRequest + Mode into one or more call plans.
func buildPlans(provider Provider, model string, req TextRequest, cfg CoraConfig) ([]callPlan, error) {
	base := callPlan{
		Provider:         provider,
		Model:            model,
		System:           req.System,
		Input:            req.Input,
		Temperature:      req.Temperature,
		MaxOutputTokens:  req.MaxOutputTokens,
		Labels:           req.Labels,
		ToolCacheTTL:     cfg.ToolCacheTTL,
		ToolCacheMaxSize: cfg.ToolCacheMaxSize,
		ToolRetryConfig:  cfg.ToolRetryConfig,
	}

	switch req.Mode {
	case ModeBasic:
		return []callPlan{base}, nil

	case ModeStructuredJSON:
		if len(req.ResponseSchema) == 0 {
			return nil, errors.New("cora: ResponseSchema is required for ModeStructuredJSON")
		}
		base.Structured = true
		base.ResponseSchema = req.ResponseSchema
		return []callPlan{base}, nil

	case ModeToolCalling:
		if len(req.Tools) == 0 {
			return nil, errors.New("cora: Tools must be provided for ModeToolCalling")
		}
		base.Tools = req.Tools
		base.ToolHandlers = req.ToolHandlers
		base.MaxToolRounds = req.MaxToolRounds
		base.ParallelTools = req.ParallelTools
		base.StopOnToolError = req.StopOnToolError
		return []callPlan{base}, nil

	case ModeTwoStepEnhance:
		// Plan 1: proofreading step
		p1 := base
		p1.Proofread = true
		p1.System = "" // system for the clean-up is internally applied

		// Plan 2: final answer on improved text (inherits original options)
		p2 := base
		return []callPlan{p1, p2}, nil

	default:
		return nil, fmt.Errorf("cora: unknown mode %v", req.Mode)
	}
}

// resultPreferredInput picks the best string to feed into the next step.
func resultPreferredInput(res callResult) string {
	if res.Text != "" {
		return res.Text
	}
	// If structured result, feed JSON string.
	if len(res.JSON) > 0 {
		b := jsonMarshalNoErr(res.JSON)
		return string(b)
	}
	return ""
}

func jsonMarshalNoErr(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
