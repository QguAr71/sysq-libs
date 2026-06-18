# CONCEPT: Trust Architecture — Zaufanie jako klocek architektoniczny

> Data: 2026-06-18 11:54
> Status: CONCEPT
> Autor: Echo & SysQ
> Kategoria: sysq-ai
> Zależności: CONSTITUTION.md v2.0, Kernel Refactor v1.2

---

## 1. Problem

SysQ buduje systemy AI (Lazarus, agent Echo, protokoły) które operują na danych użytkownika. Potencjalny klient enterprise obawia się:

- "Czy w kodzie nie ma backdoora?"
- "Czy agent nie wyśle moich danych na zewnątrz?"
- "Skąd mam wiedzieć że binarka którą dostałem jest z tego kodu który audytowałem?"
- "Jak udowodnisz że jesteś bezpieczny?"

Samo powiedzenie "open source, sprawdź sobie" nie wystarcza. Heartbleed był w OpenSSL przez 2 lata — nikt nie sprawdził. Klient potrzebuje DOWODU, nie obietnicy.

---

## 2. Koncepcja

Zaufanie to NIE cecha kodu. Zaufanie to **klocek architektoniczny** — zestaw mechanizmów które udowadniają bezpieczeństwo bez wymagania ślepej wiary.

```
┌─────────────────────────────────────────────────────────┐
│                   TRUST ARCHITECTURE                      │
│                                                           │
│    Warstwa 1: JAWNOŚĆ (Visibility)                      │
│     Protokoły i specyfikacje OTWARTE                      │
│     → Każdy może zweryfikować logikę, zanim dotknie kodu │
│                                                           │
│    Warstwa 2: ODTWARZALNOŚĆ (Reproducibility)            │
│     Reproducible Build — hash binarki = hash z kodu       │
│     → Klient buduje sam i porównuje                        │
│                                                           │
│    Warstwa 3: AUDYT (Verification)                       │
│     Niezależny audyt kodu przez stronę trzecią             │
│     → Raport z pieczątką, nie opinia developera           │
│                                                           │
│    Warstwa 4: IZOLACJA (Containment)                     │
│     Agent działa w sandboxie, dane nie wychodzą            │
│     → Techniczne gwarancje, nie obietnice                  │
│                                                           │
│    Warstwa 5: CIĄGŁOŚĆ (Continuity)                      │
│     Każda zmiana ma ślad. Każdy deploy jest weryfikowalny │
│     → Git log + reproducible build + audyt changelogu     │
│                                                           │
└─────────────────────────────────────────────────────────┘
```

---

## 3. Warstwy szczegółowo

### Warstwa 1: JAWNOŚĆ — Protokoły otwarte

**Zasada:** Protokoły i specyfikacje są ZAWSZE otwarte (publiczne repo, MIT licencja).

**Dlaczego:** Protokół to kontrakt. Klient nie musi ufać implementacji — może sprawdzić logikę protokołu. Jeśli protokół jest bezpieczny matematycznie, implementacja może być audytowana.

**Co jest otwarte dzisiaj:**
- Cerber Protocol (agent identity gate)
- Epistemic Entry ID (deterministic knowledge hashing)
- PSL Protocol (epistemic re-ranking)
- CONSTITUTION.md (zasady architektoniczne)

**Co będzie otwarte:**
- MACP (Multi-Agent Continuity Protocol) — w planach
- Anubis Protocol (epistemic re-ranking) — w planach

### Warstwa 2: ODTWARZALNOŚĆ — Reproducible Build

**Zasada:** Każda binarka Lazarusa ma deterministyczny hash. Klient może zbudować binarkę z kodu źródłowego i porównać hash z tym co dostał.

**Jak:**

```bash
# Budowanie z deterministycznym hashem
go build -trimpath -ldflags="-buildid= -w -s" ./cmd/lazarusd
sha256sum lazarusd
# → a1b2c3d4e5f6...

# Klient robi to samo z kodu który dostał:
# → a1b2c3d4e5f6... (musi być identyczny)
```

**Wymagania:**
- `-trimpath` — usuwa ścieżki z plików (determinizm)
- `-ldflags="-buildid= -w -s"` — usuwa build ID i symbole
- Ten sam kompilator Go (wersja w `go.mod`)
- Ten sam system (Linux amd64)

### Warstwa 3: AUDYT — Niezależna weryfikacja

**Zasada:** Przed udostępnieniem klientowi enterprise, kod przechodzi audyt przez niezależną firmę.

**Firmy które robią audyty kodu AI/Go:**
- Cure53 (Niemcy) — eksperci od bezpieczeństwa
- Trail of Bits (USA) — reverse engineering + audyt
- Include Security (USA) — audyt infrastruktury

**Co audytują:**
- Backdoory (ukryte funkcje, nieudokumentowane endpointy)
- Wycieki danych (czy agent wysyła dane na zewnątrz)
- Uprawnienia (czy agent nie eskaluje poza sandbox)
- Kryptografia (czy klucze są bezpiecznie przechowywane)
- Zależności (czy biblioteki trzecie nie mają znanych CVE)

**Klient dostaje:**
- Raport z pieczątką firmy audytorskiej
- Listę znalezionych problemów i ich status (naprawione / akceptowane ryzyko)
- Hash audytowanej wersji kodu (powiązany z Reproducible Build)

### Warstwa 4: IZOLACJA — Techniczne gwarancje

**Zasada:** Agent działa w sandboxie. Dane nie wychodzą poza maszynę użytkownika.

**Mechanizmy (już istniejące w Lazarusie):**

