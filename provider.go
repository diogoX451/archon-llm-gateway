// Package llmgateway is a production multi-provider LLM client for agent runtimes.
//
// Features that matter in real systems (not just demos):
//   - Registry with ordered fallback cascade on 429/5xx
//   - Per-provider circuit breaker
//   - OpenAI Responses API with prompt cache key + reasoning-model quirks
//   - Ollama /api/chat and OpenAI-compatible endpoints (NVIDIA NIM, etc.)
//   - Optional usage hooks for cost accounting (no DB coupling)
//
// Part of the Archon open-source toolkit: https://github.com/diogoX451/archon-oss
package llmgateway

import "context"

// Request is a vendor-neutral completion request for agent planners/tools.
type Request struct {
	Model    string
	System   string
	User     string
	// CacheKey is used by providers that support prompt caching (OpenAI).
	CacheKey string
	// APIKey overrides the adapter default for this call (per-tenant keys).
	APIKey string
}

// Usage is token accounting when the provider exposes it.
type Usage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
	CachedTokens int
}

// Response is a vendor-neutral completion result.
type Response struct {
	Content string
	Usage   Usage
}

// Provider is one vendor adapter.
type Provider interface {
	Name() string
	Generate(ctx context.Context, req Request) (Response, error)
}

// UsageHook is called after a successful Generate (best-effort; must not panic).
type UsageHook func(ctx context.Context, provider, model string, usage Usage)
