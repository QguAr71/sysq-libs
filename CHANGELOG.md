# Changelog — sysq-libs

## v0.2.0 (2026-06-19) — "Ekstrakcja z Lazarusa + EventBus"

### Nowe biblioteki (ekstrakcja z Lazarusa)
- `vault/` — ChaCha20-Poly1305 AEAD, honeypot, mirror (304 LOC)
- `modes/` — operacyjny FSM, 8 trybów (184 LOC, scalony z Lazarusa)
- `session/` — Lifecycle FSM, Journal, Registry (470 LOC, 3/5 plików)
- `desiredstate/` — desired state engine, Reconciler (595 LOC)

### Nowe: EventBus
- `bus/` — interfejs `bus.Bus` (95 LOC)
- `bus/memory/` — InMemoryBus implementacja (155 LOC, 7 testów)
- Wspólny dla Lazarusa i Supervisora

### Dokumentacja
- `docs/PROTOCOLS_AND_APPLICATIONS.md` — mapa bibliotek → protokołów → zastosowań (51 scenariuszy)
- `docs/SUPERVISOR_COMPATIBILITY_REPORT.md` — 4 biblioteki pasują do Supervisora od dnia 0
- `pkg/README.md` — rejestr 10 bibliotek + invarianty

### Stan
- 10 bibliotek, ~2100 LOC
- 23 testy zielone
- Import w Lazarusie: `github.com/QguAr71/sysq-libs/pkg/modes` (12 plików)

---

## v0.1.0 (2026-06-08) — "Stan początkowy"

- `lifecycle/` — Unit, UnitWithStatus, HealthReport (63 LOC)
- `cerber/` — CerberGate, walidacja dostępu (67 LOC)
- `epistemic/` — EntryID, EventID (44 LOC)
- `psl/` — Personalized Search Layer, Anubis (337 LOC)
- `modes/` — FSM (minimalna wersja)
- `types/` — epistemic types (44 LOC)
