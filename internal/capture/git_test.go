package capture_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/capture"
	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

func setupTestGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\nOutput: %s", args, err, string(out))
		}
	}

	run("init")
	run("config", "user.name", "Test Runner")
	run("config", "user.email", "test@example.com")
	run("config", "commit.gpgsign", "false")

	// Commit 1: standard chore/feat commit
	file1 := filepath.Join(dir, "README.md")
	if err := os.WriteFile(file1, []byte("# Test Repo\n"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	run("add", "README.md")
	run("commit", "-m", "chore: initialize repository")

	// Commit 2: architectural decision commit
	file2 := filepath.Join(dir, "config.json")
	if err := os.WriteFile(file2, []byte("{\"mode\": \"wal\"}\n"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	run("add", "config.json")
	run("commit", "-m", "refactor: switch SQLite journal mode\n\nDecided to use WAL mode in order to reduce write contention.")

	// Commit 3: dependency modification
	gomod := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(gomod, []byte("module example.com/app\n\ngo 1.22\n\nrequire github.com/mattn/go-sqlite3 v1.14.22\n"), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	run("add", "go.mod")
	run("commit", "-m", "feat: add sqlite driver dependency")

	return dir
}

func setupTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{DBPath: filepath.Join(dir, "test.db")}
	s, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCaptureGit_BasicAndIncremental(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	st := setupTestStore(t)
	ctx := context.Background()

	// Dry run test first
	dryRes, err := capture.CaptureGit(ctx, st, capture.GitCaptureConfig{
		RepoPath: repoDir,
		DryRun:   true,
	})
	if err != nil {
		t.Fatalf("capture git dry run: %v", err)
	}
	if dryRes.Scanned != 3 {
		t.Fatalf("expected 3 commits scanned in dry run, got %d", dryRes.Scanned)
	}

	// Verify cursor was NOT written in dry run
	c, _ := capture.ReadGitCursor(repoDir)
	if c != "" {
		t.Fatalf("cursor should not be written in dry run, got %q", c)
	}

	// Real run
	res, err := capture.CaptureGit(ctx, st, capture.GitCaptureConfig{
		RepoPath: repoDir,
		DryRun:   false,
	})
	if err != nil {
		t.Fatalf("capture git: %v", err)
	}
	if res.Scanned != 3 {
		t.Fatalf("expected 3 commits scanned, got %d", res.Scanned)
	}
	if res.MemoriesCreated < 3 {
		t.Fatalf("expected at least 3 memories created, got %d", res.MemoriesCreated)
	}

	// Verify cursor was written with latest SHA
	cAfter, err := capture.ReadGitCursor(repoDir)
	if err != nil {
		t.Fatalf("read git cursor: %v", err)
	}
	if cAfter == "" {
		t.Fatal("cursor should be written after successful run")
	}

	// Incremental run: nothing new to process
	incRes, err := capture.CaptureGit(ctx, st, capture.GitCaptureConfig{
		RepoPath: repoDir,
		DryRun:   false,
	})
	if err != nil {
		t.Fatalf("incremental capture: %v", err)
	}
	if incRes.Scanned != 0 {
		t.Fatalf("expected 0 scanned in incremental run, got %d", incRes.Scanned)
	}

	// Verify dependency fact stored in sqlite
	fact, err := st.GetFact(ctx, capture.ResolveDefaultScope(repoDir), "deps.github.com/mattn/go-sqlite3", false)
	if err != nil {
		t.Fatalf("get dependency fact: %v", err)
	}
	if fact == nil || fact.Value != "v1.14.22" {
		t.Fatalf("unexpected fact value: %v", fact)
	}
}

func TestCaptureGit_EmptyRepo(t *testing.T) {
	dir := t.TempDir()
	st := setupTestStore(t)
	ctx := context.Background()

	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\nOutput: %s", err, string(out))
	}

	res, err := capture.CaptureGit(ctx, st, capture.GitCaptureConfig{
		RepoPath: dir,
		DryRun:   false,
	})
	if err != nil {
		t.Fatalf("capture git on empty repo: %v", err)
	}
	if !res.OK || res.Scanned != 0 || len(res.Items) != 0 {
		t.Fatalf("expected 0 scanned on empty repo, got %+v", res)
	}
}

func TestCaptureGit_RootCommitDependencies(t *testing.T) {
	dir := t.TempDir()
	st := setupTestStore(t)
	ctx := context.Background()

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\nOutput: %s", args, err, string(out))
		}
	}

	run("init")
	run("config", "user.name", "Test Runner")
	run("config", "user.email", "test@example.com")
	run("config", "commit.gpgsign", "false")

	gomod := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(gomod, []byte("module example.com/rootapp\n\ngo 1.22\n\nrequire github.com/stretchr/testify v1.8.4\n"), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	run("add", "go.mod")
	run("commit", "-m", "chore: initial commit with go.mod")

	res, err := capture.CaptureGit(ctx, st, capture.GitCaptureConfig{
		RepoPath: dir,
		DryRun:   false,
	})
	if err != nil {
		t.Fatalf("capture git on root commit: %v", err)
	}
	if res.Scanned != 1 {
		t.Fatalf("expected 1 commit scanned, got %d", res.Scanned)
	}

	fact, err := st.GetFact(ctx, capture.ResolveDefaultScope(dir), "deps.github.com/stretchr/testify", false)
	if err != nil {
		t.Fatalf("get root commit dependency fact: %v", err)
	}
	if fact == nil || fact.Value != "v1.8.4" {
		t.Fatalf("expected 'v1.8.4', got %v", fact)
	}
}
