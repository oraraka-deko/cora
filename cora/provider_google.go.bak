package cora

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

type googleProvider struct {
	client *genai.Client
}

func newGoogleProvider(cfg CoraConfig) (providerClient, error) {
	if cfg.GoogleAPIKey == "" {
		return nil, errors.New("cora: Google API key is required to use ProviderGoogle")
	}
	gc, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey: cfg.GoogleAPIKey,
		HTTPOptions: genai.HTTPOptions{
			BaseURL: cfg.GoogleBaseURL,
		},
		// Backend: default Gemini Developer API for this step.
	})
	if err != nil {
		return nil, err
	}
	return &googleProvider{client: gc}, nil
}

func (p *googleProvider) Text(ctx context.Context, plan callPlan) (callResult, error) {
	if plan.Proofread {
		return p.proofread(ctx, plan)
	}

	contents := genai.Text(plan.Input)
	cfg := &genai.GenerateContentConfig{}
	if strings.TrimSpace(plan.System) != "" {
		cfg.SystemInstruction = &genai.Content{
			Parts: []*genai.Part{{Text: plan.System}},
		}
	}
	if plan.Temperature != nil {
		cfg.Temperature = genai.Ptr[float32](*plan.Temperature)
	}
	if plan.MaxOutputTokens != nil {
		cfg.MaxOutputTokens = int32(*plan.MaxOutputTokens)
	}
	if len(plan.Labels) > 0 {
		cfg.Labels = plan.Labels
	}

	// Structured JSON
	if plan.Structured && len(plan.ResponseSchema) > 0 {
		cfg.ResponseMIMEType = "application/json"
		cfg.ResponseJsonSchema = plan.ResponseSchema
	}

	// Tool calling
	if len(plan.Tools) > 0 {
		cfg.Tools = toGenAITools(plan.Tools)
		cfg.ToolConfig = &genai.ToolConfig{
			FunctionCallingConfig: &genai.FunctionCallingConfig{
				Mode: genai.FunctionCallingConfigModeAny,
			},
		}
	}

	res, err := p.client.Models.GenerateContent(ctx, plan.Model, contents, cfg)
	if err != nil {
		return callResult{}, err
	}
	cr := toCallResultFromGenAI(res)

	// Handle a single round of tool calls if requested and handlers provided.
	if len(plan.Tools) > 0 && len(plan.ToolHandlers) > 0 {
		fcs := res.FunctionCalls()
		if len(fcs) > 0 {
			// Execute each function call and send back as FunctionResponse parts.
			respContent := &genai.Content{Role: "user"} // role is not strictly enforced here
			for _, fc := range fcs {
				h, ok := plan.ToolHandlers[fc.Name]
				if !ok {
					return callResult{}, fmt.Errorf("cora: no handler for tool %q", fc.Name)
				}
				result, err := h(ctx, fc.Args) // Args is already a map[string]any in genai
				if err != nil {
					return callResult{}, fmt.Errorf("cora: tool %q failed: %w", fc.Name, err)
				}
				// Ensure JSON-serializable map
				payload, err := normalizeJSON(result)
				if err != nil {
					return callResult{}, fmt.Errorf("cora: tool %q result not JSON-serializable: %w", fc.Name, err)
				}
				respContent.Parts = append(respContent.Parts, &genai.Part{
					FunctionResponse: &genai.FunctionResponse{
						Name:     fc.Name,
						Response: payload,
					},
				})
			}
			// Follow-up call
			res2, err := p.client.Models.GenerateContent(ctx, plan.Model, []*genai.Content{respContent}, &genai.GenerateContentConfig{})
			if err != nil {
				return callResult{}, err
			}
			cr = toCallResultFromGenAI(res2)
			cr.toolLoop = true
		}
	}

	return cr, nil
}

func (p *googleProvider) proofread(ctx context.Context, plan callPlan) (callResult, error) {
	sys := &genai.Content{Parts: []*genai.Part{{
		Text: "You are a writing assistant. Rewrite the user's input to correct grammar, spelling, and clarity without changing its meaning. Return only the rewritten text.",
	}}}
	cfg := &genai.GenerateContentConfig{
		SystemInstruction: sys,
		Temperature:       genai.Ptr[float32](0.2),
	}
	if plan.Temperature != nil {
		cfg.Temperature = genai.Ptr[float32](*plan.Temperature)
	}
	if plan.MaxOutputTokens != nil {
		cfg.MaxOutputTokens = int32(*plan.MaxOutputTokens)
	}
	res, err := p.client.Models.GenerateContent(ctx, plan.Model, genai.Text(plan.Input), cfg)
	if err != nil {
		return callResult{}, err
	}
	return toCallResultFromGenAI(res), nil
}

func toGenAITools(tools []CoraTool) []*genai.Tool {
	out := make([]*genai.Tool, 0, len(tools))
	for _, t := range tools {
		out = append(out, &genai.Tool{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				{
					Name:                 t.Name,
					Description:          t.Description,
					ParametersJsonSchema: t.ParametersSchema, // provider accepts raw schema object
				},
			},
		})
	}
	return out
}

func toCallResultFromGenAI(res *genai.GenerateContentResponse) callResult {
	cr := callResult{}
	if res == nil || len(res.Candidates) == 0 || res.Candidates[0].Content == nil {
		return cr
	}
	parts := res.Candidates[0].Content.Parts
	for _, p := range parts {
		if p.Text != "" {
			// If multiple text parts, concatenate with a newline.
			if cr.Text == "" {
				cr.Text = p.Text
			} else {
				cr.Text += "\n" + p.Text
			}
		}
	}
	// Attempt to parse text as JSON for structured responses.
	if cr.Text != "" {
		var m map[string]any
		if json.Unmarshal([]byte(cr.Text), &m) == nil {
			cr.JSON = m
		}
	}

	if res.UsageMetadata != nil {
		if res.UsageMetadata.PromptTokenCount > 0 {
			pt := int(res.UsageMetadata.PromptTokenCount)
			cr.PromptTokens = &pt
		}
		if res.UsageMetadata.CandidatesTokenCount > 0 {
			ct := int(res.UsageMetadata.CandidatesTokenCount)
			cr.CompletionTokens = &ct
		}
		if res.UsageMetadata.TotalTokenCount > 0 {
			tt := int(res.UsageMetadata.TotalTokenCount)
			cr.TotalTokens = &tt
		}
	}
	return cr
}

func normalizeJSON(v any) (map[string]any, error) {
	switch t := v.(type) {
	case map[string]any:
		return t, nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			return nil, err
		}
		return m, nil
	}
}
