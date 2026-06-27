// Package gitvault — testy
package gitvault

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInitAndOpen(t *testing.T) {
	dir := t.TempDir()

	repo, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if !IsRepo(dir) {
		t.Fatal("dir should be a git repo after Init")
	}

	// Otwórz ponownie
	repo2, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if repo2.path != dir {
		t.Errorf("path = %s, want %s", repo2.path, dir)
	}

	_ = repo
}

func TestInit_Idempotent(t *testing.T) {
	dir := t.TempDir()

	_, err := Init(dir)
	if err != nil {
		t.Fatalf("First Init: %v", err)
	}

	// Drugi Init nie powinien errorować
	_, err = Init(dir)
	if err != nil {
		t.Fatalf("Second Init: %v", err)
	}
}

func TestSnapshot(t *testing.T) {
	dir := t.TempDir()

	// Utwórz wing.db z minimalną strukturą (sqlite3 init)
	dbPath := filepath.Join(dir, "wing.db")
	initDB := exec.Command("sqlite3", dbPath, "CREATE TABLE IF NOT EXISTS test(id INTEGER PRIMARY KEY, content TEXT); INSERT INTO test VALUES(1,'hello');")
	if out, err := initDB.CombinedOutput(); err != nil {
		t.Fatalf("initDB: %v\n%s", err, string(out))
	}

	repo, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := repo.Snapshot("test snapshot"); err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	// Sprawdź czy dump.sql powstał
	dumpPath := filepath.Join(dir, "dump.sql")
	if _, err := os.Stat(dumpPath); os.IsNotExist(err) {
		t.Fatal("dump.sql not created after snapshot")
	}

	// Sprawdź czy jest commit
	log, err := repo.Log(1)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if log == "" {
		t.Error("no commits after snapshot")
	}

	// Sprawdź git diff (dump.sql to tekst SQL — diff powinien działać)
	diff, err := repo.Diff("HEAD~1", "HEAD")
	if err != nil {
		t.Logf("Diff warning (may be first commit): %v", err)
	}
	if diff != "" {
		t.Logf("Diff after snapshot:\n%s", diff)
	}
}

func TestSnapshot_NoDB(t *testing.T) {
	dir := t.TempDir()

	repo, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	err = repo.Snapshot("test")
	if err == nil {
		t.Error("Snapshot should fail without wing.db")
	}
}

func TestRestore(t *testing.T) {
	dir := t.TempDir()

	// Utwórz wing.db z tabelą testową
	dbPath := filepath.Join(dir, "wing.db")
	createDB := exec.Command("sqlite3", dbPath, "CREATE TABLE test (id INTEGER); INSERT INTO test VALUES (1);")
	if out, err := createDB.CombinedOutput(); err != nil {
		t.Fatalf("create test db: %v\n%s", err, string(out))
	}

	repo, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := repo.Snapshot("before"); err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	// Usuń wing.db
	os.Remove(dbPath)

	// Restore
	if err := repo.Restore(); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// Sprawdź czy wing.db istnieje
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("wing.db not restored")
	}
}

func TestIsRepo(t *testing.T) {
	dir := t.TempDir()
	if IsRepo(dir) {
		t.Error("empty dir should not be a repo")
	}

	_, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if !IsRepo(dir) {
		t.Error("after Init, should be a repo")
	}
}

func TestOpen_NotRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := Open(dir)
	if err == nil {
		t.Error("Open should fail for non-repo dir")
	}
}
