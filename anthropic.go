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

// AnthropicConfig configures the Anthropic Messages API adapter.
type AnthropicConfig struct {
	APIKey         string
	BaseURL        string // default https://api.anthropic.com
	Timeout        time.Duration
	DefaultModel   string
	MaxTokens      int
	AnthropicVersion string // default 2023-06-01
}

// AnthropicAdapter targets POST /v1/messages.
type AnthropicAdapter struct {
	cfg    AnthropicConfig
	client *http.Client
}

// NewAnthropicAdapter constructs an Anthropic Messages client.
func NewAnthropicAdapter(cfg AnthropicConfig) *AnthropicAdapter {
	to := cfg.Timeout
	if to <= 0 {
		to = 2 * time.Minute
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 4096
	}
	if strings.TrimSpace(cfg.AnthropicVersion) == "" {
		cfg.AnthropicVersion = "2023-06-01"
	}
	if strings.TrimSpace(cfg.DefaultModel) == "" {
		cfg.DefaultModel = "claude-3-5-haiku-20241022"
	}
	return &AnthropicAdapter{cfg: cfg, client: &http.Client{Timeout: to}}
}

func (a *AnthropicAdapter) Name() string { return "anthropic" }

type anthropicReq struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResp struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens          int `json:"input_tokens"`
		OutputTokens         int `json:"output_tokens"`
		CacheReadInputTokens int `json:"cache_read_input_tokens"`
	} `json:"usage"`
}

func (a *AnthropicAdapter) Generate(ctx context.Context, req Request) (Response, error) {
	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(a.cfg.APIKey)
	}
	if apiKey == "" {
		return Response{}, fmt.Errorf("anthropic: API key is empty")
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = a.cfg.DefaultModel
	}
	base := strings.TrimRight(strings.TrimSpace(a.cfg.BaseURL), "/")
	if base == "" {
		base = "https://api.anthropic.com"
	}
	body := anthropicReq{
		Model:     model,
		MaxTokens: a.cfg.MaxTokens,
		System:    req.System,
		Messages:  []anthropicMessage{{Role: "user", Content: req.User}},
	}
	data, err := json.Marshal(body)
	if err != nil {
		return Response{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/messages", bytes.NewReader(data))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", a.cfg.AnthropicVersion)

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
		return Response{}, fmt.Errorf("anthropic failed: %s %s", resp.Status, string(respBody))
	}
	var parsed anthropicResp
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return Response{}, err
	}
	usage := Usage{
		InputTokens:  parsed.Usage.InputTokens,
		OutputTokens: parsed.Usage.OutputTokens,
		CachedTokens: parsed.Usage.CacheReadInputTokens,
		TotalTokens:  parsed.Usage.InputTokens + parsed.Usage.OutputTokens,
	}
	for _, part := range parsed.Content {
		if part.Type == "text" {
			if t := strings.TrimSpace(part.Text); t != "" {
				return Response{Content: t, Usage: usage}, nil
			}
		}
	}
	return Response{Usage: usage}, fmt.Errorf("anthropic empty content")
}
