# sysq-libs — Mapa: Biblioteki → Protokoły → Zastosowania

> Data: 2026-06-18
> Autor: Echo & SysQ
> Status: Dokumentacja kanoniczna

---

## Biblioteki

| Biblioteka | Import | LOC | Opis |
|-----------|--------|-----|------|
| `lifecycle/` | `sysq-libs/pkg/lifecycle` | 63 | Unit abstrakcja — każdy komponent to Unit |
| `cerber/` | `sysq-libs/pkg/cerber` | 67 | Gate tożsamości agentów — ACL, write gate |
| `epistemic/` | `sysq-libs/pkg/epistemic` | 44 | EntryID — deterministyczne hashowanie wiedzy |
| `types/` | `sysq-libs/pkg/types` | 44 | Typy epistemiczne — Origin, Epistemic, Confidence |
| `psl/` | `sysq-libs/pkg/psl` | 337 | Personalized Search Layer — epistemic re-ranking |
| `modes/` | `sysq-libs/pkg/modes` | 184 | Maszyna stanów — 8 trybów operacyjnych |
| `vault/` | `sysq-libs/pkg/vault` | 304 | Szyfrowanie AEAD + honeypoty + mirror |
| `session/` | `sysq-libs/pkg/session` | 470 | Lifecycle FSM + append-only journal + registry |
| `desiredstate/` | `sysq-libs/pkg/desiredstate` | 595 | Desired State engine + observer + reconciler |

---

## Protokoły

Każdy protokół ma publiczną specyfikację. Biblioteka jest implementacją referencyjną.

### Cerber Protocol — Agent Identity Gate

| Pole | Wartość |
|------|---------|
| Specyfikacja | `docs/concepts/sysq-ai/CERBER_PROTOCOL.md` |
| Implementacja | `cerber/gate.go` + `mcp/auth.go` |
| Status | v1.0 — w produkcji |

**Zastosowania:**

| # | Zastosowanie | Opis |
|---|-------------|------|
| 1 | AI Gateway / API Management | Każdy agent AI przechodzi przez Cerber zanim dotknie danych |
| 2 | Enterprise Agent Firewall | 50 agentów — który może pisać, który tylko czytać |
| 3 | Multi-Tenant SaaS z AI | Klient A nie widzi danych klienta B |
| 4 | Agent Identity Provider | SSO dla agentów AI |
| 5 | Compliance / Audit Trail | Każda próba dostępu logowana |
| 6 | Home Automation Security | Agent od światła ≠ agent od drzwi |

### Anubis Protocol — Epistemic Re-Ranking

| Pole | Wartość |
|------|---------|
| Specyfikacja | `docs/spec/PSL_SPEC_v1.md` |
| Implementacja | `psl/psl.go` |
| Status | v1 w produkcji, v2 w koncepcie |

**Zastosowania:**

| # | Zastosowanie | Opis |
|---|-------------|------|
| 1 | Enterprise Knowledge Management | Wyszukiwarka która waży pochodzenie wiedzy |
| 2 | AI Hallucination Guard | [inferred] vs [referenced] — LLM nie blefuje |
| 3 | Medical Decision Support | Badania kliniczne > opinia eksperta > AI ekstrapolacja |
| 4 | Legal Research | Precedens > komentarz > artykuł blogowy |
| 5 | Fake News Detection | Waży źródło — nie cenzuruje |
| 6 | Personal Second Brain | Najpierw co przeżyłeś, potem co przeczytałeś |

### Epistemic Entry ID — Deterministic Knowledge Hashing

| Pole | Wartość |
|------|---------|
| Specyfikacja | `docs/concepts/intelligence/EPISTEMOLOGY_VS_PROVENANCE.md` |
| Implementacja | `epistemic/entryid.go` + `types/epistemic.go` |
| Status | v1.0 — w produkcji |

**Zastosowania:**

| # | Zastosowanie | Opis |
|---|-------------|------|
| 1 | Agent Knowledge Exchange | Agent A i B — ten sam fakt = ten sam ID, bez centralnej bazy |
| 2 | Federated Knowledge Graphs | Wspólny ID → łączenie grafów bez duplikatów |
| 3 | Anti-Plagiarism / Anti-Deepfake | Zmieniony origin → INNY ID → widać fałszerstwo |
| 4 | Content Authenticity | Człowiek [self] vs AI [inferred] — ID zawiera origin |
| 5 | Decentralized Knowledge Base | Wikipedia 2.0: każdy fakt ma ID, można zweryfikować |
| 6 | Blockchain for Knowledge | ID jako hash — niezmienny dowód istnienia faktu |

