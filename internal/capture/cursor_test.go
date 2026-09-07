package capture_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/capture"
)

func TestCursor_GitCursor(t *testing.T) {
	dir := t.TempDir()

	// Initial read with no cursor
	sha, err := capture.ReadGitCursor(dir)
	if err != nil {
		t.Fatalf("read git cursor: %v", err)
	}
	if sha != "" {
		t.Fatalf("expected empty cursor, got %q", sha)
	}

	// Write cursor
	testSHA := "a8f3bc1994d8721c0e352b9921"
	if err := capture.WriteGitCursor(dir, testSHA); err != nil {
		t.Fatalf("write git cursor: %v", err)
	}

	// Verify .gitignore exists
	giPath := filepath.Join(dir, ".centmem", ".gitignore")
	giContent, err := os.ReadFile(giPath)
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if string(giContent) != "*\n" {
		t.Fatalf("unexpected .gitignore content: %q", string(giContent))
	}

	// Read cursor back
	got, err := capture.ReadGitCursor(dir)
	if err != nil {
		t.Fatalf("read git cursor back: %v", err)
	}
	if got != testSHA {
		t.Fatalf("expected %q, got %q", testSHA, got)
	}
}

func TestCursor_DocsCursor(t *testing.T) {
	dir := t.TempDir()

	m, err := capture.ReadDocsCursor(dir)
	if err != nil {
		t.Fatalf("read docs cursor: %v", err)
	}
	if len(m) != 0 {
		t.Fatalf("expected empty map, got %v", m)
	}

	testMap := map[string]int64{
		"docs/intro.md": 1788740000,
		"README.md":     1788741000,
	}
	if err := capture.WriteDocsCursor(dir, testMap); err != nil {
		t.Fatalf("write docs cursor: %v", err)
	}

	got, err := capture.ReadDocsCursor(dir)
	if err != nil {
		t.Fatalf("read docs cursor back: %v", err)
	}
	if len(got) != 2 || got["docs/intro.md"] != 1788740000 || got["README.md"] != 1788741000 {
		t.Fatalf("unexpected docs cursor map: %v", got)
	}
}

func TestCursor_ShellCursor(t *testing.T) {
	dir := t.TempDir()

	offset, err := capture.ReadShellCursor(dir)
	if err != nil {
		t.Fatalf("read shell cursor: %v", err)
	}
	if offset != 0 {
		t.Fatalf("expected 0, got %d", offset)
	}

	if err := capture.WriteShellCursor(dir, 42); err != nil {
		t.Fatalf("write shell cursor: %v", err)
	}

	got, err := capture.ReadShellCursor(dir)
	if err != nil {
		t.Fatalf("read shell cursor back: %v", err)
	}
	if got != 42 {
		t.Fatalf("expected 42, got %d", got)
	}
}

func TestCursor_ResolveDefaultScope(t *testing.T) {
	dir := t.TempDir()

	// Not a git repo
	scope := capture.ResolveDefaultScope(dir)
	if scope != "global" {
		t.Fatalf("expected 'global', got %q", scope)
	}

	// Make it a git repo
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	subDir := filepath.Join(dir, "nested", "subpkg")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}

	scopeFromSub := capture.ResolveDefaultScope(subDir)
	expected := "project:" + capture.SanitizeScopeName(filepath.Base(dir))
	if scopeFromSub != expected {
		t.Fatalf("expected %q, got %q", expected, scopeFromSub)
	}
}
