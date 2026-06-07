# sysq-libs — Roadmap

## v0.1 — Bootstrap (2026-06-08)

Ekstrakcja istniejącego kodu z chambers do bibliotek.

| Pakiet | Co | Skąd |
|--------|-----|------|
| `audit` | JSONL audit logger z interfejsem | `chambers/audit.go` (13 linii + wrapper) |
| `health` | Health checker z eskalacją | `chambers/main.go` runHealth() (72 linii) |
| `manifest` | Identity Manifest loader/validator | `chambers/main.go` validateIdentityManifest() (84 linii) |
| `vault` | ChaCha20 encrypt/decrypt | `chambers/vault.go` (326 linii) |
| `honeypot` | Detector interface | chambers vault honeypot + supervisor sentinel concept |

**Cel:** 5 pakietów, 0 nowego kodu — tylko ekstrakcja i uogólnienie.
**Szacunek:** ~600 linii Go.

---

## v0.2 — Nowe fundamenty (2026-06-09/10)

| Pakiet | Co | Źródło |
|--------|-----|--------|
| `acl` | RBAC engine (Casbin wrapper) | Nowe |
| `store` | BoltDB wrapper (CRUD, WAL, migration) | Nowe |
| `bus` | InMemoryBus (pub/sub, ACK/NACK) | RESEARCH.md + nowe |
| `recovery` | Backup/restore + 🔶🟡🔴 protokół | chambers-backup.sh → Go |

**Cel:** 4 nowe pakiety. Zero zależności zewnętrznych poza Casbin + BoltDB.
**Szacunek:** ~800 linii Go.

---

## v0.3 — Usuwanie duplikacji (2026-06-11+)

**Chambers:** wymiana własnych implementacji na importy z sysq-libs.
- `audit.go` → `sysq-libs/pkg/audit`
- `runHealth()` → `sysq-libs/pkg/health`
- `validateIdentityManifest()` → `sysq-libs/pkg/manifest`
- `vault.go` → `sysq-libs/pkg/vault`

**SysQ Supervisor:** start używania sysq-libs od początku.
- `audit` → gotowe
- `health` → gotowe
- `acl` → gotowe (Casbin)
- `store` → gotowe (BoltDB)
- `bus` → gotowe

**Citadel Go:** port z bash → używa sysq-libs dla audytu, recovery, health.

---

## v1.0 — Stabilizacja

- ✅ Wszystkie 9 pakietów ma testy (≥3 na pakiet)
- ✅ Chambers używa sysq-libs zamiast własnych implementacji
- ✅ Supervisor używa sysq-libs
- ✅ API stabilne — `go get github.com/QguAr71/sysq-libs@v1.0.0`
- ✅ Go doc kompletne
- ✅ CI/CD (GitHub Actions: test, lint, vet)