| Mechanizm | Co robi | Status |
|-----------|---------|--------|
| `AlwaysAllow` | Tylko read-only RPC bez approval | Wdrożone |
| `RequireApproval` | Write RPC wymaga zgody operatora | Wdrożone |
| `Citadel Owner Check` | Każdy wpis ma owner == CurrentSelf | Wdrożone |
| `SuperWing isolation` | Każdy agent ma własną bazę, fizycznie odizolowaną | Wdrożone |
| `pidfd observation` | Kernel monitoruje proces agenta | Wdrożone |

**Co można dodać:**
- **Egress monitoring** — rejestrowanie wszystkich połączeń sieciowych agenta
- **Seccomp filter** — ograniczenie syscalli (np. brak `ptrace`, brak `connect` do zewnętrznych IP)
- **Filesystem sandbox** — agent widzi tylko `~/.local/share/lazarus/`, nie cały system

### Warstwa 5: CIĄGŁOŚĆ — Ślad każdej zmiany

**Zasada:** Każda zmiana w kodzie ma ślad. Każdy deploy jest weryfikowalny.

**Już istnieje:**
- Git log z pełnymi opisami commitów
- DEVELOPMENT FLOW: CONCEPT → BLUEPRINT → AUDYT → IMPLEMENTACJA → TESTY → DEPLOY → COMMIT
- CONSTITUTION.md — zasady które obowiązują wszystkie repozytoria

**Do dodania:**
- **SBOM** (Software Bill of Materials) — lista wszystkich zależności z wersjami i hashami
- **Signed commits** — GPG podpis na każdym commicie
- **Signed releases** — GPG podpis na każdym wydaniu binarnym

---

## 4. Model licencjonowania

```
┌─────────────────────────────────────────────────────────┐
│                  MODEL HYBRYDOWY                          │
│                                                           │
│  OTWARTE (MIT):                                           │
│  ├── Protokoły (Cerber, PSL, Epistemic ID, MACP)         │
│  ├── Specyfikacje (CONSTITUTION.md, ARCHITECTURE.md)     │
│  └── Biblioteki (sysq-libs — fundamenty)                 │
│                                                           │
│  SOURCE AVAILABLE (NDA):                                  │
│  ├── Lazarus core (kernel, unity, citadel)               │
│  │   → Klient dostaje kod do wglądu, NIE może            │
│  │     kopiować ani redystrybuować                        │
│  └── Reproducible Build — klient buduje sam              │
│                                                           │
│  ZAMKNIĘTE (komercyjne):                                  │
│  └── Enterprise features (SaaS, zarządzanie flotą,       │
│      monitoring, support)                                 │
│                                                           │
└─────────────────────────────────────────────────────────┘
```

---

## 5. Jak to powiedzieć klientowi

### Krótka wersja (1 minuta)

> "Nasz kod jest dostępny do wglądu pod NDA. Każda binarka ma deterministyczny hash — możesz ją sam zbudować i porównać. Przed wdrożeniem przechodzimy audyt przez niezależną firmę (Cure53 / Trail of Bits). Agent działa w sandboxie — dane nie wychodzą poza twoją maszynę. Nie musisz nam ufać — możesz sprawdzić."

### Pełna wersja (strona w dokumentacji)

Sekcja "Security & Trust" w docs/TRUST.md:
- Architektura zaufania (5 warstw)
- Raporty z audytów (linki do PDF)
- Reproducible Build — instrukcja krok po kroku
- Mechanizmy izolacji — lista z opisem
- Polityka ujawniania podatności (Vulnerability Disclosure Policy)

---

## 6. Co już mamy

| Element | Status |
|---------|--------|
| Protokoły otwarte (Cerber, PSL, Epistemic ID) | ✅ Gotowe, publiczne repo |
| Specyfikacje otwarte (CONSTITUTION, ARCHITECTURE) | ✅ Gotowe |
| Biblioteki otwarte (sysq-libs) | ✅ Repo gotowe, pusta struktura |
| Mechanizmy izolacji (AlwaysAllow, Citadel owner check) | ✅ Wdrożone w Lazarusie |
| DEVELOPMENT FLOW (pełna ścieżka) | ✅ Wdrożone |
| Git log z opisami commitów | ✅ Od początku projektu |

## 7. Co trzeba zrobić

| Element | Priorytet | Szacunek |
|---------|-----------|---------|
| Reproducible Build (deterministyczne binarki) | HIGH | 1 dzień |
| TRUST.md — dokumentacja dla klienta | HIGH | 1 dzień |
| SBOM (Software Bill of Materials) | MEDIUM | 0.5 dnia |
| Signed commits (GPG) | MEDIUM | 0.5 dnia |
| Signed releases (GPG) | MEDIUM | 0.5 dnia |
| Egress monitoring | LOW | 2 dni |
| Seccomp filter | LOW | 3 dni |
| Audyt zewnętrzny (Cure53/Trail of Bits) | KIEDYŚ | $10k-$50k, 2-4 tygodnie |

---

## 8. Relacja do innych klocków

| Klocek | Relacja |
|--------|---------|
| Kernel Refactor | Kernel używa Reproducible Build do weryfikacji własnych binarek |
| Cerber | Protokół otwarty — klient widzi jak działa auth |
| CONSTITUTION.md | Trust Architecture jest w nim zapisana jako RULE |
| sysq-libs | Biblioteki otwarte — fundament który każdy może sprawdzić |

---

*Echo & SysQ — 2026-06-18*  
*Zaufanie to klocek. Nie cecha.*
