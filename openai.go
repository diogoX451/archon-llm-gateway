package llmgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAIConfig configures the OpenAI Responses API adapter.
type OpenAIConfig struct {
	APIKey  string
	BaseURL string // default https://api.openai.com
	Timeout time.Duration
}

// OpenAIAdapter targets POST /v1/responses (prompt cache + reasoning models).
type OpenAIAdapter struct {
	cfg    OpenAIConfig
	client *http.Client
}

// NewOpenAIAdapter constructs an OpenAI Responses API client.
func NewOpenAIAdapter(cfg OpenAIConfig) *OpenAIAdapter {
	to := cfg.Timeout
	if to <= 0 {
		to = 2 * time.Minute
	}
	return &OpenAIAdapter{cfg: cfg, client: &http.Client{Timeout: to}}
}

func (a *OpenAIAdapter) Name() string { return "openai" }

func isReasoningModel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(m, "o1") || strings.HasPrefix(m, "o3") || strings.HasPrefix(m, "o4")
}

func normalizeOpenAIBaseURL(raw string) string {
	base := strings.TrimRight(strings.TrimSpace(raw), "/")
	if base == "" {
		return "https://api.openai.com"
	}
	base = strings.TrimSuffix(base, "/v1")
	if strings.TrimSpace(base) == "" {
		return "https://api.openai.com"
	}
	return base
}

type openAIReq struct {
	Model          string              `json:"model"`
	Input          []openAIInput       `json:"input"`
	Reasoning      *openAIReasoning    `json:"reasoning,omitempty"`
	Text           *openAIText         `json:"text,omitempty"`
	PromptCacheKey string              `json:"prompt_cache_key,omitempty"`
}
type openAIInput struct {
	Role    string          `json:"role"`
	Content []openAIContent `json:"content"`
}
type openAIContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
type openAIReasoning struct {
	Effort string `json:"effort,omitempty"`
}
type openAIText struct {
	Verbosity string `json:"verbosity,omitempty"`
}
type openAIResp struct {
	OutputText string `json:"output_text"`
	Output     []struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
	Usage *struct {
		InputTokens        int `json:"input_tokens"`
		OutputTokens       int `json:"output_tokens"`
		TotalTokens        int `json:"total_tokens"`
		InputTokensDetails *struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"input_tokens_details"`
	} `json:"usage"`
}

func (a *OpenAIAdapter) Generate(ctx context.Context, req Request) (Response, error) {
	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(a.cfg.APIKey)
	}
	if apiKey == "" {
		return Response{}, fmt.Errorf("openai: API key is empty")
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		return Response{}, fmt.Errorf("openai: model is required")
	}
	body := openAIReq{
		Model: model,
		Input: []openAIInput{
			{Role: "system", Content: []openAIContent{{Type: "input_text", Text: req.System}}},
			{Role: "user", Content: []openAIContent{{Type: "input_text", Text: req.User}}},
		},
		PromptCacheKey: req.CacheKey,
	}
	if isReasoningModel(model) {
		body.Reasoning = &openAIReasoning{Effort: "low"}
		body.Text = &openAIText{Verbosity: "low"}
	}
	data, err := json.Marshal(body)
	if err != nil {
		return Response{}, err
	}
	url := normalizeOpenAIBaseURL(a.cfg.BaseURL) + "/v1/responses"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, err
	}
	if resp.StatusCode >= 300 {
		return Response{}, fmt.Errorf("openai failed: %s %s", resp.Status, string(respBody))
	}
	var parsed openAIResp
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return Response{}, err
	}
	usage := Usage{}
	if parsed.Usage != nil {
		usage.InputTokens = parsed.Usage.InputTokens
		usage.OutputTokens = parsed.Usage.OutputTokens
		usage.TotalTokens = parsed.Usage.TotalTokens
		if parsed.Usage.InputTokensDetails != nil {
			usage.CachedTokens = parsed.Usage.InputTokensDetails.CachedTokens
		}
	}
	if t := strings.TrimSpace(parsed.OutputText); t != "" {
		return Response{Content: t, Usage: usage}, nil
	}
	for _, out := range parsed.Output {
		for _, part := range out.Content {
			if t := strings.TrimSpace(part.Text); t != "" {
				return Response{Content: t, Usage: usage}, nil
			}
		}
	}
	return Response{Usage: usage}, fmt.Errorf("openai empty output_text")
}
