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

// OllamaConfig configures a local or remote Ollama server.
type OllamaConfig struct {
	URL       string // default http://127.0.0.1:11434
	BaseModel string
	Timeout   time.Duration
	// JSONFormat requests format=json (useful for planner-style agents).
	JSONFormat bool
}

// OllamaAdapter targets POST /api/chat.
type OllamaAdapter struct {
	cfg    OllamaConfig
	client *http.Client
}

func NewOllamaAdapter(cfg OllamaConfig) *OllamaAdapter {
	to := cfg.Timeout
	if to <= 0 {
		to = 2 * time.Minute
	}
	return &OllamaAdapter{cfg: cfg, client: &http.Client{Timeout: to}}
}

func (a *OllamaAdapter) Name() string { return "ollama" }

func (a *OllamaAdapter) Generate(ctx context.Context, req Request) (Response, error) {
	base := strings.TrimRight(a.cfg.URL, "/")
	if base == "" {
		base = "http://127.0.0.1:11434"
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = a.cfg.BaseModel
	}
	if model == "" {
		return Response{}, fmt.Errorf("ollama: model is required")
	}
	body := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": req.System},
			{"role": "user", "content": req.User},
		},
		"stream": false,
	}
	if a.cfg.JSONFormat {
		body["format"] = "json"
	}
	data, err := json.Marshal(body)
	if err != nil {
		return Response{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/chat", bytes.NewReader(data))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
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
		return Response{}, fmt.Errorf("ollama failed: %s %s", resp.Status, string(respBody))
	}
	var parsed struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		PromptEvalCount int `json:"prompt_eval_count"`
		EvalCount       int `json:"eval_count"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return Response{}, err
	}
	return Response{
		Content: strings.TrimSpace(parsed.Message.Content),
		Usage: Usage{
			InputTokens:  parsed.PromptEvalCount,
			OutputTokens: parsed.EvalCount,
			TotalTokens:  parsed.PromptEvalCount + parsed.EvalCount,
		},
	}, nil
}
