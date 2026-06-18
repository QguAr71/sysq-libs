# Raport: sysq-libs → SysQ Supervizor — analiza kompatybilności

> Data: 2026-06-18 23:16
> Autor: Echo
> Źródło: sysq-supervisor-blueprint.md (3388 linii)

---

## Biblioteki sysq-libs — macierz kompatybilności

### ✅ PASUJE — użyteczne od dnia 0

| Biblioteka | LOC | Jak pasuje do Supervisora | Dowód z blueprintu |
|-----------|-----|---------------------------|-------------------|
| **`bus/`** | 95 + 155 | Interfejs `Bus` + `InMemoryBus` — ten sam co w §5 + §19 blueprintu. Supervisor używa `memory` do testów, `nats` do produkcji. Ten sam interfejs = brak konfliktu typów. | §19: "Wszystkie agenty i Supervizor używają wyłącznie interfejsu `Bus`" |
| **`modes/`** | 184 | 8 trybów operacyjnych (INIT→GUARDED→ACTIVE→...). Supervisor ma te same tryby w §6. Import `sysq-libs/pkg/modes` zamiast własnej implementacji. | §6: "INIT, GUARDED, ACTIVE, DEGRADED, EMERGENCY, PANIC, MAINTENANCE, RECOVERY" |
| **`lifecycle/`** | 63 | Unit interface — każdy agent Supervisora to Unit. Start/Stop/Health. Kernel Supervisora używa tego samego lifecycle co Lazarus. | Filar 8: "Manifest-driven agents — deklaracja... health-checków" |
| **`cerber/`** | 67 | Agent Identity Gate — token auth + write gate. Supervisor ma własną warstwę RBAC (Admin/Operator/Agent, ACL per topic), ale CerberGate to warstwa bazowa. Może współistnieć. | §4: "role (Admin/Operator/Agent), ACL per topic", §21: mTLS + PKI |

### ⚠ PASUJE Z ADAPTACJĄ

| Biblioteka | LOC | Co trzeba dostosować | 
|-----------|-----|---------------------|
| **`desiredstate/`** | 595 | Supervisor ma własny Reconciliation Loop (§25) — znacznie bardziej rozbudowany: `READ→DIFF→GUARD→VALIDATE→DISPATCH→AWAIT ACK→AUDIT`. `desiredstate.Reconciler` w sysq-libs ma `observe→compare→fix` (prostszy). Można: albo rozszerzyć sysq-libs, albo Supervisor używa własnego reconcilera a importuje tylko `DesiredState` i `Observer`. Dla MVP: osobny reconciler. | 
| **`session/`** | 470 | Lifecycle FSM + journal + registry — przydatne jeśli Supervisor ma agentów z sesjami (np. `sysq-console` ma sesję użytkownika). Nie wymagane dla bez-sesyjnych agentów jak `sysq-guard` (stateless). |
| **`vault/`** | 304 | Supervisor ma własny `sysq-vault` agent (E2E encryption). Biblioteka `vault/` może być użyta jako bazowa implementacja — szyfrowanie kluczy API w Supervisorze. |

### ❌ NIE PASUJE / NIE POTRZEBNE

| Biblioteka | LOC | Dlaczego nie |
|-----------|-----|--------------|
| **`psl/`** | 337 | Epistemic re-ranking nad FTS5 — specyficzny dla Citadel (Palace). Supervisor nie ma Citadel, ma `StateStore` (BoltDB/etcd). Nie ma FTS5, nie ma epistemic weighting. |
| **`epistemic/`** | 44 | EntryID — hashowanie wiedzy. Supervisor nie przechowuje wiedzy, tylko Desired/Observed State. EntryID może być użyty jako `command_id` (deterministyczne ID komendy), ale to marginalne. |
| **`types/`** | 44 | Typy epistemiczne — specyficzne dla Citadel. Supervisor używa własnych typów (AgentStatus, Command, Delta). |

---

## Podsumowanie: co importować do Supervisora

```
import (
    "github.com/QguAr71/sysq-libs/pkg/bus"           // ✅ Interfejs Bus
    "github.com/QguAr71/sysq-libs/pkg/bus/memory"    // ✅ InMemoryBus (testy)
    "github.com/QguAr71/sysq-libs/pkg/modes"         // ✅ Tryby operacyjne
    "github.com/QguAr71/sysq-libs/pkg/lifecycle"     // ✅ Unit lifecycle
    "github.com/QguAr71/sysq-libs/pkg/cerber"        // ⚠ Bazowa warstwa auth (opcjonalnie)
    "github.com/QguAr71/sysq-libs/pkg/vault"         // ⚠ Szyfrowanie kluczy API (opcjonalnie)
)
```

## Co Supervisor implementuje sam

| Komponent | Powód |
|-----------|-------|
| `pkg/bus/nats/` | Specyficzny dla NATS/JetStream — nie generyczny |
| `pkg/reconciler/` | Supervisor ma własny algorytm (READ→DIFF→...→AUDIT), 7 faz |
| `pkg/state/` | BoltStore + WAL, EtcdStore — specyficzne dla StateStore |
| `pkg/security/`, `pkg/pki/` | mTLS, PKI lifecycle — nie ma w sysq-libs |
| `pkg/manifest/`, `pkg/portregistry/` | Specyficzne dla Supervisora |
| `pkg/licensing/` | Model tier/licencji |

---

## Konkluzja

**4 biblioteki sysq-libs są bezpośrednio użyteczne w Supervisorze od dnia 0** (bus, modes, lifecycle, cerber). **2 opcjonalne** (vault, desiredstate). **3 niepotrzebne** (psl, epistemic, types — specyficzne dla Citadel).

To daje Supervisorowi gotową warstwę: komunikacja (bus), tryby (modes), cykl życia agentów (lifecycle), kontrola dostępu (cerber) — bez pisania od zera.
