package llmgateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGeminiAdapter_ParsesUsageMetadata(t *testing.T) {
	t.Parallel()
	fixture := `{
		"candidates": [{"content": {"parts": [{"text": "hello"}]}}],
		"usageMetadata": {
			"promptTokenCount": 10,
			"candidatesTokenCount": 5,
			"cachedContentTokenCount": 0,
			"totalTokenCount": 15
		}
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, ":generateContent") {
			t.Errorf("path=%s", r.URL.Path)
		}
		if r.Header.Get("x-goog-api-key") != "test-key" {
			t.Error("missing api key")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixture))
	}))
	defer srv.Close()

	a := NewGeminiAdapter(GeminiConfig{APIKey: "test-key", BaseURL: srv.URL})
	resp, err := a.Generate(context.Background(), Request{
		Model: "gemini-2.0-flash", System: "sys", User: "user",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "hello" {
		t.Fatalf("content=%q", resp.Content)
	}
	if resp.Usage.InputTokens != 10 || resp.Usage.OutputTokens != 5 || resp.Usage.TotalTokens != 15 {
		t.Fatalf("usage=%+v", resp.Usage)
	}
}

func TestGeminiAdapter_EmptyKey(t *testing.T) {
	t.Parallel()
	a := NewGeminiAdapter(GeminiConfig{})
	_, err := a.Generate(context.Background(), Request{User: "hi"})
	if err == nil {
		t.Fatal("expected error")
	}
}
