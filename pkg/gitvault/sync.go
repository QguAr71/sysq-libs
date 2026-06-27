// Package gitvault — sync, diff, clone
//
// CONSTITUTION: III-18, W3
package gitvault

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ─────────────────────────────────────────────────
// Sync — push/pull
// ─────────────────────────────────────────────────

// Push wysyła snapshoty do remote.
func (r *Repo) Push(remote, branch string) error {
	cmd := exec.Command("git", "push", remote, branch)
	cmd.Dir = r.path
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gitvault: push failed: %w\n%s", err, string(out))
	}
	return nil
}

// Pull pobiera snapshoty z remote.
func (r *Repo) Pull(remote, branch string) error {
	cmd := exec.Command("git", "pull", remote, branch)
	cmd.Dir = r.path
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gitvault: pull failed: %w\n%s", err, string(out))
	}
	return nil
}

// ─────────────────────────────────────────────────
// Diff — różnice między snapshotami
// ─────────────────────────────────────────────────

// Diff zwraca diff między dwoma commitami.
func (r *Repo) Diff(fromCommit, toCommit string) (string, error) {
	args := []string{"diff"}
	if fromCommit != "" && toCommit != "" {
		args = append(args, fromCommit+".."+toCommit)
	}
	args = append(args, "--", "dump.sql")

	cmd := exec.Command("git", args...)
	cmd.Dir = r.path
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gitvault: diff failed: %w", err)
	}
	return string(out), nil
}

// Log zwraca ostatnie N commitów.
func (r *Repo) Log(n int) (string, error) {
	cmd := exec.Command("git", "log", fmt.Sprintf("-%d", n), "--oneline", "--", "dump.sql")
	cmd.Dir = r.path
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gitvault: log failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Status zwraca status git.
func (r *Repo) Status() (string, error) {
	cmd := exec.Command("git", "status", "--short")
	cmd.Dir = r.path
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gitvault: status failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ─────────────────────────────────────────────────
// Clone — klonowanie z remote
// ─────────────────────────────────────────────────

// Clone klonuje repozytorium Citadel do .lazarus/.
func Clone(remoteURL, lazarusDir string) error {
	if _, err := os.Stat(lazarusDir); os.IsNotExist(err) {
		if err := os.MkdirAll(lazarusDir, 0755); err != nil {
			return fmt.Errorf("gitvault: cannot create %s: %w", lazarusDir, err)
		}
	}

	cmd := exec.Command("git", "clone", remoteURL, lazarusDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gitvault: clone failed: %w\n%s", err, string(out))
	}
	return nil
}

// ─────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────

// IsRepo sprawdza czy ścieżka ma .git.
func IsRepo(path string) bool {
	gitDir := filepath.Join(path, ".git")
	_, err := os.Stat(gitDir)
	return err == nil
}

// gitAdd dodaje plik do staging.
func (r *Repo) gitAdd(path string) error {
	relPath, err := filepath.Rel(r.path, path)
	if err != nil {
		relPath = path
	}
	cmd := exec.Command("git", "add", relPath)
	cmd.Dir = r.path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(out))
	}
	return nil
}


