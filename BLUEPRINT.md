# sysq-libs — Wspólne biblioteki Go dla projektów SysQ

> **Licencja:** BSD 3-Clause  
> **Status:** v0.1 — blueprint, roadmap, dokumentacja  
> **Data:** 2026-06-07  
> **Repo:** prywatne

---

## Po co to istnieje

SysQ Supervisor, Chambers i Citadel (bash → Go) dzielą te same fundamenty:
audyt, ACL, health check, backup, manifesty. Zamiast duplikować kod w każdym
repo — jedna biblioteka. Wspólna.

```
sysq-libs
├── chambers   (MCP proxy, vault, memory)
├── supervisor (control plane, DNS, firewall)
├── citadel    (DNS stack, privacy — port z bash)
└── przyszłe... (IoT, media, home automation)
```

---

## Architektura

```
pkg/
├── audit/          ← JSONL audit logger (symetryczny, timestamped)
├── health/         ← Health checker z eskalacją
├── manifest/       ← Identity Manifest loader/validator (spec v1.0)
├── vault/          ← ChaCha20-Poly1305 encrypt/decrypt
├── acl/            ← RBAC engine (Casbin wrapper)
├── store/          ← BoltDB wrapper (CRUD, WAL, migration)
├── bus/            ← InMemoryBus (pub/sub, ACK/NACK)
├── recovery/       ← Backup/restore z protokołem weryfikacji 🔶🟡🔴
└── honeypot/       ← Detector interface
```

---

## Zasady

1. **Zero zależności zewnętrznych** poza stdlib + Casbin + BoltDB
2. **Interfejs > implementacja** — każdy pakiet ma `Interface` i co najmniej jedną implementację
3. **Backward compatibility** — `v0.x` może łamać API. `v1.0+` nie.
4. **Jeden pakiet = jeden cel** — jeśli komponent robi dwie rzeczy, rozdzielamy
5. **Testy** — każdy pakiet ma minimum 3 testy (happy path, edge case, error)
6. **Dokumentacja w kodzie** — Go doc, komentarze na eksportowanych funkcjach

---

## Który projekt używa czego

| Pakiet | Chambers | Supervisor | Citadel Go | Przyszłe |
|--------|----------|------------|------------|----------|
| `audit` | ✅ już (własny) | ✅ plan | 🔶 | ✅ |
| `health` | ✅ już (własny) | ✅ plan | 🔶 | ✅ |
| `manifest` | ✅ już (własny) | ✅ | — | ✅ |
| `vault` | ✅ już (własny) | 🔶 sentinel | — | ✅ |
| `acl` | 🔶 własny→Casbin | ✅ | — | ✅ |
| `store` | 🔶 plan BoltDB | ✅ | — | ✅ |
| `bus` | 🔶 plan | ✅ | — | ✅ |
| `recovery` | ✅ już (własny) | ✅ LKG | 🔶 | ✅ |
| `honeypot` | ✅ vault | ✅ sentinel | ✅ TCP | ✅ |

---

## Migracja z własnych implementacji

| Pakiet | Obecnie w | Migracja |
|--------|-----------|----------|
| `audit` | chambers/audit.go (13 linii) | Wyciąć, zaimportować sysq-libs |
| `health` | chambers/main.go (72 linii) | Wyciąć, zaimportować sysq-libs |
| `manifest` | chambers/main.go (84 linii) | Wyciąć, zaimportować sysq-libs |
| `vault` | chambers/vault.go (326 linii) | Przenieść do sysq-libs, importować |
| `recovery` | chambers-backup.sh (bash) | Przepisać na Go, importować |

---

## Szacunek rozmiaru

| Pakiet | Linii | Źródło |
|--------|-------|--------|
| `audit` | ~80 | Ekstrakcja z chambers |
| `health` | ~100 | Ekstrakcja z chambers |
| `manifest` | ~120 | Ekstrakcja z chambers |
| `vault` | ~200 | Ekstrakcja z chambers |
| `acl` | ~200 | Nowe (Casbin wrapper) |
| `store` | ~150 | Nowe (BoltDB wrapper) |
| `bus` | ~300 | RESEARCH.md + nowe |
| `recovery` | ~150 | chambers-backup + uogólnienie |
| `honeypot` | ~80 | Interface z chambers + supervisor |
| **Razem** | **~1380** | |

~1400 linii Go. 9 pakietów. Zero zależności poza Casbin + BoltDB.

---

## Nie wrzucamy do sysq-libs

- **Domeny** — DNS, firewall, adblock, AI — to zostaje w projektach
- **Konfiguracja** — config.yaml każdego projektu jest osobny
- **Binarki** — cmd/ zostaje w projektach
- **UI** — TUI, web frontend — osobno
