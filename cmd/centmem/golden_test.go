package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// goldenDir returns the absolute path to the committed golden fixtures.
func goldenDir() string {
	wd, _ := os.Getwd()
	return filepath.Join(wd, "..", "..", "testdata", "golden")
}

// TestCLI_Golden_Init asserts init output shape matches the committed golden file.
// Dynamic values (db path) are normalized before comparison.
func TestCLI_Golden_Init(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	stdout, _, code := runCLI(t, home, "init")
	if code != 0 {
		t.Fatalf("init exit code = %d, want 0", code)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("init output not valid JSON: %v", err)
	}
	got["db"] = "<DB_PATH>" // normalize dynamic path

	wantBytes, err := os.ReadFile(filepath.Join(goldenDir(), "init.golden.json"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var want map[string]any
	if err := json.Unmarshal(wantBytes, &want); err != nil {
		t.Fatalf("golden not valid JSON: %v", err)
	}

	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("init output mismatch\ngot:  %s\nwant: %s", gotJSON, wantJSON)
	}
}

// TestCLI_Golden_GetNotFound asserts the stderr error shape matches the golden.
func TestCLI_Golden_GetNotFound(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	_, stderr, code := runCLI(t, home, "get", "--scope", "project:cent-mem", "--key", "missing.key")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stderr), &got); err != nil {
		t.Fatalf("stderr not valid JSON: %v", err)
	}

	wantBytes, err := os.ReadFile(filepath.Join(goldenDir(), "get_not_found.golden.json"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var want map[string]any
	if err := json.Unmarshal(wantBytes, &want); err != nil {
		t.Fatalf("golden not valid JSON: %v", err)
	}

	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("error shape mismatch\ngot:  %s\nwant: %s", gotJSON, wantJSON)
	}
}

// TestE2E_WriteThenRecallAcrossScopes validates the flagship flow: a memory
// written to global is recalled from a project scope via inheritance.
func TestE2E_WriteThenRecallAcrossScopes(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	// Write a durable fact + note into the global scope.
	_, _, code := runCLI(t, home, "set", "--scope", "global", "--key", "project.conventions", "--value", `"use tabs"`, "--tags", "convention")
	if code != 0 {
		t.Fatalf("set exit code = %d, want 0", code)
	}
	_, _, code = runCLI(t, home, "put", "--scope", "global", "--type", "note", "--content", "we deploy via github actions to fly", "--tags", "deploy,ci")
	if code != 0 {
		t.Fatalf("put exit code = %d, want 0", code)
	}

	// Recall from the project scope (inherits global).
	stdout, _, code := runCLI(t, home, "recall", "deploy", "--scope", "project:cent-mem", "--top", "5")
	if code != 0 {
		t.Fatalf("recall exit code = %d, want 0", code)
	}
	m := parseJSON(t, stdout)
	results, ok := m["results"].([]any)
	if !ok {
		t.Fatalf("expected results array, got %T", m["results"])
	}
	if len(results) == 0 {
		t.Fatal("expected inherited result from global scope")
	}
	first := results[0].(map[string]any)
	if first["scope"] != "global" {
		t.Errorf("expected scope global, got %v", first["scope"])
	}

	// get a fact inherited from global.
	getOut, _, code := runCLI(t, home, "get", "--scope", "project:cent-mem", "--key", "project.conventions", "--inherit")
	if code != 0 {
		t.Fatalf("get exit code = %d, want 0", code)
	}
	gm := parseJSON(t, getOut)
	if gm["value"] != "use tabs" {
		t.Errorf("inherited fact value = %v, want use tabs", gm["value"])
	}
	if gm["scope"] != "global" {
		t.Errorf("fact scope = %v, want global", gm["scope"])
	}
}
