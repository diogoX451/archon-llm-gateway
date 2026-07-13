package llmgateway

import (
	"context"
	"errors"
	"testing"
)

type stub struct {
	name string
	fail bool
}

func (s stub) Name() string { return s.name }
func (s stub) Generate(ctx context.Context, req Request) (Response, error) {
	if s.fail {
		return Response{}, errors.New("fail")
	}
	return Response{Content: "ok:" + s.name}, nil
}

func TestRegistryFallback(t *testing.T) {
	r := NewRegistry()
	r.Register(stub{name: "a", fail: true})
	r.Register(stub{name: "b", fail: false})
	r.FallbackProvider = "b"
	used, resp, err := r.Generate(context.Background(), "a", Request{User: "hi"})
	if err != nil || used != "b" || resp.Content != "ok:b" {
		t.Fatalf("used=%s resp=%+v err=%v", used, resp, err)
	}
}