---

## Zastosowania platformowe

### Lazarus jako OS dla tożsamości agentów AI

Z `docs/concepts/sysq-ai/LAZARUS_JAKO_PLATFORMA.md`:

```
Warstwa 0: Jądro (zbudowane)
  Citadel v7, Wing, MCP Bridge, GC/Merge

Warstwa 1: Platforma agentowa (multi-SuperWing)
  operator/vault, agent-*/default, system/library, system/conference

Warstwa 2: Komunikacja między agentami
  MCP Bridge + EventBus + Sala Konferencyjna
```

### 7 wektorów poza AI

Z `docs/concepts/philosophy/WEKTORY_ZASTOSOWANIA.md`:

| # | Wektor | Opis |
|---|--------|------|
| 1 | **Open Source Governance** | `.lazarus/` w repo — cała historia decyzji |
| 2 | **Compliance & Audyt** | Regulator widzi kto, co, kiedy, dlaczego |
| 3 | **Software Due Diligence** | Przed akwizycją: nie tylko kod, ale DLACZEGO |
| 4 | **Edukacja** | Student widzi proces decyzyjny, nie tylko wynik |
| 5 | **Research** | Pełna ścieżka eksperymentu z replay |
| 6 | **Enterprise AI Governance** | Korporacja kontroluje swoją wiedzę |
| 7 | **Personal Knowledge Management** | Deweloper: osobista baza wiedzy z gitem |

### Biuro Pracy — dynamiczne teamy agentów

Z `docs/plans/BIURO_PRACY.md`:

- Operator definiuje potrzebę → EventBus → agenci-specjaliści odpowiadają
- System zatrudnia ich do zadania → Sala Konferencyjna → output → zwolnienie
- Każdy specjalista = osobna binarka, osobny SuperWing

### SysQ AI Augmentation — produkt

Z `docs/concepts/sysq-ai/SYSQ_AI_AUGMENTATION_ZASTOSOWANIA.md`:

- Każdy klient = SuperWing w repo (`git clone` = kod + pamięć)
- Session Compactor — agent pamięta 50 sesji zamiast 3
- GitHub = drugi mózg (offsite backup, synchronizacja)
- Automatyzacje nocne — Reconciler, GC, Merge, Pattern Scanner
- Produkt: "AI Memory for Projects"

---

## Architektura zaufania

Z `docs/concepts/sysq-ai/CONCEPT_TRUST_ARCHITECTURE.md`:

| Warstwa | Mechanizm | Status |
|---------|-----------|--------|
| 1. JAWNOŚĆ | Protokoły i specyfikacje otwarte | ✓ |
| 2. ODTWARZALNOŚĆ | Reproducible Build | Plan |
| 3. AUDYT | Niezależny audyt kodu | Plan |
| 4. IZOLACJA | Agent w sandboxie | ✓ (Cerber + Citadel owner check) |
| 5. CIĄGŁOŚĆ | Git log + SBOM + signed commits | Częściowo |

---

## Linia czasu projektów

Z `docs/concepts/sysq-ai/LINIA_CZASU_PROJEKTY.md`:

| Projekt | Repo | LOC | Rola |
|---------|------|-----|------|
| `qguar-config` | github.com/QguAr71/qguar-config | 303 ZSH | Pra-początek: rollback, tryby, integrity |
| `layered-zsh` | github.com/QguAr71/layered-zsh | 1326 ZSH | Żart → architektura |
| `Cytadela` | github.com/QguAr71/Cytadela | 821 bash | System bezpieczeństwa, źródło nazwy |
| `sysqcli` | github.com/QguAr71/sysqcli | 1283 ZSH | Diagnostyka, ewolucja qguar-config |
| `lazarus` | github.com/QguAr71/lazarus | ~20k Go | Kernel tożsamości + pamięć |
| `sysq-libs` | github.com/QguAr71/sysq-libs | ~1k Go | Biblioteki współdzielone |
| `sysq-supervizor` | (plan) | — | Koordynator multi-agent |

---

## Model hybrydowy

Z `docs/decisions/DECISION_CLOSED_SOURCE.md` i `CONCEPT_TRUST_ARCHITECTURE.md`:

| Warstwa | Licencja |
|---------|----------|
| Protokoły (Cerber, Anubis, Epistemic ID) | MIT — otwarte |
| Biblioteki (`sysq-libs/pkg/*`) | MIT — otwarte |
| Core (Lazarus, Supervisor) | Source Available (NDA) |
| Enterprise features | Zamknięte (komercyjne) |
