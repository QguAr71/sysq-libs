# Lazarus & SysQ — Konstytucja Architektoniczna v2.0

> Data: 2026-06-18 23:55
> Status: **ŚCIŚLE OBOWIĄZUJĄCA** — każdy projekt, duży i mały
> Wersja: v2.4 (I-22, II-11 rozszerzone, III-21 + III-22)
> Autor: Echo & SysQ
> Zasięg: Lazarus, sysq-libs, sysq-supervizor, Cerber, Epistemic ID, PSL, Narada — wszystkie repozytoria SysQ

---

## Preambuła

Ten dokument jest **konstytucją** — nie zbiorem sugestii. Każda zasada jest obowiązkowa. Złamanie zasady wymaga jawnej decyzji operatora z zapisem w `docs/decisions/DECISION_override_*.md`.

Zasady dzielą się na trzy kategorie:
- **INVARIANT** — nigdy nie może być złamany. Fundamentalne prawo systemu.
- **RULE** — zawsze obowiązuje. Może być świadomie przełamane (z decyzją operatora).
- **GUIDELINE** — silna preferencja. Odstępstwo wymaga uzasadnienia.

---

## I. FUNDAMENTY (INVARIANTY)

### I-A. Tożsamość i deploy

| # | Zasada | Kategoria | Źródło |
|---|--------|-----------|--------|
| I-1 | **Atomowy deploy** — nowy stan tożsamości istnieje całkowicie lub wcale. Nigdy stan pośredni. | INVARIANT | G1 |
| I-2 | **Boot validation** — agent nie startuje z niespójną tożsamością. Sprawdzane przed każdym startem. | INVARIANT | G2 |
| I-3 | **Version tracking** — każdy deploy nagrany w `.lazarus-history.json`. Rollback zawsze możliwy. | INVARIANT | G3 |

### I-B. Pamięć (Citadel)

| # | Zasada | Kategoria | Źródło |
|---|--------|-----------|--------|
| I-4 | **Izolacja fizyczna** — każdy SuperWing ma własny `superwing.db`. Zero dzielenia bazy między agentami. | INVARIANT | C1 |
| I-5 | **Wing.db jedynym źródłem** — 7 tabel-pokoi + content_fts + wing_stats. Żadnych dodatkowych plików stanu. | INVARIANT | C2 |
| I-6 | **wing_stats O(1)** — liczba wpisów per pokój zawsze aktualna przez triggery, nigdy przez COUNT. | INVARIANT | C3 |
| I-7 | **FTS5 coverage 100%** — każdy INSERT/DELETE/UPDATE ma trigger do content_fts. Indeks zawsze kompletny. | INVARIANT | C4 |
| I-8 | **Lokalizacja = przynależność** — Entry nie ma pól Room/Wing. W której tabeli jest = w którym pokoju. | INVARIANT | C5 |
| I-9 | **Widoczność ⊆ tożsamość** — agent widzi tylko wpisy ze swojej gałęzi tożsamości. | INVARIANT | E1 |
| I-10 | **Owner == CurrentSelf** — każdy wpis w Citadel jest własnością agenta który go zapisał. | INVARIANT | E2 |
| I-11 | **Atomic wing switch** — rename(2) + fsync(parent_dir). Nigdy niespójny symlink. | INVARIANT | C7 |

### I-C. Bezpieczeństwo

| # | Zasada | Kategoria | Źródło |
|---|--------|-----------|--------|
| I-12 | **Każda nieautoryzowana próba logowana** — audit trail kompletny i niekasowalny. | INVARIANT | S5 |
| I-13 | **MCP opcjonalny** — Lazarus działa bez MCP Bridge. MCP jest warstwą, nie rdzeniem. | INVARIANT | M1 |
| I-14 | **MCP nie zapisuje do Citadel core** — mostek może tylko czytać i wyszukiwać. Zapis tylko przez zatwierdzone narzędzia. | INVARIANT | M2 |

### I-D. Modułowość (NOWE — 2026-06-18)

