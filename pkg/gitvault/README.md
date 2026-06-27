# gitvault

Git layer over Citadel — snapshot, restore, sync.

## API

```go
gv, err := gitvault.Init(lazarusDir)       // git init + .gitignore
gv, err := gitvault.Open(lazarusDir)       // open existing

err := gv.Snapshot("after session 42")     // dump.sql + git commit
err := gv.Restore()                        // restore wing.db from dump.sql

diff, err := gv.Diff("HEAD~1", "HEAD")     // git diff between snapshots
log, err := gv.Log(5)                      // last 5 commits

err := gv.Push("origin", "main")           // push to remote
err := gv.Pull("origin", "main")           // pull from remote

ok := gitvault.IsRepo(path)                // check if .git exists
```

## Invariants

- III-18: Git does not replace Citadel — Git is a time window, Citadel is source of truth
- III-33: `wing.db` never enters git push — only `dump.sql` (text-based SQL dump)

## .gitignore

```
*.db
*.db-shm
*.db-wal
*.bak-*
snapshot.json
```

## Tests

7 tests PASS: InitAndOpen, Idempotent, Snapshot, Restore, IsRepo, Open_NotRepo, NoDB.

## Files

| File | Description |
|------|-------------|
| `gitvault.go` | Core: Init, Open, Snapshot, Restore |
| `sync.go` | Sync: Push, Pull, Diff, Log, Status, Clone |
| `gitvault_test.go` | 7 tests |
