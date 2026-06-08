# sysq-libs — Dokumentacja pakietów

## Projektowanie

Każdy pakiet:
- **1 cel** — jeden dobrze zdefiniowany problem
- **1 interface** — `type Interface interface { ... }`
- **≥1 implementacja** — domyślna (stdlib) + opcjonalne zaawansowane
- **≥3 testy** — happy path, edge case, error
- **0 zależności zewnętrznych** — wyjątek: Casbin (`acl`), BoltDB (`store`)
- **Go doc** — każda eksportowana funkcja ma komentarz `// NazwaFunkcji robi X.`

---

## audyt

JSONL audit logger. Symetryczny format dla wszystkich projektów.

```go
package audit

type Interface interface {
    Log(eventType string, details map[string]interface{}) error
}

// Event types (konwencja):
// DAEMON_STARTED, UPSTREAM_DIED, HEARTBEAT_FAILED
// HONEYPOT_TRIPPED, MIRRORED, DECRYPTED
// ALLOWED, DENIED
```

**Przykład:**
```json
{"ts":"2026-06-07T23:00:00Z","type":"HONEYPOT_TRIPPED","agent":"gemini-cli","wing":"mem-goose","room":"api-keys"}
```

**Implementacje:**
- `FileAuditor` — zapisuje do pliku JSONL
- `NoopAuditor` — /dev/null (do testów)

---

## health

Health checker z eskalacją. Zwraca kod wyjścia 0/1 + szczegóły.

```go
package health

type Interface interface {
    Check() (Status, error)
}

type Status struct {
    Healthy   bool
    Component string
    Detail    string
    Since     time.Time
}
```

**Eskalacja:**
- 1 fail → HEALTHY z warningiem
- 3 consecutive fails → DEGRADED
- 10 consecutive fails → DEAD

**Implementacje:**
- `SocketHealth` — sprawdza czy Unix socket odpowiada
- `ProcessHealth` — sprawdza czy PID żyje
- `CompositeHealth` — agregacja wielu checkerów

---

## manifest

Identity Manifest loader/validator wg specyfikacji v1.0.

```go
package manifest

type Interface interface {
    Validate(path string) (*Manifest, error)
}

type Manifest struct {
    Agent    AgentInfo    `yaml:"agent"`
    Identity IdentityInfo `yaml:"identity"`
    Recovery RecoveryInfo `yaml:"recovery"`
}

type AgentInfo struct {
    Name    string `yaml:"name"`
    Version int    `yaml:"version"`
}

type IdentityInfo struct {
    CoreDirectives []string `yaml:"core_directives"`
    Language       string   `yaml:"language"`
    Style          string   `yaml:"style"`
    Philosophy     string   `yaml:"philosophy"`
}

type RecoveryInfo struct {
    MinRequiredFiles []string `yaml:"min_required_files"`
    RebuildCommand   string   `yaml:"rebuild_command"`
    BackupPaths      []string `yaml:"backup_paths"`
}
```

**Reguły walidacji:**
1. `agent.name` — wymagane, niepuste
2. `agent.version` — wymagane, ≥1
3. `identity.core_directives` — minimum 3
4. `recovery.min_required_files` — minimum 1
5. Plik musi być poprawnym YAML

**Implementacje:**
- `FileValidator` — ładuje z pliku, waliduje reguły

---

## vault

ChaCha20-Poly1305 encrypt/decrypt z stdlib. Master key z pliku.

```go
package vault

type Interface interface {
    Encrypt(plaintext []byte) (ciphertext []byte, err error)
    Decrypt(ciphertext []byte) (plaintext []byte, err error)
    DeriveKey(plaintext, salt []byte) []byte
}

type Manager struct {
    key [32]byte
}
```

**Metody:**
- `NewVaultManager(masterKeyPath string)` — ładuje klucz z pliku
- `GenerateMasterKey()` — generuje losowy klucz 32B hex
- `Encrypt/Decrypt` — ChaCha20-Poly1305, losowy nonce

**Użycie:**
```go
v, _ := vault.NewVaultManager("~/.config/sysq/master.key")
encrypted, _ := v.Encrypt([]byte("sekret"))
plain, _ := v.Decrypt(encrypted)
// plain == []byte("sekret")
```

---

## acl

RBAC engine oparty na Casbin. Wrapper upraszczający API do potrzeb SysQ.

```go
package acl

type Interface interface {
    Allow(subject, object, action string) (bool, error)
}

type Engine struct {
    enforcer *casbin.Enforcer
}
```

**Model RBAC:**
```
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && r.obj == p.obj && r.act == p.act
```

**Dla chambers:** model rozszerzony o wing-based policy:
```
p = sub, wing, room, act
m = r.sub == p.sub && (r.wing == p.wing || p.wing == "shared") && r.act == p.act
```

**Implementacje:**
- `CasbinEngine` — pełny RBAC
- `StaticPolicy` — prosta mapa (MVP, gdy Casbin za duży)

---

## store

BoltDB wrapper. CRUD, WAL, migracja.

```go
package store

type Interface interface {
    Get(key []byte) ([]byte, error)
    Put(key, value []byte) error
    Delete(key []byte) error
    List(prefix []byte) ([][]byte, error)
}

type Store struct {
    db *bbolt.DB
}
```

**Metody:**
- `New(path string)` — otwiera/zakłada bazę
- `Close()` — zamyka
- `Backup(path string)` — snapshot do pliku
- `Restore(path string)` — odtworzenie z pliku

**Buckety:**
- `state/` — Desired State
- `observed/` — Observed State
- `audit/` — Audituj logi (kopia w BoltDB)
- `manifest/` — Manifesty agentów

---

## bus

InMemoryBus z pub/sub. MVP — goroutine + channels.

