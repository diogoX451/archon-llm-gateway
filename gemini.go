package llmgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GeminiConfig configures the Google Generative Language API adapter.
type GeminiConfig struct {
	APIKey       string
	BaseURL      string // default https://generativelanguage.googleapis.com
	Timeout      time.Duration
	DefaultModel string
	// JSONMode sets generation_config.response_mime_type to application/json.
	JSONMode bool
}

// GeminiAdapter targets POST /v1beta/models/{model}:generateContent.
type GeminiAdapter struct {
	cfg    GeminiConfig
	client *http.Client
}

// NewGeminiAdapter constructs a Gemini generateContent client.
func NewGeminiAdapter(cfg GeminiConfig) *GeminiAdapter {
	to := cfg.Timeout
	if to <= 0 {
		to = 2 * time.Minute
	}
	if strings.TrimSpace(cfg.DefaultModel) == "" {
		cfg.DefaultModel = "gemini-2.0-flash"
	}
	return &GeminiAdapter{cfg: cfg, client: &http.Client{Timeout: to}}
}

func (a *GeminiAdapter) Name() string { return "gemini" }

type geminiReq struct {
	SystemInstruction *geminiContent  `json:"system_instruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
	GenerationConfig  *geminiGenCfg   `json:"generation_config,omitempty"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenCfg struct {
	ResponseMIMEType string `json:"response_mime_type,omitempty"`
}

type geminiResp struct {
	Candidates []struct {
		Content geminiContent `json:"content"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount        int `json:"promptTokenCount"`
		CandidatesTokenCount    int `json:"candidatesTokenCount"`
		CachedContentTokenCount int `json:"cachedContentTokenCount"`
		TotalTokenCount         int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

func (a *GeminiAdapter) Generate(ctx context.Context, req Request) (Response, error) {
	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(a.cfg.APIKey)
	}
	if apiKey == "" {
		return Response{}, fmt.Errorf("gemini: API key is empty")
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = a.cfg.DefaultModel
	}
	base := strings.TrimRight(strings.TrimSpace(a.cfg.BaseURL), "/")
	if base == "" {
		base = "https://generativelanguage.googleapis.com"
	}
	endpoint := base + "/v1beta/models/" + url.PathEscape(model) + ":generateContent"
	body := geminiReq{
		SystemInstruction: &geminiContent{Parts: []geminiPart{{Text: req.System}}},
		Contents:          []geminiContent{{Parts: []geminiPart{{Text: req.User}}}},
	}
	if a.cfg.JSONMode {
		body.GenerationConfig = &geminiGenCfg{ResponseMIMEType: "application/json"}
	}
	data, err := json.Marshal(body)
	if err != nil {
		return Response{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", apiKey)

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
		return Response{}, fmt.Errorf("gemini failed: %s %s", resp.Status, string(respBody))
	}
	var parsed geminiResp
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return Response{}, err
	}
	usage := Usage{
		InputTokens:  parsed.UsageMetadata.PromptTokenCount,
		OutputTokens: parsed.UsageMetadata.CandidatesTokenCount,
		CachedTokens: parsed.UsageMetadata.CachedContentTokenCount,
		TotalTokens:  parsed.UsageMetadata.TotalTokenCount,
	}
	for _, cand := range parsed.Candidates {
		for _, part := range cand.Content.Parts {
			if t := strings.TrimSpace(part.Text); t != "" {
				return Response{Content: t, Usage: usage}, nil
			}
		}
	}
	return Response{Usage: usage}, fmt.Errorf("gemini empty content")
}
