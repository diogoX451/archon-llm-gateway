package llmgateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAnthropicAdapter_ParsesUsage(t *testing.T) {
	t.Parallel()
	fixture := `{
		"content": [{"type": "text", "text": "hello"}],
		"usage": {
			"input_tokens": 20,
			"output_tokens": 10,
			"cache_read_input_tokens": 5
		}
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path=%s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("missing api key header")
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("missing anthropic-version")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixture))
	}))
	defer srv.Close()

	a := NewAnthropicAdapter(AnthropicConfig{APIKey: "test-key", BaseURL: srv.URL})
	resp, err := a.Generate(context.Background(), Request{
		Model: "claude-3-5-haiku-20241022", System: "sys", User: "user",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "hello" {
		t.Fatalf("content=%q", resp.Content)
	}
	if resp.Usage.InputTokens != 20 || resp.Usage.OutputTokens != 10 || resp.Usage.CachedTokens != 5 {
		t.Fatalf("usage=%+v", resp.Usage)
	}
	if resp.Usage.TotalTokens != 30 {
		t.Fatalf("total=%d", resp.Usage.TotalTokens)
	}
}

func TestAnthropicAdapter_EmptyKey(t *testing.T) {
	t.Parallel()
	a := NewAnthropicAdapter(AnthropicConfig{})
	_, err := a.Generate(context.Background(), Request{User: "hi"})
	if err == nil {
		t.Fatal("expected error")
	}
}
