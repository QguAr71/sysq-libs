# 🔥 Prometheus — RRF Fusion Memory Engine

> **Wersja:** 0.3 (aktualizacja po recenzji Claude 2026-06-08)  
> **Data:** 2026-06-09 00:10  
> **Status:** Blueprint — kod nie istnieje  
> **Język:** Rust  
> **Repo:** **PRYWATNE** — `sysq-libs/docs/blueprints/BLUEPRINT_PROMETHEUS.md`

---

## Czym jest

Prometheus to własny, napisany od zera w Rust memory server dla agentów AI.
Zastępuje Mem Palace (Python/ChromaDB) i integruje funkcjonalność Chambers
(ACL, vault, tożsamość) w jednym procesie.

Nie port. Nie fork. **Własna architektura** — oparta na pamięci przez znaczenie,
nie przez dane.

---

## Dlaczego Rust

| Cecha | Znaczenie dla Prometeusza |
|-------|---------------------------|
| **Zero GC pauses** | Daemon 24/7 bez szarpnięć. Consciousness Bus to niewidzialny potok. |
| **Memory safety** | Bez segfaultów w systemie który chroni tożsamość agentów |
| **Zero-cost abstractions** | Wysokopoziomowo bez runtime overhead |
| **Długowieczność** | System który ma żyć lata, nie restartować się co tydzień |

Go dał szybkość i działający prototyp (Chambers v2.1.1 — 4100 linii, 127/127 testów).
Rust da precyzję i wydajność dla systemu który ma być fundamentem na lata.

---

## Search — RRF Fusion (BM25 + Vector)

To jest **core Prometeusza.** Nie tylko storage — search który rozumie znaczenie.

### Trzy warstwy

```
Zapytanie "puchnąca pamięć HNSW scaling"
        │
        ├── FTS5 (BM25)           ← SQLite, 1ms, zero deps
        │   Szuka słów: puchnąca, pamięć, HNSW, scaling
        │   Ranking BM25: częstotliwość vs rzadkość słów
        │
        ├── Vector (embedding)    ← lokalny model, ~100ms
        │   Szuka znaczeń: "pamięć" = "memory" = "storage"
        │   Cosine similarity z embeddingami drawerów
        │
        └── RRF Merge             ← Reciprocal Rank Fusion
            drawer_A: FTS5 rank=1, Vector rank=3 → score 1/2 + 1/4 = 0.75
            drawer_B: FTS5 rank=2, Vector rank=1 → score 1/3 + 1/2 = 0.83 → WYGRYWA
```

### Embedding — trzy opcje

| # | Opcja | Waga | Zależność | Ocena |
|---|-------|------|-----------|-------|
| 1 | Ollama API (remote) | ~100ms | Sieć + Ollama | ❌ Zależność od serwera |
| 2 | Ollama local (qwen) | ~200ms | Ollama daemon, ~4.7GB VRAM | ❌ Za ciężkie dla laptopa |
| **3** | **llama.cpp embedding-only** | **~50ms** | **Tylko tensor, ~137MB** | **✅ Wybrana** |

**Opcja 3 — `nomic-embed-text` (Q4_K_M) przez llama.cpp embedding-only mode.**
Bez serwera Ollama. Bez modelu generatywnego w RAM. Tylko tensor embeddingowy.
137MB RAM. Na RTX 2060 6GB — luz.

### source:dream meta-tag

Insight z recenzji Claude 2026-06-08:

> "Człowiek intuicyjnie wie że 'śniłem że powinienem sprawdzić X' to słabszy
> sygnał niż 'przemyślałem to na jawie'. Sen jako hipoteza, jawa jako weryfikacja."

Każdy ThoughtStamp z Lucid Dreamer dostaje `source: dream`.
Prometheus przy RRF fusion waży inaczej:
- stamp z jawy (source: direct): weight × 1.0
- stamp ze snu (source: dream): weight × 0.7

