package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/config"
)

func TestE2E_ConfigThreeInterfacesRoundTrip(t *testing.T) {
	stubDownloader()
	home := newHome(t)

	// Step 1: Initialize with centmem init
	_, _, code := runCLI(t, home, "init", "--non-interactive")
	if code != 0 {
		t.Fatalf("init failed (code %d)", code)
	}

	// Step 2: Interface 1 (CLI Config Set)
	_, _, code = runCLI(t, home, "config", "set", "capture.harness", "cursor")
	if code != 0 {
		t.Fatalf("config set harness failed (code %d)", code)
	}
	_, _, code = runCLI(t, home, "config", "set", "capture.scope", "project:e2e-app")
	if code != 0 {
		t.Fatalf("config set scope failed (code %d)", code)
	}

	// Step 3: Verify CLI Config Get
	stdout, _, code := runCLI(t, home, "config", "get", "capture.scope")
	if code != 0 {
		t.Fatalf("config get scope failed (code %d)", code)
	}
	var getRes struct {
		OK    bool   `json:"ok"`
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal([]byte(stdout), &getRes); err != nil {
		t.Fatalf("unmarshal get output: %v", err)
	}
	if getRes.Value != "project:e2e-app" {
		t.Errorf("expected scope project:e2e-app, got %q", getRes.Value)
	}

	// Step 4: Interface 3 (Direct config.toml editing)
	configPath := filepath.Join(home, "config.toml")
	fileCfg, err := config.LoadTOML(configPath)
	if err != nil {
		t.Fatalf("LoadTOML failed: %v", err)
	}
	fileCfg.Capture.Scope = "project:modified-app"
	fileCfg.Capture.Enabled = true
	if err := config.SaveTOML(configPath, fileCfg); err != nil {
		t.Fatalf("SaveTOML failed: %v", err)
	}

	// Step 5: Verify CLI picks up direct config.toml changes
	stdout, _, code = runCLI(t, home, "config", "get", "capture.scope")
	if code != 0 {
		t.Fatalf("config get after edit failed (code %d)", code)
	}
	if err := json.Unmarshal([]byte(stdout), &getRes); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if getRes.Value != "project:modified-app" {
		t.Errorf("expected project:modified-app, got %q", getRes.Value)
	}

	// Step 6: Create a transcript and run capture run (it should use project:modified-app automatically)
	fixturePath := filepath.Join(home, "test_transcript.jsonl")
	transcriptContent := `{"role": "user", "content": "How should we store database credentials?"}
{"role": "assistant", "content": "We decided to store database credentials in environment variables."}
`
	if err := os.WriteFile(fixturePath, []byte(transcriptContent), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	captureOut, stderr, code := runCLI(t, home, "capture", "run", "--transcript", fixturePath)
	if code != 0 {
		t.Fatalf("capture run failed (code %d): %s", code, stderr)
	}
	if !strings.Contains(captureOut, `"captured":1`) {
		t.Errorf("expected 1 captured item in summary: %s", captureOut)
	}

	// Step 7: Recall memory from the modified project scope
	recallOut, _, code := runCLI(t, home, "recall", "database credentials", "--scope", "project:modified-app")
	if code != 0 {
		t.Fatalf("recall failed (code %d)", code)
	}
	if !strings.Contains(recallOut, "environment variables") {
		t.Errorf("expected recall to find captured memory: %s", recallOut)
	}
}
