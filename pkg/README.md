# sysq-libs/pkg/ — Współdzielone biblioteki SysQ

Każdy katalog to niezależna biblioteka Go importowana jako `github.com/QguAr71/sysq-libs/pkg/<nazwa>`.

Zasada: przed implementacją nowego komponentu w Lazarusie/Supervisorze/Naradzie — sprawdź czy nie ma go tutaj.
Jeśli nie ma, a komponent jest generyczny — zaimplementuj tutaj i zaimportuj.

## Katalogi

| Biblioteka | Opis | Używana przez |
|-----------|------|---------------|
| `lifecycle/` | Unit, UnitWithStatus, HealthReport — bazowe interfejsy systemu unitów | Lazarus (kernel) |
| `cerber/` | CerberGate — walidacja dostępu | Lazarus |
| `epistemic/` | EntryID, EventID — identyfikatory epistemiczne | Lazarus (citadel) |
| `psl/` | Personalized Search Layer — re-ranking wyników FTS5 | Lazarus (embbridge) |
| `modes/` | Maszyna stanów trybów operacyjnych (8 trybów, 184 LOC) | Lazarus |
| `types/` | Typy epistemiczne (Origin, Epistemic) | Lazarus (citadel) |
| `vault/` | ChaCha20-Poly1305 AEAD + honeypoty + mirror (304 LOC) | Lazarus |
| `portfixer/` | Port conflict resolver z graceful degradation (273 LOC) — env → default → scan → fallback | Lazarus (initMCPServer) |
| `session/` | Lifecycle FSM + append-only journal + registry (470 LOC) | Lazarus (kernel_deploy) |
| `desiredstate/` | Desired State engine + observer + reconciler (595 LOC) | Lazarus (kernel_deploy) |

## Invarianty

Zgodnie z konstytucją (`CONSTITUTION.md #I-20, #I-21`):

- **Przed reinwencją** — sprawdź czy funkcjonalność już istnieje w `pkg/`
- **Propagacja** — gdy projekt rozwija generyczny komponent → ekstrakcja do `pkg/`
- **Żywe biblioteki** — rozwijane razem z projektami, nie zamrożone

## Dodawanie nowej biblioteki

1. `mkdir pkg/<nazwa>`
2. Plik `<nazwa>.go` + `<nazwa>_test.go`
3. Zero zależności od Lazarusa — tylko stdlib + zewnętrzne moduły
4. Interfejs jawny, invarianty w komentarzu
5. Dokumentacja DEV + USER w `lazarus/docs/DEV/<nazwa>.md` i `lazarus/docs/USER/<nazwa>.md`
6. Aktualizuj ten README
