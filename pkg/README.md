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
| `modes/` | Maszyna stanów trybów operacyjnych | Lazarus |
| `types/` | Typy epistemiczne (Origin, Epistemic) | Lazarus (citadel) |

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
5. Aktualizuj ten README
