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

// OpenAICompatConfig targets any OpenAI Chat Completions compatible API
// (NVIDIA NIM, vLLM, LocalAI, Groq, …).
type OpenAICompatConfig struct {
	Name      string // provider name reported by Name(), e.g. "nvidia"
	APIKey    string
	BaseURL   string // e.g. https://integrate.api.nvidia.com/v1
	Timeout   time.Duration
	DefaultModel string
}

// OpenAICompatAdapter implements chat/completions for OpenAI-compatible hosts.
type OpenAICompatAdapter struct {
	cfg    OpenAICompatConfig
	client *http.Client
}

func NewOpenAICompatAdapter(cfg OpenAICompatConfig) *OpenAICompatAdapter {
	to := cfg.Timeout
	if to <= 0 {
		to = 2 * time.Minute
	}
	if strings.TrimSpace(cfg.Name) == "" {
		cfg.Name = "openai-compat"
	}
	return &OpenAICompatAdapter{cfg: cfg, client: &http.Client{Timeout: to}}
}

func (a *OpenAICompatAdapter) Name() string { return a.cfg.Name }

func (a *OpenAICompatAdapter) Generate(ctx context.Context, req Request) (Response, error) {
	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(a.cfg.APIKey)
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = a.cfg.DefaultModel
	}
	if model == "" {
		return Response{}, fmt.Errorf("%s: model is required", a.cfg.Name)
	}
	base := strings.TrimRight(a.cfg.BaseURL, "/")
	if base == "" {
		return Response{}, fmt.Errorf("%s: BaseURL is required", a.cfg.Name)
	}
	// Accept both root and /v1
	if !strings.HasSuffix(base, "/v1") {
		base = base + "/v1"
	}
	body := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": req.System},
			{"role": "user", "content": req.User},
		},
	}
	data, err := json.Marshal(body)
	if err != nil {
		return Response{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}
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
		return Response{}, fmt.Errorf("%s failed: %s %s", a.cfg.Name, resp.Status, string(respBody))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return Response{}, err
	}
	if len(parsed.Choices) == 0 {
		return Response{}, fmt.Errorf("%s empty choices", a.cfg.Name)
	}
	usage := Usage{}
	if parsed.Usage != nil {
		usage = Usage{
			InputTokens:  parsed.Usage.PromptTokens,
			OutputTokens: parsed.Usage.CompletionTokens,
			TotalTokens:  parsed.Usage.TotalTokens,
		}
	}
	return Response{Content: strings.TrimSpace(parsed.Choices[0].Message.Content), Usage: usage}, nil
}