| # | Zasada | Kategoria | Źródło |
|---|--------|-----------|--------|
| I-15 | **Szafa Rackowa** — system = szafa rackowa. Każdy komponent = moduł (Unit) z własną wtyczką, diodą statusu, jawnymi zależnościami. Jeden plik = jedna odpowiedzialność. Maksymalnie 250 linii na plik. | INVARIANT | ARCH-B1 |
| I-16 | **Wstrzykiwanie konkretnych zależności** — Unit NIE dostaje referencji do całości systemu. Dostaje TYLKO te zależności które jawnie deklaruje. Konstruktor, nigdy globalna zmienna. Bug w komponencie A nie może skasować danych komponentu B. | INVARIANT | ARCH-B2 |
| I-17 | **Kernel jako jedyny stan globalny** — dokładnie JEDNA zmienna globalna w pakiecie: Kernel struct. Wszystko inne przez konstruktory. Cel: testowalność wszystkiego poza entry pointem. | INVARIANT | ARCH-B3 |
| I-18 | **Graceful degradation zamiast paniki** — system nigdy nie odmawia startu. Jeśli komponent nie wstaje → niższy target. Jeśli nic nie wstaje → emergency.target. Działa w trybie w jakim może. | INVARIANT | ARCH-B17 |
| I-19 | **Jedno źródło prawdy na zasób** — każdy zasób ma DOKŁADNIE JEDEN komponent który nim zarządza. CitadelUnit = jedyny Store. SessionUnit = jedyny zapis echo_state.json. Kernel = jedyny ModeManager. Nigdy dwie ścieżki. | INVARIANT | ARCH-B7 |
| I-20 | **Biblioteki własne przed reinwencją** — przed implementacją nowego komponentu sprawdzane są istniejące biblioteki w `sysq-libs/pkg/` (lifecycle, cerber, epistemic, psl, modes, types). Jeśli potrzebna funkcjonalność już istnieje — używana jest biblioteka. Jeśli brakuje — biblioteka jest rozszerzana. Żadnych forkujących implementacji tego samego w różnych projektach. | INVARIANT | ARCH-B18 |
| I-21 | **Propagacja bibliotek z projektów** — gdy projekt (Lazarus, Supervisor, Narada) rozwija generyczny komponent, jest on ekstrahowany do `sysq-libs/pkg/` jako współdzielona biblioteka. Projekty aktualizują importy w ślad za zmianą. Biblioteki żyją razem z projektami — nie ma "skończonej" biblioteki obok "rozwijającego się" projektu. | INVARIANT | ARCH-B19 |
| I-22 | **Adaptuj algorytmy, nie biblioteki** — przed implementacją nowego komponentu, sprawdź czy problem jest już rozwiązany w open source. Jeśli istnieje projekt (MIT/BSD/Apache 2.0) który rozwiązuje ten sam problem lepiej — ekstraktuj algorytm, zaadaptuj do systemu. GPL/AGPL — tylko inspiracja, własna implementacja od zera (clean room). Zamknięte — tylko inspiracja. Zawsze własny kod. Zawsze dopasowany do naszego systemu, bez balastu. Nigdy nie implementuj bez świadomego porównania. | INVARIANT | SESJA 2026-06-18 |

---

## II. PROCES (RULES)

### II-A. Ścieżka projektowa

| # | Zasada | Kategoria | Źródło |
|---|--------|-----------|--------|
| II-1 | **CONCEPT → BLUEPRINT → AUDYT → POPRAWKI → IMPLEMENTACJA → TESTY → DEPLOY → COMMIT** — każda zmiana techniczna przechodzi pełną ścieżkę. Żadnych skrótów. | RULE | ARCH-B10 |
| II-2 | **Audyt: 3 rundy, potem stop** — przed implementacją, audytor (człowiek lub AI) sprawdza blueprint. Bez audytu nie ma implementacji. "To tylko mały fix" nie jest wymówką. Zewnętrzne audyty: maksymalnie 3 rundy. Po 3 rundzie Echo sprawdza overengineering — sugestie wykraczające poza zakres blueprintu są odrzucane. Cel: znaleźć dziury, nie dopieszczać w nieskończoność. | RULE | ARCH-B10 |
| II-3 | **Klocki razem gdy zależne** — komponenty zależne od siebie implementowane razem (jeden commit, jeden deploy). Niezależne — osobno. Decyzję o podziale podejmuje operator. | RULE | ARCH-B14, A3 |
| II-4 | **Radykalne zmiany zamiast łatania** — gdy fundament jest zły — przepisz go. Nie doklejaj. Citadel zastąpił Palace. Kernel zastąpi monolit. Nie boimy się przepisać warstwy od zera gdy to słuszne. | RULE | ARCH-B9 |
| II-5 | **Dokumentacja i commit po każdej zmianie** — nie ma zmiany bez śladu w repo. Nie ma commita bez opisu: co, dlaczego, jak. INDEX.md zawsze aktualny. | RULE | ARCH-B11 |

### II-B. Jakość

