package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/capture"
)

func setupRichTestGitRepo(t *testing.T) string {
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
	run("config", "user.email", "runner@example.com")
	run("config", "commit.gpgsign", "false")

	// Commit 1: initial docs
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# Project Readme\n"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	run("add", "README.md")
	run("commit", "-m", "chore: initial commit")

	// Commit 2: architectural decision
	configPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(configPath, []byte("wal = true\n"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	run("add", "config.toml")
	run("commit", "-m", "refactor: enable WAL journal mode\n\nDecided to switch to WAL journal mode to improve multi-process concurrency.")

	// Commit 3: dependency update
	gomodPath := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(gomodPath, []byte("module example.com/rich\n\ngo 1.22\n\nrequire github.com/mattn/go-sqlite3 v1.14.22\n"), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	run("add", "go.mod")
	run("commit", "-m", "feat: add sqlite3 driver")

	return dir
}

func TestE2ECapture_Git(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	repoDir := setupRichTestGitRepo(t)

	// Initialize store
	_, _, initCode := runCLI(t, home, "init", "--non-interactive")
	if initCode != 0 {
		t.Fatalf("init failed with code %d", initCode)
	}

	scope := "project:rich-repo"

	// 1. Dry run
	stdout, stderr, code := runCLI(t, home, "capture", "git", "--repo", repoDir, "--scope", scope, "--dry-run")
	if code != 0 {
		t.Fatalf("capture git dry-run failed (code %d): %s", code, stderr)
	}
	var dryRes capture.CaptureResult
	if err := json.Unmarshal([]byte(stdout), &dryRes); err != nil {
		t.Fatalf("unmarshal dry-run json: %v\nOutput: %s", err, stdout)
	}
	if !dryRes.OK || dryRes.Scanned != 3 {
		t.Fatalf("expected dry-run ok and 3 commits, got %+v", dryRes)
	}

	// 2. Real run
	stdout, stderr, code = runCLI(t, home, "capture", "git", "--repo", repoDir, "--scope", scope)
	if code != 0 {
		t.Fatalf("capture git failed (code %d): %s", code, stderr)
	}
	var realRes capture.CaptureResult
	if err := json.Unmarshal([]byte(stdout), &realRes); err != nil {
		t.Fatalf("unmarshal json: %v\nOutput: %s", err, stdout)
	}
	if realRes.MemoriesCreated < 3 {
		t.Fatalf("expected at least 3 memories created, got %d", realRes.MemoriesCreated)
	}

	// 3. Verify get dependency fact
	stdout, stderr, code = runCLI(t, home, "get", "--scope", scope, "--key", "deps.github.com/mattn/go-sqlite3")
	if code != 0 {
		t.Fatalf("get fact failed (code %d): %s", code, stderr)
	}
	var factResp struct {
		OK    bool   `json:"ok"`
		Key   string `json:"key"`
		Value string `json:"value"`
		Scope string `json:"scope"`
	}
	if err := json.Unmarshal([]byte(stdout), &factResp); err != nil {
		t.Fatalf("unmarshal fact json: %v", err)
	}
	if factResp.Value != "v1.14.22" {
		t.Fatalf("unexpected dependency fact: %+v", factResp)
	}

	// 4. Incremental run: 0 scanned
	stdout, stderr, code = runCLI(t, home, "capture", "git", "--repo", repoDir, "--scope", scope)
	if code != 0 {
		t.Fatalf("incremental capture git failed: %s", stderr)
	}
	var incRes capture.CaptureResult
	if err := json.Unmarshal([]byte(stdout), &incRes); err != nil {
		t.Fatalf("unmarshal inc json: %v", err)
	}
	if incRes.Scanned != 0 {
		t.Fatalf("expected 0 scanned on incremental run, got %d", incRes.Scanned)
	}
}

func TestE2ECapture_Docs(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	dir := t.TempDir()

	docFile := filepath.Join(dir, "architecture.md")
	docContent := `# System Architecture

The core database uses SQLite in WAL mode.

## Search Pipeline

Hybrid search combines vectors and FTS5 BM25.
`
	if err := os.WriteFile(docFile, []byte(docContent), 0644); err != nil {
		t.Fatalf("write doc: %v", err)
	}

	// Initialize store
	_, _, initCode := runCLI(t, home, "init", "--non-interactive")
	if initCode != 0 {
		t.Fatalf("init failed with code %d", initCode)
	}

	scope := "project:docs-test"

	// Real run
	stdout, stderr, code := runCLI(t, home, "capture", "docs", "--dir", dir, "--scope", scope)
	if code != 0 {
		t.Fatalf("capture docs failed (code %d): %s", code, stderr)
	}
	var res capture.CaptureResult
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("unmarshal docs json: %v\nOutput: %s", err, stdout)
	}
	if res.MemoriesCreated != 2 {
		t.Fatalf("expected 2 doc chunks created, got %d", res.MemoriesCreated)
	}

	// Recall test
	stdout, stderr, code = runCLI(t, home, "recall", "search pipeline", "--scope", scope)
	if code != 0 {
		t.Fatalf("recall failed (code %d): %s", code, stderr)
	}
	if !strings.Contains(stdout, "Search Pipeline") || !strings.Contains(stdout, "architecture.md") {
		t.Fatalf("expected doc breadcrumb in recall output: %s", stdout)
	}

	// Incremental run: mtime unchanged -> 0 items
	stdout, stderr, code = runCLI(t, home, "capture", "docs", "--dir", dir, "--scope", scope)
	if code != 0 {
		t.Fatalf("second capture docs failed: %s", stderr)
	}
	var secondRes capture.CaptureResult
	if err := json.Unmarshal([]byte(stdout), &secondRes); err != nil {
		t.Fatalf("unmarshal second json: %v", err)
	}
	if len(secondRes.Items) != 0 {
		t.Fatalf("expected 0 new items on unchanged docs, got %d", len(secondRes.Items))
	}
}

func TestE2ECapture_Shell(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	dir := t.TempDir()

	histPath := filepath.Join(dir, ".zsh_history")
	histContent := `: 1788740000:0;go test -tags fts5 ./...
: 1788740010:0;go test -tags fts5 ./...
: 1788740020:0;git status
: 1788740030:0;export API_KEY=secret_key_1234567890
`
	if err := os.WriteFile(histPath, []byte(histContent), 0644); err != nil {
		t.Fatalf("write history: %v", err)
	}

	// Initialize store
	_, _, initCode := runCLI(t, home, "init", "--non-interactive")
	if initCode != 0 {
		t.Fatalf("init failed with code %d", initCode)
	}

	scope := "project:shell-test"

	stdout, stderr, code := runCLI(t, home, "capture", "shell", "--history", histPath, "--shell", "zsh", "--scope", scope)
	if code != 0 {
		t.Fatalf("capture shell failed (code %d): %s", code, stderr)
	}
	var res capture.CaptureResult
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("unmarshal shell json: %v\nOutput: %s", err, stdout)
	}
	if res.MemoriesCreated != 1 {
		t.Fatalf("expected 1 fact created, got %d", res.MemoriesCreated)
	}

	// Verify fact via get
	stdout, stderr, code = runCLI(t, home, "get", "--scope", scope, "--key", "shell.frequent_commands")
	if code != 0 {
		t.Fatalf("get fact failed: %s", stderr)
	}
	if !strings.Contains(stdout, "go test") || !strings.Contains(stdout, "git status") {
		t.Fatalf("expected patterns in get output: %s", stdout)
	}
}

func TestE2ECapture_Comments(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	dir := t.TempDir()

	codePath := filepath.Join(dir, "service.go")
	codeContent := `package main

// TODO: add connection pooling
func Connect() {
	// SECURITY: sanitize auth token header
}
`
	if err := os.WriteFile(codePath, []byte(codeContent), 0644); err != nil {
		t.Fatalf("write code: %v", err)
	}

	// Initialize store
	_, _, initCode := runCLI(t, home, "init", "--non-interactive")
	if initCode != 0 {
		t.Fatalf("init failed with code %d", initCode)
	}

	scope := "project:comments-test"

	stdout, stderr, code := runCLI(t, home, "capture", "comments", "--dir", dir, "--scope", scope)
	if code != 0 {
		t.Fatalf("capture comments failed (code %d): %s", code, stderr)
	}
	var res capture.CaptureResult
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("unmarshal comments json: %v\nOutput: %s", err, stdout)
	}
	if res.MemoriesCreated != 2 {
		t.Fatalf("expected 2 comments captured, got %d", res.MemoriesCreated)
	}

	// Recall test
	stdout, stderr, code = runCLI(t, home, "recall", "connection pooling", "--scope", scope)
	if code != 0 {
		t.Fatalf("recall comments failed: %s", stderr)
	}
	if !strings.Contains(stdout, "[file: service.go:3] [TODO]") {
		t.Fatalf("expected comment provenance in recall output: %s", stdout)
	}
}
