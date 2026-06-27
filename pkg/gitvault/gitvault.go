// Package gitvault — warstwa git nad Citadel
//
// GitVault integruje gita z Citadelem. Snapshot (backup wing.db + metadata),
// Restore, Sync (push/pull), Diff między snapshotami.
//
// CONSTITUTION: III-18 (Git nie zastępuje Citadel), III-33 (wing.db nie wchodzi do git push)
package gitvault

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ─────────────────────────────────────────────────
// GitVault — core
// ─────────────────────────────────────────────────

// Repo reprezentuje lokalne repozytorium git Citadel (.lazarus/.git).
type Repo struct {
	path         string // ścieżka do .lazarus/
	dbPath       string // wing.db
	snapshotPath string // snapshot.json (metadata)
}

// Open otwiera GitVault z istniejącego .lazarus/.
func Open(lazarusDir string) (*Repo, error) {
	gitDir := filepath.Join(lazarusDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("gitvault: not a git repository: %s", gitDir)
	}

	return &Repo{
		path:         lazarusDir,
		dbPath:       filepath.Join(lazarusDir, "wing.db"),
		snapshotPath: filepath.Join(lazarusDir, "snapshot.json"),
	}, nil
}

// Init inicjalizuje GitVault (git init w .lazarus/).
func Init(lazarusDir string) (*Repo, error) {
	if _, err := os.Stat(lazarusDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("gitvault: .lazarus/ does not exist: %s", lazarusDir)
	}

	gitDir := filepath.Join(lazarusDir, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		// Already init — return existing
		return Open(lazarusDir)
	}

	cmd := exec.Command("git", "init")
	cmd.Dir = lazarusDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("gitvault: git init failed: %w\n%s", err, string(out))
	}

	// Set minimal git config for automated commits (no global config required)
	for _, kv := range [][2]string{
		{"user.email", "gitvault@lazarus.local"},
		{"user.name", "GitVault (Lazarus)"},
	} {
		cfgCmd := exec.Command("git", "config", kv[0], kv[1])
		cfgCmd.Dir = lazarusDir
		cfgCmd.Run() // ignore errors — may already be set
	}

	// .gitignore: track only dump.sql, never wing.db
	gitignore := "*.db\n*.db-shm\n*.db-wal\n*.bak-*\nsnapshot.json\n"
	os.WriteFile(filepath.Join(lazarusDir, ".gitignore"), []byte(gitignore), 0644)

	return &Repo{
		path:         lazarusDir,
		dbPath:       filepath.Join(lazarusDir, "wing.db"),
		snapshotPath: filepath.Join(lazarusDir, "snapshot.json"),
	}, nil
}

// ─────────────────────────────────────────────────
// Snapshot — zrzut stanu Citadel
// ─────────────────────────────────────────────────

// Snapshot wykonuje zrzut wing.db do dump.sql (SQL text) i commit w gicie.
// Formaty SQL pozwala na git diff — widać co dodane między snapshotami.
// III-33: wing.db nie wchodzi do git push. dump.sql jest tekstową reprezentacją.
func (r *Repo) Snapshot(message string) error {
	// Sprawdź czy wing.db istnieje
	if _, err := os.Stat(r.dbPath); os.IsNotExist(err) {
		return fmt.Errorf("gitvault: wing.db not found at %s", r.dbPath)
	}

	// Dump do .sql (tekstowy SQL — git diff działa)
	dumpCmd := exec.Command("sqlite3", r.dbPath, ".dump")
	dumpOut, err := dumpCmd.Output()
	if err != nil {
		return fmt.Errorf("gitvault: sqlite3 dump failed: %w", err)
	}

	dumpPath := filepath.Join(filepath.Dir(r.dbPath), "dump.sql")
	if err := os.WriteFile(dumpPath, dumpOut, 0644); err != nil {
		return fmt.Errorf("gitvault: cannot write dump.sql: %w", err)
	}

	// git add + commit (tylko dump.sql, nie wing.db!)
	if err := r.gitAdd(dumpPath); err != nil {
		return fmt.Errorf("gitvault: git add failed: %w", err)
	}

	if message == "" {
		message = fmt.Sprintf("snapshot %s", time.Now().Format("2006-01-02 15:04:05"))
	}
	if err := r.gitCommit(message); err != nil {
		return fmt.Errorf("gitvault: git commit failed: %w", err)
	}

	return nil
}

// ─────────────────────────────────────────────────
// Restore — odtworzenie stanu Citadel
// ─────────────────────────────────────────────────

// Restore odtwarza wing.db z dump.sql.
func (r *Repo) Restore() error {
	dumpPath := filepath.Join(filepath.Dir(r.dbPath), "dump.sql")
	if _, err := os.Stat(dumpPath); os.IsNotExist(err) {
		return fmt.Errorf("gitvault: dump.sql not found at %s", dumpPath)
	}

	// Utwórz kopię zapasową obecnego wing.db
	backupPath := r.dbPath + ".bak-" + time.Now().Format("20060102_150405")
	if _, err := os.Stat(r.dbPath); err == nil {
		if err := os.Rename(r.dbPath, backupPath); err != nil {
			return fmt.Errorf("gitvault: cannot backup wing.db: %w", err)
		}
	}

	// Odtwórz z .sql
	sqlBytes, err := os.ReadFile(dumpPath)
	if err != nil {
		return fmt.Errorf("gitvault: cannot read dump.sql: %w", err)
	}

	restoreCmd := exec.Command("sqlite3", r.dbPath)
	restoreCmd.Stdin = strings.NewReader(string(sqlBytes))
	if out, err := restoreCmd.CombinedOutput(); err != nil {
		os.Rename(backupPath, r.dbPath)
		return fmt.Errorf("gitvault: sqlite3 restore failed: %w\n%s", err, string(out))
	}

	return nil
}

// gitCommit robi commit.
func (r *Repo) gitCommit(message string) error {
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = r.path
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "nothing to commit") ||
			strings.Contains(string(out), "nothing added to commit") ||
			strings.Contains(string(out), "nie dodano nic do złożenia") {
			return nil
		}
		return fmt.Errorf("%w: %s", err, string(out))
	}
	return nil
}
