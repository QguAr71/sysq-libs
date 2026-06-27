# llmsched

Single-GPU cooperative scheduler for LLM inference.

## Architecture

One worker goroutine, two queues (HIGH/LOW), credit-based bounded fairness.

```
Submit(priority, prompt) → queue → worker → Ollama API → result
```

## API

```go
sched := llmsched.New(llmsched.Config{
    OllamaURL:    "http://localhost:11434",
    Model:        "phi3.5:3.8b",
    MaxInflight:  1,
    HighCredits:  10,   // max HIGHs between LOWs
    Timeout:      30 * time.Second,
    SingleFlight: true, // cache identical prompts
})

result, err := sched.Submit(ctx, llmsched.PrioHigh, "Summarize: ...")
```

## Priorities

- `PrioHigh` — blocking, timeout (Router classification)
- `PrioLow` — async, retryable (GraphUnit, background tasks)

## Invariants

- Max 1 inflight request at a time (single GPU)
- Max 10 HIGH between LOW (bounded fairness)
- Dual-layer timeout (queue wait + HTTP request)
- Singleflight cache: identical prompts → one HTTP request, broadcast result

## Tests

6 tests PASS: bounded fairness, backpressure, timeout, shutdown, stats, singleflight.

## Files

| File | Description |
|------|-------------|
| `scheduler.go` | Core scheduler |
| `scheduler_test.go` | 6 tests |