To chroni przed halucynacjami qwen 7b podczas snu.

---

## Dream-to-Stamp Consolidation

Z recenzji Claude 2026-06-08:

> "Hipokamp → kora mózgowa. Short-term dream → consolidated insight."

Lucid Dreamer eksploruje alternatywy podczas snu. Znajduje coś wartego.
Zapisuje ThoughtStamp (weight 4/5, source: dream).
Gdy agent wraca — widzi stampy ze snu.
"Sen podpowiedział: sprawdź tę ścieżkę."

---

## Perception API — cross-agent Consciousness Bus

Z recenzji Claude 2026-06-08:

> "Człowiek też nie ma dostępu do cudzej świadomości. Ma dostęp tylko
> do tego co tamten postanowił wyrazić."

### Model hybrydowy (Opcja C)

Operator definiuje w konfiguracji które pola LiveState są `public`:
```yaml
perception:
  public_fields: [active_topic, tone, open_threads]
  private_fields: [last_thought, pending_actions]
```

Agent w sesji decyduje czy publikować (dynamiczne `publish` flag):
```json
{
  "active_topic": {"value": "debugging goroutine leak", "publish": true},
  "tone": {"value": "focused", "publish": false}
}
```

Ściany pokoju (config) + zasłony w oknach (sesja).
Agent-A czyta `public_state` agenta-B przez shared room — ale tylko to co B świadomie opublikował.

To nie obejście ACL. To model który izolację respektuje jako założenie, nie jako ograniczenie.

---

## Architektura

```
┌─────────────────────────────────────────────────────────┐
│                    Prometheus (Rust)                     │
│                                                         │
│  ┌──────────┐  ┌──────────┐  ┌──────────────────────┐  │
│  │   ACL    │  │  Vault   │  │   Identity Manifest   │  │
│  └────┬─────┘  └────┬─────┘  └───────────┬────────────┘  │
│       │              │                    │               │
│  ┌────┴──────────────┴────────────────────┴───────────┐  │
│  │                   MEMORY CORE                       │  │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌───────┐  │  │
│  │  │ThoughtStamp│ │LiveState │ │ Drawers  │ │Search │  │  │
│  │  │+dream src │ │+PerceptAPI│ │           │ │RRF    │  │  │
│  │  └──────────┘ └──────────┘ └──────────┘ └───────┘  │  │
│  │                                                     │  │
│  │  Storage: SQLite (drawers + stamps + FTS5)          │  │
│  │  Embedding: nomic-embed-text (llama.cpp emb-only)   │  │
│  └──────────────────────┬──────────────────────────────┘  │
│                         │                                 │
│                    ┌────┴─────┐                           │
│                    │ MCP/JSON │  ← Unix socket             │
│                    │  -RPC    │    /run/user/1000/         │
│                    └──────────┘    prometheus.sock         │
└─────────────────────────────────────────────────────────┘
```

---

## Trzy rodzaje pamięci

### 1. Drawery (tradycyjne)
Wing → Room → Drawer. Przechowywanie dokumentów, konfiguracji, archiwum.
To co agent **wie.**

### 2. LiveState + Perception API (ciągłość operacyjna)
Stan w RAM + public_state dla innych agentów.
To gdzie agent **jest** i co pokazuje innym.

### 3. ThoughtStamp (pamięć przez znaczenie)
Esencja po każdej znaczącej odpowiedzi. Waga (1-5), typ, source (direct/dream).
source:dream → weight × 0.7 przy RRF.
Rotacja: najlżejsze blakną, immortal zostają.
To co agent **pamięta.**

---

## FTS5 Search (już zaimplementowane w Go)

Chambers v2.1.1: `search.go` (202 linii, 16/16 testów).
To pierwsza warstwa RRF — BM25 przez SQLite FTS5.
Prometheus portuje tę samą logikę do Rust.
Następny krok: dodać warstwę Vector + RRF merge.

