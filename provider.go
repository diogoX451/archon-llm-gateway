// Package llmgateway defines a multi-provider LLM interface used by Archon.
//
// This package ships the contracts and a simple registry. Concrete vendor
// adapters (OpenAI, Anthropic, Ollama, NVIDIA, …) can live here or in
// separate modules — contributions welcome.
//
// Extracted from the Archon agent platform (https://github.com/diogoX451).
package llmgateway

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// Request is a vendor-neutral chat completion request.
type Request struct {
	Model    string
	System   string
	User     string
	// CacheKey optional provider-side prompt cache key.
	CacheKey string
	// APIKey optional per-call override (e.g. per-tenant key).
	APIKey string
}

// Usage captures token accounting when the provider returns it.
type Usage struct {
	InputTokens  int
	OutputTokens int
	CachedTokens int
	TotalTokens  int
}

// Response is a vendor-neutral completion result.
type Response struct {
	Content string
	Usage   Usage
}

// Provider generates text for one vendor.
type Provider interface {
	Name() string
	Generate(ctx context.Context, req Request) (Response, error)
}

// Registry routes Generate calls by provider name with optional fallback.
type Registry struct {
	mu               sync.RWMutex
	providers        map[string]Provider
	FallbackProvider string
	FallbackModel    string
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}

// Register adds a provider (lowercased name).
func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[strings.ToLower(p.Name())] = p
}

// Generate routes to the named provider; on failure may try FallbackProvider.
func (r *Registry) Generate(ctx context.Context, provider string, req Request) (used string, resp Response, err error) {
	r.mu.RLock()
	p := r.providers[strings.ToLower(strings.TrimSpace(provider))]
	fbName := r.FallbackProvider
	fb := r.providers[strings.ToLower(strings.TrimSpace(fbName))]
	r.mu.RUnlock()

	if p == nil {
		return "", Response{}, fmt.Errorf("unknown provider %q", provider)
	}
	resp, err = p.Generate(ctx, req)
	if err == nil {
		return p.Name(), resp, nil
	}
	if fb == nil || strings.EqualFold(fb.Name(), p.Name()) {
		return p.Name(), Response{}, err
	}
	if req.Model == "" && r.FallbackModel != "" {
		req.Model = r.FallbackModel
	}
	resp2, err2 := fb.Generate(ctx, req)
	if err2 != nil {
		return fb.Name(), Response{}, fmt.Errorf("primary %v; fallback %w", err, err2)
	}
	return fb.Name(), resp2, nil
}
