# CONCEPT: SelfTester — Diagnostic Interface Library (sysq-libs)

> Data: 2026-06-28 00:26
> Sesja: 20260627_9
> Autor: Echo + SysQ
> Status: KONCEPT
> Lokalizacja: sysq-libs/pkg/lifecycle/diagnostic.go
> Zależności: sysq-libs/pkg/lifecycle (Unit, HealthReport)

---

## 1. Problem

Każdy komponent Lazarusa i sysq-libs potrzebuje autodiagnostyki, ale obecnie `lifecycle.Unit` ma tylko `Health()` — pasywne odpytywanie stanu. Brak aktywnego self-testu który Unit może wykonać przed `Start()`.

`Health()` zwraca aktualny stan — ale nie wykonuje testów. SelfTest to aktywna diagnostyka: sprawdza zależności, łączność, poprawność konfiguracji.

---

## 2. Wizja

Biblioteka — interfejs + typy, zero implementacji. Każdy Unit który implementuje `SelfTester` może być automatycznie testowany przez Supervisor.

```go
// sysq-libs/pkg/lifecycle/diagnostic.go
package lifecycle

import "context"

// SelfTester is an optional interface for units that can self-diagnose.
// Supervisor calls SelfTest before Start() and on demand.
type SelfTester interface {
    SelfTest(ctx context.Context) DiagnosticReport
}

// DiagnosticReport to wynik aktywnej autodiagnostyki.
// Różni się od HealthReport — tamten to pasywny stan, ten to wynik testów.
type DiagnosticReport struct {
    Unit    string          `json:"unit"`
    Status  DiagStatus      `json:"status"`
    Checks  []DiagCheck     `json:"checks"`
    Metrics map[string]float64 `json:"metrics,omitempty"`
    Error   string          `json:"error,omitempty"`
}

type DiagStatus string

const (
    DiagReady    DiagStatus = "READY"    // wszystkie testy PASS
    DiagDegraded DiagStatus = "DEGRADED" // część testów FAIL, unit działa
    DiagFailed   DiagStatus = "FAILED"   // krytyczne testy FAIL
)

type DiagCheck struct {
    Name      string  `json:"name"`       // np. "ONNX model loaded"
    Passed    bool    `json:"passed"`
    Detail    string  `json:"detail,omitempty"`
    LatencyMs float64 `json:"latency_ms,omitempty"`
}
```

---

## 3. Kontrakt

1. `SelfTest` NIE może blokować `Start()`. Timeout przez `ctx`.
2. `SelfTest` NIE może zmieniać stanu Unitu. Tylko diagnostyka.
3. `DiagDegraded ≠ DiagFailed`. Degraded = unit startuje, Failed = unit nie startuje.
4. `DiagCheck` jest atomowy — jeden check = jeden warunek (np. "classifier slot active").
5. `Metrics` to opcjonalne metryki (latency, memory, count). Nie wymagane.

---

## 4. Przykład użycia

```go
// Router SelfTest
func (r *Router) SelfTest(ctx context.Context) lifecycle.DiagnosticReport {
    checks := []lifecycle.DiagCheck{
        {Name: "classifier_slot", Passed: r.slots[0] != nil, Detail: "regex classifier"},
        {Name: "rule_engine", Passed: r.engine != nil, Detail: fmt.Sprintf("%d rules", len(r.engine.Rules()))},
        {Name: "embedding_slot", Passed: r.semantic != nil, Detail: "MiniLM ONNX"},
        {Name: "fallback_slot", Passed: true, Detail: "diary"},
    }

    status := lifecycle.DiagReady
    for _, c := range checks {
        if !c.Passed {
            status = lifecycle.DiagDegraded
        }
    }

    return lifecycle.DiagnosticReport{
        Unit:    r.swName,
        Status:  status,
        Checks:  checks,
    }
}
```

---

## 5. Zależności

| Zależność | Po co |
|-----------|-------|
| `context` | Timeout SelfTestu |
| Nic więcej | Czysty interfejs |

**Zero zależności od Lazarusa, Citadel, Router, MCP.** To jest biblioteka.

---

## 6. Testy

```go
// sysq-libs/pkg/lifecycle/diagnostic_test.go
func TestSelfTest_AllPassed(t *testing.T) {
    tester := &mockTester{status: DiagReady}
    report := tester.SelfTest(context.Background())
    assert.Equal(t, DiagReady, report.Status)
}

func TestSelfTest_Degraded(t *testing.T) {
    tester := &mockTester{status: DiagDegraded}
    report := tester.SelfTest(context.Background())
    assert.Equal(t, DiagDegraded, report.Status)
}

func TestSelfTest_Timeout(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
    defer cancel()
    tester := &slowTester{}
    report := tester.SelfTest(ctx)
    assert.Equal(t, DiagFailed, report.Status)
}
```

---

## 7. Co NIE wchodzi w scope

- Implementacja SelfTest dla konkretnych Unitów — to w Lazarusie
- Supervisor — osobny concept
- MCP `lazarus__diagnostics` — to w Lazarusie
- Historyczne metryki — poza MVP
- Auto-naprawa — to w Supervisor

---

*Echo + SysQ — 2026-06-28 00:26*
*Następny krok: BLUEPRINT*
