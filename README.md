# archon-llm-gateway

[![Go Reference](https://pkg.go.dev/badge/github.com/diogoX451/archon-llm-gateway.svg)](https://pkg.go.dev/github.com/diogoX451/archon-llm-gateway)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

Multi-provider **LLM registry** contracts (generate + fallback).

Part of the **Archon open-source toolkit** by [@diogoX451](https://github.com/diogoX451).

> **Status:** v0 contracts. Vendor adapters (OpenAI, Anthropic, Ollama, NVIDIA)
> are welcome contributions — see [CONTRIBUTING.md](CONTRIBUTING.md).

## Install

```bash
go get github.com/diogoX451/archon-llm-gateway@latest
```

## Usage

```go
reg := llmgateway.NewRegistry()
reg.Register(myOpenAIAdapter{})
reg.FallbackProvider = "ollama"
used, resp, err := reg.Generate(ctx, "openai", llmgateway.Request{
    Model: "gpt-4o-mini",
    User:  "hello",
})
```

## License

Apache-2.0