```go
package bus

type Interface interface {
    Subscribe(topic string) (<-chan Message, error)
    Publish(topic string, msg Message) error
    Request(topic string, msg Message, timeout time.Duration) (Message, error)
}

type Message struct {
    ID            string
    CorrelationID string
    Timestamp     time.Time
    Source        string
    Topic         string
    Type          string // event, command, query, response
    Priority      int    // 0-4
    TTL           time.Duration
    Payload       []byte
    Meta          map[string]interface{}
}
```

**Implementacje:**
- `InMemoryBus` — goroutine + channels (MVP, ~200 linii)
- `NATSBus` — NATS/JetStream backend (produkcja, Faza 2+)

**Zachowanie:**
- Subskrybenci dostają kopię wiadomości (fan-out)
- `correlation_id` łączy request z response
- NACK wspierany — publisher dostaje zwrotkę gdy brak subów
- TTL — wiadomości starsze niż TTL są ignorowane

---

## recovery

Backup/restore z protokołem weryfikacji 🔶🟡🔴.

```go
package recovery

type Interface interface {
    Backup(paths []string, dest string) error
    Restore(src, dest string) error
    Verify(src string, paths []string) (*VerifyResult, error)
}

type VerifyResult struct {
    Total   int
    Present int
    Missing []string
}
```

**Protokół testowy:**
- 🔶 Dry-run — sprawdź czy backup istnieje i zawiera krytyczne pliki
- 🟡 Restore do /tmp — odtwórz i zweryfikuj strukturę
- 🔴 Full — usuń, odtwórz, zweryfikuj

**Implementacje:**
- `TarRecovery` — tar.gz backup/restore
- `BoltRecovery` — BoltDB snapshot backup/restore

---

## honeypot

Detector interface. Wspólny interfejs dla różnych typów pułapek.

```go
package honeypot

type Interface interface {
    Check(access AccessInfo) *Alert
}

type AccessInfo struct {
    Subject string // agent name or IP
    Target  string // drawer room or TCP port
    Action  string // "read", "write", "connect"
}

type Alert struct {
    Triggered bool
    Subject   string
    Target    string
    Action    string
    Severity  string // "low", "medium", "high"
    Timestamp time.Time
}
```

**Implementacje:**
- `DrawerHoneypot` — drawer-pułapka (chambers vault)
- `TCPHoneypot` — TCP port trap (supervisor sentinel)
- `CompositeHoneypot` — wiele pułapek w jednym

**Przykład (chambers):**
```go
hp := honeypot.DrawerHoneypot{Room: "api-keys"}
alert := hp.Check(honeypot.AccessInfo{
    Subject: "gemini-cli",
    Target:  "api-keys",
    Action:  "read",
})
// alert.Triggered == true → audit.Log("HONEYPOT_TRIPPED", ...)
```

---

## Konwencje nazewnicze

| Konwencja | Przykład |
|-----------|---------|
| **Pakiety** | lowercase, pojedyncze słowo: `audit`, `health`, `vault` |
| **Interfejsy** | `Interface` — jeden per pakiet jako domyślny kontrakt |
| **Implementacje** | `NazwaInterface` + `NazwaImplementacji`: `FileAuditor`, `SocketHealth` |
| **Testy** | `*_test.go` w tym samym pakiecie |
| **Błędy** | Zawsze wrapping: `fmt.Errorf("audit: %w", err)` |
| **Kontekst** | Każda funkcja I/O przyjmuje `context.Context` jako pierwszy argument |

---

## Dla integratorów

Żeby użyć sysq-libs w projekcie:

```bash
go get github.com/QguAr71/sysq-libs@v0.1.0
```

```go
import "github.com/QguAr71/sysq-libs/pkg/audit"
import "github.com/QguAr71/sysq-libs/pkg/vault"

auditor := audit.NewFileAudit("/var/log/sysq/audit.jsonl")
vaultMgr, _ := vault.NewVaultManager("/etc/sysq/master.key")

auditor.Log("DAEMON_STARTED", map[string]interface{}{"version": "1.0"})
encrypted, _ := vaultMgr.Encrypt([]byte("sekret"))
```

**Minimum zależności projektu:**
```go.mod
require github.com/QguAr71/sysq-libs v0.1.0
require github.com/casbin/casbin/v2 v2.x // tylko jeśli używasz acl
require go.etcd.io/bbolt v1.x            // tylko jeśli używasz store
```

---

## Polityka licencyjna (obowiązuje od 2026-06-08)

1. **Zawsze sprawdzaj licencję** — przed każdą integracją zewnętrznego kodu, koncepcji lub algorytmu. Brak licencji = tylko koncept, nie kod.
2. **Koncepcje tak, kod nie** — własna implementacja w Go/Rust, zero kopiowania fragmentów z cudzych repo.
3. **Apache 2.0 → BSD 3-Clause** — Apache wymaga NOTICE przy kopiowaniu kodu, nie przy niezależnej implementacji idei. BSD 3-Clause to nasz standard dla wszystkich bibliotek sysq.
4. **Teorie i algorytmy z domeny publicznej** (Ebbinghaus 1885, FSRS MIT) — wolne do wykorzystania bez ograniczeń.
5. **Przed integracją: audit licencji** — każdy nowy plik/import sprawdzony pod kątem kompatybilności z BSD 3-Clause.
6. **GPL/AGPL — zabronione** — sysq-libs nigdy nie będzie zawierać kodu z licencją wirusową.

Zobacz: [BLUEPRINT_NEUROADAPTIVE.md](https://github.com/QguAr71/chambers/blob/main/BLUEPRINT_NEUROADAPTIVE.md) — szczegółowy audit licencji zewnętrznych forków.