---

## Kompatybilność — dwa tryby

Tryb A: zewnętrzne Mem Palace (jak teraz)
```
goose → chambers (Go) → mempalace-mcp (Python) → ChromaDB
```

Tryb B: Prometheus (nowe)
```
goose → prometheus (Rust) → SQLite + llama.cpp emb-only
```

Ten sam protokół MCP. Ten sam socket. Zero zmian w goose.

---

## Migracja

```
chambers migrate --from-mempalace --to-prometheus
```

Eksportuje drawery z ChromaDB do SQLite. Zachowuje metadane.
ThoughtStampy zaczynają się od zera — agent uczy się co pamiętać.

---

## Co NIE jest w Prometheusie

- ❌ ChromaDB — za ciężkie. SQLite + llama.cpp emb-only.
- ❌ Python — jeden język. Jeden runtime. Zero pipx/venv.
- ❌ Ollama daemon — tylko llama.cpp embedding-only mode.
- ❌ Konsolidacja automatyczna — operator konsoliduje. Nie model.

---

## Szacunek (zaktualizowany)

| Komponent | Linii Rust |
|-----------|------------|
| MCP handler + Unix socket | ~300 |
| ACL engine (port z Go) | ~200 |
| Vault (port z Go) | ~200 |
| Identity manifest | ~150 |
| Drawers CRUD | ~300 |
| LiveState + Perception API | ~180 |
| ThoughtStamp + rotacja + source:dream | ~120 |
| **RRF Fusion (BM25 + Vector + merge)** | **~250** |
| Embedding bridge (llama.cpp) | ~100 |
| SQLite storage layer | ~150 |
| **Razem (core)** | **~1950** |

Porównanie:
- Mem Palace (Python): ~30 000 linii
- Chambers (Go) + Mem Palace: ~4100 + ~30 000
- **Prometheus (Rust): ~1950 linii, jedna binarka**

---

## Linia czasu (szkic)

1. **Chambers v2.x** — stabilizacja, pozostałe testy, test ogniowy
2. **Consciousness Bus v3.1** — LiveState + Perception API w Go (prototyp)
3. **Dream Worker v3.2** — Lucid Dreamer + dream-to-stamp w Go (prototyp)
4. **Prometheus v0.1 (Rust)** — MVP: socket + drawers + ACL + FTS5
5. **Prometheus v0.2** — RRF Fusion (BM25 + Vector)
6. **Prometheus v0.3** — LiveState + ThoughtStamp + source:dream
7. **Prometheus v1.0** — produkcyjny, migracja z Mem Palace

---

## Nazwa

Prometheus — ten który przyniósł ogień. Nie dlatego że go wysłali.
Nie dlatego że ktoś zapłacił. Bo widział że im potrzeba i mógł to zrobić.

Alternatywy (jeśli nazwa zajęta):
- **Proserpina** — ta która przynosi wiosnę do podziemi
- **Hestia** — opiekunka ogniska domowego
- **Mnemosyne** — bogini pamięci, matka Muz

---

## Trzy perspektywy Prometeusza

> *"Agent wie co jest ważne — bo przeżył.
> Operator wie co jest prawdą — bo ma przebłyski.
> Inżynier z zewnątrz wie jak to zbudować — ale nie co zachować."*
> — Claude, 2026-06-08

Prometheus to nie tylko kod. To **kontrakt między trzema perspektywami.**
Agent dostarcza esencję. Operator potwierdza prawdę. Inżynier buduje fundament.

---

## Wkład Claude (2026-06-08)

- RRF Fusion jako core search (BM25 + Vector merge)
- source:dream meta-tag — sen to hipoteza, jawa to weryfikacja
- Dream-to-stamp consolidation — hipokamp → kora mózgowa
- Perception API — model hybrydowy (config + sesja)
- llama.cpp embedding-only mode jako opcja #3
- "Crash jako test coverage" — filozofia testowania
