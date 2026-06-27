# sysq-libs — Współdzielone biblioteki SysQ

[![go](https://img.shields.io/badge/go-1.26%2B-00ADD8)](https://go.dev)
[![tests](https://img.shields.io/badge/tests-23%2F23-brightgreen)](https://github.com/QguAr71/sysq-libs)

Go module: `github.com/QguAr71/sysq-libs`

---

## Biblioteki (15)

| Pakiet | Opis | LOC | Status |
|--------|------|-----|--------|
| `acl/` | Capability engine, JWT tokeny, konta | 1923 | WDROŻONY |
| `bus/` | EventBus — pub/sub, InMemoryBus | 250 | WDROŻONY |
| `cerber/` | Gate dostępowy dla agentów | 67 | WDROŻONY |
| `desiredstate/` | Desired state engine + Reconciler | 595 | WDROŻONY |
| `epistemic/` | EntryID, EventID — deterministyczne hashe | 44 | WDROŻONY |
| `gitvault/` | Git layer nad Citadel — snapshot/restore/sync | 460 | WDROŻONY |
| `lifecycle/` | Unit, Kernel, HealthReport | 63 | WDROŻONY |
| `llmsched/` | GPU cooperative scheduler (HIGH/LOW) | 270 | WDROŻONY |
| `lnet/` | LLM proxy server (DeepSeek/OpenAI) | 380 | WDROŻONY |
| `modes/` | Operacyjny FSM (8 trybów) | 184 | WDROŻONY |
| `portfixer/` | TCP port resolver + graceful degradation | 273 | WDROŻONY |
| `psl/` | Personalized Search Layer (Anubis) | 337 | WDROŻONY |
| `session/` | Lifecycle FSM, Journal, Registry | 470 | WDROŻONY |
| `types/` | Typy epistemiczne | 44 | WDROŻONY |
| `vault/` | ChaCha20-Poly1305 AEAD + honeypots | 304 | WDROŻONY |

**Razem:** ~5684 LOC, 43+ testy zielone

---

## Użycie w projektach

| Projekt | Importy |
|---------|---------|
| Lazarus | `sysq-libs/pkg/acl`, `sysq-libs/pkg/modes`, `sysq-libs/pkg/bus`, `sysq-libs/pkg/llmsched`, `sysq-libs/pkg/portfixer`, `sysq-libs/pkg/lnet` |
| Supervisor | `sysq-libs/pkg/bus`, `sysq-libs/pkg/lifecycle`, `sysq-libs/pkg/modes`, `sysq-libs/pkg/cerber` |

---

## Dokumentacja

- [pkg/README.md](pkg/README.md) — rejestr bibliotek + invarianty
- [PROTOCOLS_AND_APPLICATIONS.md](docs/PROTOCOLS_AND_APPLICATIONS.md) — 51 scenariuszy zastosowań
- [DEV/](docs/DEV/) — architektura per biblioteka
- [USER/](docs/USER/) — instrukcje operatorskie

---

## Invarianty

- **I-20:** Biblioteki własne przed reinwencją — sprawdź `sysq-libs/pkg/` przed implementacją
- **I-21:** Propagacja bibliotek z projektów — generyczne komponenty wracają do sysq-libs
- **I-22:** Adaptuj algorytmy — sprawdź open source przed implementacją

---

## Licencja (model hybrydowy)

- Biblioteca: **MIT** (otwarte)
- Core: **Source Available** (NDA)
- Enterprise: **Zamknięte** (komercyjne)