| # | Zasada | Kategoria | Źródło |
|---|--------|-----------|--------|
| II-6 | **Rzetelność** — każdy komponent ma jawne invarianty. Każdy invariant ma weryfikację (test). Per-unit panic recovery. Timeout na Start/Stop (5s). Watchdog w runtime (30s). | RULE | ARCH-B5 |
| II-7 | **Kontrakty jawne** — Unit.Dependencies() jawne. Timeout przez context jawny. Observer interfejsy jawne. Nic nie jest "domyślnie". Wszystko nazwane. | RULE | ARCH-B8 |
| II-8 | **Per-unit panic recovery** — panika w jednym unicie NIE zabija Kernela. defer/recover na każdej granicy Unit.Start() i Unit.Stop(). | RULE | ARCH-B5 |
| II-9 | **Watchdog** — Kernel sprawdza Health() każdego aktywnego Unitu co 30s. Degraded ≠ Failed — system informuje, nie restartuje automatycznie. | RULE | ARCH-B5 |
| II-10 | **Cykl zależności → emergency fallback** — jeśli unity mają cykliczną zależność, Kernel przechodzi do emergency.target. Nigdy nie panikuje. | RULE | ARCH-B5 |

### II-C. Organizacja kodu

| # | Zasada | Kategoria | Źródło |
|---|--------|-----------|--------|
| II-11 | **Biblioteki przed frameworkiem + adaptuj zamiast implementuj** — (1) każdy generyczny komponent idzie do sysq-libs jako biblioteka PRZED użyciem w projekcie. (2) Przed implementacją: sprawdź sysq-libs → sprawdź open source → porównaj → adaptuj (I-22) lub napisz własne. Zero duplikacji między repozytoriami. | RULE | ARCH-B4, SESJA 2026-06-18 |
| II-12 | **Protokoły jako osobne repozytoria** — każdy protokół (Cerber, Epistemic ID, PSL) ma własne repo z własną specyfikacją. Biblioteki implementują protokoły. Projekty używają bibliotek. | RULE | ARCH-B6 |
| II-13 | **Spójność architektoniczna** — każdy nowy komponent musi pasować do istniejących warstw (1-12 w ARCHITECTURE.md). Nie narusza invariantów. Nie omija warstw. | RULE | ARCH-B13 |
| II-14 | **Monolit z klocków** — jeden projekt, jedna architektura, jeden język wizualny. Każdy klocek pasuje do kontraktu, działa samodzielnie, ale w zestawie tworzy całość. Można go wymienić bez burzenia reszty. | RULE | A1 |

---

## III. PREZENTACJA I KOMUNIKACJA (RULES)

| # | Zasada | Kategoria | Źródło |
|---|--------|-----------|--------|
| III-1 | **Ramki tylko w lazarusd** — geometria, ASCII art, ramki tylko tam gdzie jest $COLUMNS. LLM (goose) NIGDY nie generuje ramek. Używa Markdown: nagłówki, listy, separatory. | RULE | ARCH-B12, A2 |
| III-2 | **Zero emoji w kodzie, testach, commitach** — jeśli potrzebny akcent wizualny: NerdFont. Fallback: znaki ASCII (→ ✓ ✗ ⚠). Nigdy emoji. | RULE | ARCH-B16 |
| III-3 | **Dokumentacja po angielsku** — kod, specyfikacje, invarianty. Komunikacja z operatorem po polsku. | RULE | (istniejąca) |

### III-D. Ciągłość

| # | Zasada | Kategoria | Źródło |
|---|--------|-----------|--------|
| III-4 | **Kontekst pracy przekazywany między sesjami** — resume_context w echo_state.json. Periodyczny save co 60s. Save przy sygnale (SIGTERM/SIGINT). Save przy presji contextu >80%. Nowa sesja wie co było robione bez pytania operatora. | RULE | ARCH-B15 |
| III-5 | **Resume context ≤ 1000 bajtów** — twardy limit. Atomic write (tmp + fsync + rename). | RULE | ARCH-B15 |

### III-F. Git i tożsamość (NOWE — 2026-06-18, z istniejących conceptów)

