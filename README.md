# archon-llm-gateway

[![Go Reference](https://pkg.go.dev/badge/github.com/diogoX451/archon-llm-gateway.svg)](https://pkg.go.dev/github.com/diogoX451/archon-llm-gateway)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

**Production multi-provider LLM client** for agent runtimes — not another
hello-world wrapper.

## Why this exists

Agent platforms die in production on:

| Failure | What this package does |
|---------|------------------------|
| Provider 429 / 5xx | Ordered **fallback cascade** |
| Provider flapping | Per-provider **circuit breaker** |
| OpenAI URL footguns | Normalizes trailing `/v1`; Responses API |
| Reasoning models (o1/o3) | Only sends params they accept |
| Multi-tenant keys | Per-call `APIKey` override |
| Cost tracking | Pluggable **UsageHook** (no DB lock-in) |

Battle patterns extracted from the **Archon** agent platform.

## Install

```bash
go get github.com/diogoX451/archon-llm-gateway@v0.2.0
```

## Quick start

```go
reg := llmgateway.NewRegistry(llmgateway.RegistryConfig{
    FallbackChain: []string{"ollama"},
    UsageHook: func(ctx context.Context, provider, model string, u llmgateway.Usage) {
        log.Printf("llm provider=%s model=%s tokens=%d", provider, model, u.TotalTokens)
    },
})
reg.Register(llmgateway.NewOpenAIAdapter(llmgateway.OpenAIConfig{
    APIKey: os.Getenv("OPENAI_API_KEY"),
}))
reg.Register(llmgateway.NewOllamaAdapter(llmgateway.OllamaConfig{
    URL: "http://127.0.0.1:11434", BaseModel: "llama3.2", JSONFormat: true,
}))
// NVIDIA NIM / vLLM / any OpenAI-compatible host:
reg.Register(llmgateway.NewOpenAICompatAdapter(llmgateway.OpenAICompatConfig{
    Name: "nvidia",
    BaseURL: "https://integrate.api.nvidia.com/v1",
    APIKey: os.Getenv("NVIDIA_API_KEY"),
}))

used, resp, err := reg.Generate(ctx, "openai", llmgateway.Request{
    Model: "gpt-4o-mini",
    System: "You are a planner. Reply JSON only.",
    User: userJSON,
    CacheKey: blueprintHash, // OpenAI prompt cache
    APIKey: tenantKey,       // optional per-tenant override
})
```

## What to contribute (high value)

1. **Anthropic / Gemini adapters** with the same Request/Response shape  
2. **Retry-After** parsing from 429 headers into cascade backoff  
3. **Streaming** GenerateStream (SSE) without breaking non-stream API  
4. **Benchmarks** vs raw SDKs (allocs, latency overhead)  
5. **httptest integration tests** with recorded fixtures  

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Non-goals

- Embedding / rerank APIs (separate package later)
- SaaS billing or tenant key storage
- Prompt template engines

## Related

- [archon-oss](https://github.com/diogoX451/archon-oss) — toolkit index  
- [archon-need-protocol](https://github.com/diogoX451/archon-need-protocol) — agent need wire format  

## License

Apache-2.0
