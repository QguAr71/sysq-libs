# lnet

Lazarus Network — LLM proxy server with routing, health checks, and metrics.

## Architecture

```
Client → lnet (:9998) → upstream LLM API (DeepSeek/OpenAI compatible)
```

## Packages

| Package | Description |
|---------|-------------|
| `core/` | Router, config, metrics, health, graceful shutdown |
| `modules/input/` | HTTP proxy — accepts OpenAI-compatible requests |
| `modules/output/` | DeepSeek API client |
| `modules/common/` | Shared headers, token saver |

## API

```go
cfg := core.Config{
    ListenAddr: ":9998",
    UpstreamURL: "https://api.deepseek.com/v1",
    APIKey: "...",
}
server, err := core.NewServer(cfg)
err := server.Start()
```

## Usage

Used by `cmd/lazarus-network` — standalone proxy binary. Routes LLM requests from agents to DeepSeek with token counting, metrics, and health checks.
