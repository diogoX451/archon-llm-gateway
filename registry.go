package llmgateway

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Registry routes Generate calls with cascade fallback and circuit breaking.
type Registry struct {
	providers     map[string]Provider
	fallbackChain []string
	fallbackModel string
	sleepFn       func(time.Duration)
	breakers      map[string]*CircuitBreaker
	usageHook     UsageHook
}

// RegistryConfig configures fallback behaviour.
type RegistryConfig struct {
	// FallbackChain is tried in order after the primary fails with a retryable error.
	FallbackChain []string
	// FallbackModel applied when cascading if the request Model is empty.
	FallbackModel string
	// UsageHook optional cost/metrics callback.
	UsageHook UsageHook
}

// NewRegistry builds a registry.
func NewRegistry(cfg RegistryConfig) *Registry {
	chain := make([]string, 0, len(cfg.FallbackChain))
	for _, p := range cfg.FallbackChain {
		if v := strings.ToLower(strings.TrimSpace(p)); v != "" {
			chain = append(chain, v)
		}
	}
	return &Registry{
		providers:     make(map[string]Provider),
		fallbackChain: chain,
		fallbackModel: strings.TrimSpace(cfg.FallbackModel),
		sleepFn:       time.Sleep,
		breakers:      make(map[string]*CircuitBreaker),
		usageHook:     cfg.UsageHook,
	}
}

// Register adds or replaces a provider.
func (r *Registry) Register(p Provider) {
	key := strings.ToLower(strings.TrimSpace(p.Name()))
	r.providers[key] = p
	r.breakers[key] = &CircuitBreaker{}
}

// Generate dispatches to providerName and cascades on retryable failures.
// Returns the provider name actually used.
func (r *Registry) Generate(ctx context.Context, providerName string, req Request) (string, Response, error) {
	name := strings.ToLower(strings.TrimSpace(providerName))
	p, ok := r.providers[name]
	if !ok {
		return "", Response{}, fmt.Errorf("unknown provider %q", providerName)
	}
	if cb := r.breakers[name]; cb != nil {
		if err := cb.Allow(); err != nil {
			return r.tryFallback(ctx, req, name, err)
		}
	}
	resp, err := p.Generate(ctx, req)
	if err == nil {
		if cb := r.breakers[name]; cb != nil {
			cb.OnSuccess()
		}
		r.fireUsage(ctx, p.Name(), req.Model, resp.Usage)
		return p.Name(), resp, nil
	}
	if cb := r.breakers[name]; cb != nil {
		cb.OnFailure(is5xx(err))
	}
	if !isRetryable(err) {
		return p.Name(), Response{}, err
	}
	return r.tryFallback(ctx, req, name, err)
}

func (r *Registry) tryFallback(ctx context.Context, req Request, primary string, primaryErr error) (string, Response, error) {
	var last error = primaryErr
	for i, fb := range r.fallbackChain {
		if fb == primary {
			continue
		}
		p, ok := r.providers[fb]
		if !ok {
			continue
		}
		if cb := r.breakers[fb]; cb != nil {
			if err := cb.Allow(); err != nil {
				last = err
				continue
			}
		}
		// brief backoff between attempts
		if i > 0 && r.sleepFn != nil {
			r.sleepFn(time.Duration(100*(i+1)) * time.Millisecond)
		}
		rreq := req
		if strings.TrimSpace(rreq.Model) == "" && r.fallbackModel != "" {
			rreq.Model = r.fallbackModel
		}
		resp, err := p.Generate(ctx, rreq)
		if err == nil {
			if cb := r.breakers[fb]; cb != nil {
				cb.OnSuccess()
			}
			r.fireUsage(ctx, p.Name(), rreq.Model, resp.Usage)
			return p.Name(), resp, nil
		}
		last = err
		if cb := r.breakers[fb]; cb != nil {
			cb.OnFailure(is5xx(err))
		}
	}
	return primary, Response{}, fmt.Errorf("all providers failed: %w", last)
}

func (r *Registry) fireUsage(ctx context.Context, provider, model string, usage Usage) {
	if r.usageHook == nil {
		return
	}
	defer func() { _ = recover() }()
	r.usageHook(ctx, provider, model, usage)
}
