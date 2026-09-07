package capture_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/capture"
)

func TestCaptureShell_ZshAndScrubbing(t *testing.T) {
	dir := t.TempDir()
	st := setupTestStore(t)
	ctx := context.Background()

	zshHistPath := filepath.Join(dir, ".zsh_history")
	content := `: 1788740000:0;git status
: 1788740010:0;git status
: 1788740020:0;go test -tags fts5 ./...
: 1788740030:0;go test -tags fts5 ./internal/store/...
: 1788740040:0;export SECRET_TOKEN=supersecrettoken123456789
: 1788740050:0;docker compose up -d
: 1788740060:0;cd /var/www && git pull origin main
`
	if err := os.WriteFile(zshHistPath, []byte(content), 0644); err != nil {
		t.Fatalf("write zsh history: %v", err)
	}

	res, err := capture.CaptureShell(ctx, st, capture.ShellCaptureConfig{
		HistoryPath: zshHistPath,
		ShellType:   "zsh",
		Scope:       "project:test",
		TopN:        5,
		DryRun:      false,
	})
	if err != nil {
		t.Fatalf("capture shell: %v", err)
	}
	if res.Scanned != 7 {
		t.Fatalf("expected 7 lines scanned, got %d", res.Scanned)
	}
	if len(res.Items) != 1 {
		t.Fatalf("expected 1 fact item, got %d", len(res.Items))
	}

	fact, err := st.GetFact(ctx, "project:test", "shell.frequent_commands", false)
	if err != nil {
		t.Fatalf("get shell fact: %v", err)
	}
	if fact == nil {
		t.Fatal("fact should exist in store")
	}

	var patterns map[string]int
	if err := json.Unmarshal([]byte(fact.Value), &patterns); err != nil {
		t.Fatalf("unmarshal fact value: %v", err)
	}

	if patterns["git status"] != 2 {
		t.Errorf("expected 'git status' count 2, got %d", patterns["git status"])
	}
	if patterns["go test"] != 2 {
		t.Errorf("expected 'go test' count 2, got %d", patterns["go test"])
	}
	if patterns["docker compose up"] != 1 {
		t.Errorf("expected 'docker compose up' count 1, got %d", patterns["docker compose up"])
	}
	if patterns["git pull"] != 1 {
		t.Errorf("expected 'git pull' count 1, got %d", patterns["git pull"])
	}
	// Verify secret was not retained as a pattern
	if _, found := patterns["export"]; found {
		t.Error("secret command should not be in frequent commands")
	}
}

func TestCaptureShell_Bash(t *testing.T) {
	dir := t.TempDir()
	st := setupTestStore(t)
	ctx := context.Background()

	bashHistPath := filepath.Join(dir, ".bash_history")
	content := `#1788740000
npm run build
#1788740010
npm run build
#1788740020
npm test
`
	if err := os.WriteFile(bashHistPath, []byte(content), 0644); err != nil {
		t.Fatalf("write bash history: %v", err)
	}

	res, err := capture.CaptureShell(ctx, st, capture.ShellCaptureConfig{
		HistoryPath: bashHistPath,
		ShellType:   "bash",
		Scope:       "project:test",
		TopN:        5,
		DryRun:      false,
	})
	if err != nil {
		t.Fatalf("capture shell: %v", err)
	}

	fact, err := st.GetFact(ctx, "project:test", "shell.frequent_commands", false)
	if err != nil {
		t.Fatalf("get shell fact: %v", err)
	}
	var patterns map[string]int
	if err := json.Unmarshal([]byte(fact.Value), &patterns); err != nil {
		t.Fatalf("unmarshal fact: %v", err)
	}

	if patterns["npm run"] != 2 {
		t.Errorf("expected 'npm run' count 2, got %d", patterns["npm run"])
	}
	if patterns["npm test"] != 1 {
		t.Errorf("expected 'npm test' count 1, got %d", patterns["npm test"])
	}
	_ = res
}
