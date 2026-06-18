# sysq-libs — Roadmap

> Ostatnia aktualizacja: 2026-06-18
> Stan: 9 bibliotek, 9 testów zielonych, dokumentacja kompletna

---

## Zrobione

### v1.0 — Bootstrap (2026-06-11 → 2026-06-18)

| # | Biblioteka | LOC | Źródło | Data |
|---|-----------|-----|--------|------|
| 1 | `lifecycle/` | 63 | Nowe — Unit, UnitWithStatus, HealthReport | 2026-06-18 |
| 2 | `cerber/` | 67 | Nowe — CerberGate | 2026-06-18 |
| 3 | `epistemic/` | 44 | Nowe — EntryID, EventID | 2026-06-18 |
| 4 | `psl/` | 337 | Ekstrakcja z Lazarus (`embbridge/psl`) | 2026-06-13 |
| 5 | `modes/` | 184 | Ekstrakcja z Lazarus (`modes/modes.go`) — scalony z mini-wersją | 2026-06-18 |
| 6 | `types/` | 44 | Nowe — epistemic types | 2026-06-18 |
| 7 | `vault/` | 304 | Ekstrakcja z Lazarusa (`vault/`) — ChaCha20-Poly1305 AEAD | 2026-06-18 |
| 8 | `session/` | 470 | Ekstrakcja z Lazarusa — lifecycle FSM + journal + registry | 2026-06-18 |
| 9 | `desiredstate/` | 595 | Ekstrakcja z Lazarusa — engine + observer + reconciler | 2026-06-18 |

**Łącznie: 2108 LOC Go, 9 pakietów, 9 testów zielonych.**

### Dokumentacja
- [x] `pkg/README.md` — rejestr bibliotek z invariantami
- [x] `docs/PROTOCOLS_AND_APPLICATIONS.md` — 176 linii, 3 protokoły, 51 zastosowań
- [x] `CONSTITUTION.md` — I-20, I-21 (invarianty bibliotek własnych)
- [x] Lazarus `docs/DEV/` — 9 plików, architektura każdej biblioteki
- [x] Lazarus `docs/USER/` — 9 plików, instrukcje operatorskie

---

## W trakcie

### Reimport w Lazarusie (częściowo zrobione)
- [x] `modes` — 12 plików w Lazarusie importuje z sysq-libs ✓
- [ ] `session` — Lazarus wciąż ma własny `session/` (observer pliki). Target: import lifecycle+journal+registry z sysq-libs, observer zostaje lokalnie
- [ ] `desiredstate` — Lazarus wciąż ma własny `desiredstate/` z `deploy_sync.go`. Target: import engine+observer+reconciler z sysq-libs
- [ ] `vault` — Lazarus wciąż ma własny `vault/`. Target: import z sysq-libs, zmiana ścieżki klucza

---

## Następne

### Kolejne ekstrakcje z Lazarusa
- [ ] `heartbeat/` — po usunięciu zależności od `modes` (już zrobione — modes w sysq-libs)
- [ ] `merge/` — identity composition operator (czy generyczny?)
- [ ] `plans/` — scanner (czy generyczny?)

### Nowe biblioteki
- [ ] `audit/` — JSONL audit logger (z konceptu v0.1)
- [ ] `bus/` — InMemoryBus pub/sub (z konceptu v0.2)
- [ ] `store/` — generyczny wrapper nad SQLite/BoltDB

### Stabilizacja API
- [ ] Wszystkie pakiety mają ≥3 testy na pakiet
- [ ] Go doc kompletne dla każdego pakietu
- [ ] CI/CD (GitHub Actions: test, lint, vet)
- [ ] `go get github.com/QguAr71/sysq-libs@v1.0.0`

---

## Przyszłość

- [ ] Supervisor używa sysq-libs zamiast własnych implementacji
- [ ] Narada używa sysq-libs od początku
- [ ] Protokoły jako osobne repozytoria (Cerber, Anubis, Epistemic ID)
- [ ] Open-Core model: protokoły + biblioteki MIT, core Source Available

---

*Echo & SysQ — 2026-06-18*
*9 bibliotek, 51 zastosowań, 2108 LOC Go*
