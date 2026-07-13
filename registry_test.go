package llmgateway

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stub struct {
	name string
	fail error
	out  string
}

func (s stub) Name() string { return s.name }
func (s stub) Generate(ctx context.Context, req Request) (Response, error) {
	if s.fail != nil {
		return Response{}, s.fail
	}
	return Response{Content: s.out, Usage: Usage{TotalTokens: 3}}, nil
}

func TestRegistryCascadeOn429(t *testing.T) {
	t.Parallel()
	r := NewRegistry(RegistryConfig{FallbackChain: []string{"b"}})
	r.sleepFn = func(d time.Duration) {}
	r.Register(stub{name: "a", fail: errors.New("429 rate limit")})
	r.Register(stub{name: "b", out: "from-b"})
	used, resp, err := r.Generate(context.Background(), "a", Request{User: "hi"})
	if err != nil || used != "b" || resp.Content != "from-b" {
		t.Fatalf("used=%s resp=%+v err=%v", used, resp, err)
	}
}

func TestRegistryNoFallbackOnPermanent(t *testing.T) {
	t.Parallel()
	r := NewRegistry(RegistryConfig{FallbackChain: []string{"b"}})
	r.Register(stub{name: "a", fail: errors.New("invalid api key")})
	r.Register(stub{name: "b", out: "from-b"})
	_, _, err := r.Generate(context.Background(), "a", Request{User: "hi"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUsageHook(t *testing.T) {
	t.Parallel()
	var called bool
	r := NewRegistry(RegistryConfig{UsageHook: func(ctx context.Context, provider, model string, usage Usage) {
		called = true
		if usage.TotalTokens != 3 {
			t.Fatalf("usage %+v", usage)
		}
	}})
	r.Register(stub{name: "a", out: "x"})
	_, _, err := r.Generate(context.Background(), "a", Request{Model: "m", User: "hi"})
	if err != nil || !called {
		t.Fatalf("err=%v called=%v", err, called)
	}
}

func TestCircuitBreakerTrips(t *testing.T) {
	t.Parallel()
	cb := &CircuitBreaker{}
	for i := 0; i < 3; i++ {
		cb.OnFailure(true)
	}
	if err := cb.Allow(); err == nil {
		t.Fatal("expected open")
	}
}
