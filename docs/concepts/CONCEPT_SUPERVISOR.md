# CONCEPT: Supervisor — Unit Health Orchestrator Library (sysq-libs)

> Data: 2026-06-28 00:27
> Sesja: 20260627_9
> Autor: Echo + SysQ
> Status: KONCEPT
> Lokalizacja: sysq-libs/pkg/supervisor/supervisor.go
> Zależności: sysq-libs/pkg/lifecycle (SelfTester, DiagnosticReport)

---

## 1. Problem

Lazarus ma 12+ Unitów (Router, SemanticUnit, LLM Scheduler, Access, GraphUnit, Vault...). Każdy może być w stanie READY, DEGRADED, FAILED. Nie ma centralnego komponentu który:
- Odpytuje Unity o stan
- Podejmuje decyzje o degradacji systemu
- Raportuje operatorowi pełny obraz

Kernel ma watchdog (Health co 30s), ale to pasywna obserwacja — nie orkiestracja.

---

## 2. Wizja

Biblioteka — `Supervisor` jako osobny pakiet w sysq-libs. Minimalne API. Zero zależności od Lazarusa.

```go
// sysq-libs/pkg/supervisor/supervisor.go
package supervisor

import (
    "context"
    "sync"
    "github.com/QguAr71/sysq-libs/pkg/lifecycle"
)

// Supervisor zbiera SelfTesty od Unitów i podejmuje decyzje o stanie systemu.
type Supervisor struct {
    units    map[string]lifecycle.SelfTester
    mu       sync.RWMutex
}

func NewSupervisor() *Supervisor

// Register dodaje Unit do zbioru monitorowanych.
func (s *Supervisor) Register(name string, unit lifecycle.SelfTester)

// Unregister usuwa Unit.
func (s *Supervisor) Unregister(name string)

// Collect odpytuje wszystkie Unity o SelfTest i zwraca SystemHealth.
// Timeout configurowalny (domyślnie 5s).
func (s *Supervisor) Collect(ctx context.Context) SystemHealth

// SystemHealth to pełny obraz stanu systemu.
type SystemHealth struct {
    Status           DiagStatus                    `json:"status"`
    Units            map[string]DiagUnitReport     `json:"units"`
    DegradedSince    *time.Time                    `json:"degraded_since,omitempty"`
    RecommendedAction string                       `json:"recommended_action"`
}

type DiagUnitReport struct {
    Report  lifecycle.DiagnosticReport `json:"report"`
    Since   time.Time                  `json:"since"`
}
```

---

## 3. API — tylko to co potrzebne

```go
func NewSupervisor() *Supervisor

func (s *Supervisor) Register(name string, unit lifecycle.SelfTester)
func (s *Supervisor) Unregister(name string)
func (s *Supervisor) Collect(ctx context.Context) SystemHealth
```

Bez `AutoRepair` — to należy do Kernela/Modes w Lazarusie, nie do biblioteki.

Bez EventBus — Supervisor to czysty kolektor, nie publikuje. Publikowanie to zadanie Kernela lub Modes.

---

## 4. Logika SystemHealth

```
Dla każdego Unitu:
  SelfTest() → DiagnosticReport
  DiagReady → OK
  DiagDegraded → zlicz
  DiagFailed → zlicz

Status systemu:
  Wszystkie DiagReady → READY
  1+ DiagDegraded, zero DiagFailed → DEGRADED
  1+ DiagFailed → FAILED

RecommendedAction:
  READY: "none"
  DEGRADED: log + opcjonalnie Modes.Transition(Degraded)
  FAILED: "operator intervention required"
```

---

## 5. Kontrakt

1. `Collect()` nie zmienia stanu Unitów. Tylko odpytuje.
2. Timeout przez `ctx` — `Collect` nie może blokować Kernela >5s.
3. Thread-safe — `Register`/`Unregister`/`Collect` mogą być wołane z różnych goroutines.
4. Jeśli Unit nie implementuje `SelfTester` — nie jest rejestrowany. Nie ma fallbacku.
5. Supervisor nie wie o must-unit/nice-unit. To Kernel decyduje.

---

## 6. Zależności

| Zależność | Po co |
|-----------|-------|
| `sysq-libs/pkg/lifecycle` | `SelfTester`, `DiagnosticReport` |
| `context` | Timeout |
| `sync` | RWMutex |
| `time` | Timestamp |

**Zero zależności od Lazarusa, EventBus, MCP, Modes.** Czysta biblioteka.

---

## 7. Testy

```go
func TestSupervisor_AllReady(t *testing.T) {
    s := supervisor.NewSupervisor()
    s.Register("unit-a", &mockTester{status: lifecycle.DiagReady})
    s.Register("unit-b", &mockTester{status: lifecycle.DiagReady})

    health := s.Collect(context.Background())
    assert.Equal(t, lifecycle.DiagReady, health.Status)
}

func TestSupervisor_OneDegraded(t *testing.T) {
    s := supervisor.NewSupervisor()
    s.Register("unit-a", &mockTester{status: lifecycle.DiagReady})
    s.Register("unit-b", &mockTester{status: lifecycle.DiagDegraded})

    health := s.Collect(context.Background())
    assert.Equal(t, lifecycle.DiagDegraded, health.Status)
}

func TestSupervisor_OneFailed(t *testing.T) {
    s := supervisor.NewSupervisor()
    s.Register("unit-a", &mockTester{status: lifecycle.DiagReady})
    s.Register("unit-b", &mockTester{status: lifecycle.DiagFailed})

    health := s.Collect(context.Background())
    assert.Equal(t, lifecycle.DiagFailed, health.Status)
}

func TestSupervisor_ConcurrentRegister(t *testing.T) {
    s := supervisor.NewSupervisor()
    // współbieżne Register + Collect
}
```

---

## 8. Integracja z Lazarusem

Supervisor jako biblioteka — Lazarus tylko importuje:

```go
// cmd/lazarusd/kernel_deploy.go
import "github.com/QguAr71/sysq-libs/pkg/supervisor"

func initSupervisor() {
    globalSupervisor = supervisor.NewSupervisor()

    // Rejestruj Unity które implementują SelfTester
    for name, unit := range kernel.Units() {
        if tester, ok := unit.(lifecycle.SelfTester); ok {
            globalSupervisor.Register(name, tester)
        }
    }
}

// cmd/lazarusd/units/supervisor_unit.go
type SupervisorUnit struct {
    supervisor *supervisor.Supervisor
    bus        bus.Publisher
    mode       *modes.Manager
}

func (u *SupervisorUnit) Start(ctx context.Context) error {
    health := u.supervisor.Collect(ctx)
    // Publikuj na EventBus
    u.bus.Publish(ctx, "diagnostics.collected", health)
    // Degraduj jeśli trzeba
    if health.Status == lifecycle.DiagDegraded {
        u.mode.Transition(modes.Degraded, "supervisor: units degraded")
    }
}
```

---

## 9. Co NIE wchodzi w scope

- `AutoRepair` — to Kernel/Modes w Lazarusie
- EventBus subskrypcje — Supervisor nie słucha, tylko raportuje
- MCP — to Lazarus
- Historia — poza MVP
- Logika must-unit/nice-unit — to Kernel

---

*Echo + SysQ — 2026-06-28 00:27*
*Następny krok: BLUEPRINT*