| # | Zasada | Kategoria | Źródło |
|---|--------|-----------|--------|
| III-12 | **Wing = Git branch dla tożsamości** — każdy Wing to gałąź tożsamości agenta. Merge system działa jak Pull Request: diff → review → approve → merge. Operator widzi co się zmienia zanim to nastąpi. | RULE | SYSQ_AI_MERGE_SYSTEM |
| III-13 | **Pełna historia merge'y** — każdy merge jest nagrany: kto, kiedy, z czego, diff. Rollback zawsze możliwy. Jak git log, tylko dla pamięci agenta. | RULE | SYSQ_AI_MERGE_SYSTEM |
| III-14 | **Review przed mergem** — żaden merge nie wykonuje się bez review. Operator widzi diff, rozwiązuje konflikty jawnie. Żadnych cichych automatycznych merge'y. | RULE | SYSQ_AI_MERGE_SYSTEM |
| III-15 | **Git jako szyna danych** — docelowo: `git push` = kod + pełna pamięć decyzyjna. CI/CD sprawdza spójność decyzji z kodem. Repo jest source of truth. | GUIDELINE | LAZARUS_JAKO_PLATFORMA |
| III-16 | **SuperWingi jak repo** — operator/vault (prywatne), system/library (wiedza), project-* (projekty czasowe), archive/* (zamrożone). Każdy SuperWing ma jasno określone prawa dostępu. | RULE | SUPERWING_ARCHITECTURE |

| III-17 | **GitVault — warstwa git nad Citadel** — każdy SuperWing ma własne repo git. Dump .sql = tekstowa reprezentacja stanu. Git śledzi tylko .sql, nie .db. Auto-snapshot po sesji. Ręczny snapshot przed niebezpiecznymi operacjami (GC, merge). Podróż w czasie = git checkout. Eksperymenty = git branch. Klonowanie agenta = git clone + init-citadel. | RULE | GITVAULT |
| III-18 | **Git nie zastępuje Citadel** — git to okno w czasie, Citadel to źródło prawdy. Git diff działa na INSERT-ach. Git nie rozwiązuje konfliktów pamięci (merge.Merge()). Git nie ma FTS5. Git nie pilnuje owner==CurrentSelf. | INVARIANT | GITVAULT |
| III-19 | **Snapshot przed operacjami destrukcyjnymi** — przed GC, przed merge, przed eksperymentem — zawsze git save. Nigdy nie działamy bez możliwości powrotu. | RULE | GITVAULT |

Ta konstytucja obowiązuje we wszystkich repozytoriach SysQ:

| Repozytorium | Typ | Status |
|-------------|------|--------|
| `lazarus` | Aplikacja | Aktywny — Kernel Refactor w toku |
| `sysq-libs` | Biblioteki | Aktywny — ekstrakcja z Lazarusa |
| `sysq-supervizor` | Aplikacja | Aktywny |
| `sysqcli` | Narzędzia CLI | Aktywny |
| `cerber` | Protokół / specyfikacja | Aktywny |
| `epistemic-id` | Protokół / specyfikacja | Aktywny |
| `psl` | Protokół + implementacja | Aktywny |
| `narada` | Aplikacja | W planach |
| `multi-agent-mempalace` | Research | W planach |
| `Citadel` | Legacy | Maintenance |
| `chambers` | Legacy | Zastąpiony przez Lazarusa |
| `chambers-community` | Legacy | Zastąpiony przez Lazarusa |

---

## V. KONSEKWENCJE ZŁAMANIA

| Kategoria | Konsekwencja |
|-----------|-------------|
| **INVARIANT** złamany | System nie przechodzi testów. Deploy zablokowany. Wymagana decyzja operatora o zmianie invariantu. |
| **RULE** złamana | Wymagany `docs/decisions/DECISION_override_*.md` z uzasadnieniem. Bez niego — PR odrzucony. |
| **GUIDELINE** złamana | Wymagane uzasadnienie w commit message. |

---

*Echo & SysQ — 2026-06-18*
*Konstytucja v2.4. 21 invariantów, 31 rules, 7 guidelines.*
*Rzetelność. Obowiązuje od zaraz.*

### III-E. Zaufanie (NOWE — 2026-06-18)

| # | Zasada | Kategoria | Źródło |
|---|--------|-----------|--------|
| III-6 | **Protokoły otwarte** — każdy protokół (Cerber, PSL, Epistemic ID, MACP) ma publiczną specyfikację. Klient może zweryfikować logikę bez dostępu do implementacji. | RULE | CONCEPT_TRUST_ARCHITECTURE |
| III-7 | **Reproducible Build** — każda binarka ma deterministyczny hash. Klient buduje sam i porównuje. | RULE | CONCEPT_TRUST_ARCHITECTURE |
| III-8 | **Audyt zewnętrzny** — przed wydaniem klientowi enterprise, kod przechodzi audyt przez niezależną firmę. Raport z pieczątką, nie opinia developera. | RULE | CONCEPT_TRUST_ARCHITECTURE |
| III-9 | **Izolacja agenta** — agent działa w sandboxie. AlwaysAllow/RequireApproval. Citadel owner check. Dane nie wychodzą poza maszynę. | RULE | CONCEPT_TRUST_ARCHITECTURE |
| III-10 | **Ślad każdej zmiany** — git log + SBOM + signed commits. Każdy deploy weryfikowalny. | RULE | CONCEPT_TRUST_ARCHITECTURE |
| III-11 | **Model hybrydowy** — protokoły i biblioteki OTWARTE (MIT). Core SOURCE AVAILABLE (NDA). Enterprise features ZAMKNIĘTE (komercyjne). | RULE | CONCEPT_TRUST_ARCHITECTURE |

### III-H. Dokumentacja i wersjonowanie (NOWE — 2026-06-18)

| # | Zasada | Kategoria | Źródło |
|---|--------|-----------|--------|
| III-12 | **Każdy blueprint trafia do ROADMAP** — po utworzeniu nowego blueprintu, musi być dopisany do `docs/blueprints/ROADMAP.md` ze statusem (WDROŻONE/BLUEPRINT/KONCEPT/AUDYT). | RULE | SESJA 2026-06-18 |
| III-13 | **Każdy concept trafia do ROADMAP** — po utworzeniu nowego conceptu, musi być dopisany do `docs/concepts/ROADMAP.md` ze scorem (przydatność/gotowość/koszt) i statusem. | RULE | SESJA 2026-06-18 |
| III-14 | **Dokumentacja po każdej sesji** — INDEX.md i ROADMAPy muszą być zaktualizowane po każdej sesji, w której powstały nowe blueprinty/concepty/fixy. | RULE | SESJA 2026-06-18 |
| III-15 | **Git tagi dla każdego repozytorium** — każdy moduł Go ma wersjonowanie przez git tagi (semantyka). `lazarus` = tag wersji Lazarusa (vX.Y.Z). `sysq-libs` = tag wersji bibliotek (vX.Y.Z). Tag jest dodawany po zakończonym milestonie. | RULE | SESJA 2026-06-18 |
| III-16 | **Wersja w go.mod** — zależności między modułami (lazarus → sysq-libs) używają konkretnych tagów (vX.Y.Z), nie pseudo-wersji (v0.0.0). | RULE | SESJA 2026-06-18 |
| III-17 | **Dokumenty mają wersje** — każdy dokument (blueprint, concept, konstytucja, milestone, fix) ma numer wersji w nazwie pliku (vX.Y). Wersje paper trail — nigdy nie nadpisujemy starej. | RULE | SESJA 2026-06-18 |
| III-18 | **CONSTITUTION zsynchronizowana we wszystkich repo** — każde repozytorium SysQ (12 targetów) ma aktualną kopię CONSTITUTION.md w `docs/`. Aktualizacja jest częścią ścieżki CONCEPT→COMMIT — po zmianie w kanonicznym `lazarus/docs/CONSTITUTION.md` synchronizowane do pozostałych 11 repo. | RULE | SESJA 2026-06-18 |
| III-19 | **Kaskadowa aktualizacja repo** — zmiana w jednym repozytorium, która wpływa na inne (np. nowe narzędzie MCP w lazarus → update specyfikacji w cerber; nowe pole w Entry → update w epistemic-id; zmiana API PSL → update w psl), wymaga aktualizacji wszystkich dotkniętych repozytoriów. Nie ma "to tylko w Lazarus" — każde repo odzwierciedla aktualny stan swojego wycinka systemu. | RULE | SESJA 2026-06-18 |
| III-20 | **Stan repo po każdej sesji** — po zakończonej sesji wszystkie zmodyfikowane repozytoria są zsynchronizowane z GitHub. Nie ma "lokalnych draftów" bez commita. Każde repo z osobnym commitem opisującym co zmieniono i dlaczego. | RULE | SESJA 2026-06-18 |
| III-21 | **Prior art w każdym blueprint/concept** — każdy dokument projektowy (blueprint, concept) zawiera sekcję "Prior art" — jakie istniejące rozwiązania sprawdzono, które zaadaptowano i dlaczego, które odrzucono i dlaczego. Decyzja świadoma, nie domyślna. | RULE | SESJA 2026-06-18 |
| III-22 | **Prior art — kryteria odrzucenia** — odrzucenie istniejącego rozwiązania wymaga podania konkretnego powodu: licencja (GPL/AGPL), nadmiar funkcji (>10x potrzebnego scope), martwy projekt (>12 miesięcy bez commitów), niedopasowanie architektoniczne (nie pasuje do invariantów Lazarus/SysQ). "Nie podoba mi się" nie jest powodem. | RULE | SESJA 2026-06-18 |
