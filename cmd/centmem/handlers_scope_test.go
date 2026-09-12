package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCLI_ScopeDelete_Force(t *testing.T) {
	home := newHome(t)
	stubDownloader()

	// Put memories in project:foo and child agent scope
	_, _, code := runCLI(t, home, "put", "--scope", "project:foo", "--type", "note", "--content", "Root note")
	if code != 0 {
		t.Fatalf("put failed with code %d", code)
	}

	_, _, code = runCLI(t, home, "put", "--scope", "project:foo/agent:bot", "--type", "note", "--content", "Child note")
	if code != 0 {
		t.Fatalf("put child failed with code %d", code)
	}

	// Put memory in sibling project sharing string prefix
	_, _, code = runCLI(t, home, "put", "--scope", "project:foo-bar", "--type", "note", "--content", "Sibling note")
	if code != 0 {
		t.Fatalf("put sibling failed with code %d", code)
	}

	// Delete scope with --force
	stdout, stderr, code := runCLI(t, home, "scope", "delete", "project:foo", "--force")
	if code != 0 {
		t.Fatalf("scope delete failed with code %d: stderr=%s", code, stderr)
	}

	var res struct {
		Ok              bool   `json:"ok"`
		DeletedScope    string `json:"deleted_scope"`
		MemoriesDeleted int    `json:"memories_deleted"`
		ScopesDeleted   int    `json:"scopes_deleted"`
	}
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("unmarshal scope delete output: %v; stdout=%s", err, stdout)
	}

	if !res.Ok || res.DeletedScope != "project:foo" {
		t.Errorf("unexpected delete result: %+v", res)
	}
	if res.MemoriesDeleted != 2 {
		t.Errorf("expected 2 memories deleted, got %d", res.MemoriesDeleted)
	}
	if res.ScopesDeleted != 2 {
		t.Errorf("expected 2 scopes deleted, got %d", res.ScopesDeleted)
	}

	// Verify memories no longer recalled for project:foo
	stdout, _, code = runCLI(t, home, "recall", "Root note", "--scope", "project:foo")
	var recallRes struct {
		Results []any `json:"results"`
	}
	_ = json.Unmarshal([]byte(stdout), &recallRes)
	if len(recallRes.Results) != 0 {
		t.Errorf("expected 0 recall results for deleted scope, got %d", len(recallRes.Results))
	}

	// Verify sibling project:foo-bar is still recallable
	stdout, _, code = runCLI(t, home, "recall", "Sibling note", "--scope", "project:foo-bar")
	if code != 0 {
		t.Fatalf("recall sibling failed with code %d", code)
	}
	var siblingRecall struct {
		Results []any `json:"results"`
	}
	_ = json.Unmarshal([]byte(stdout), &siblingRecall)
	if len(siblingRecall.Results) != 1 {
		t.Errorf("expected 1 recall result for sibling scope, got %d", len(siblingRecall.Results))
	}
}

func TestCLI_ScopeDelete_NonInteractiveWithoutForce(t *testing.T) {
	home := newHome(t)
	stubDownloader()

	_, _, code := runCLI(t, home, "put", "--scope", "project:bar", "--type", "note", "--content", "Note to delete")
	if code != 0 {
		t.Fatalf("put failed with code %d", code)
	}

	// Calling without --force in non-interactive test environment should fail
	_, stderr, code := runCLI(t, home, "scope", "delete", "project:bar")
	if code != 1 {
		t.Fatalf("expected exit code 1 for non-interactive deletion without --force, got %d", code)
	}
	if !strings.Contains(stderr, "--force") {
		t.Errorf("expected stderr to mention --force, got: %s", stderr)
	}
}

func TestCLI_ScopeDelete_RejectsGlobal(t *testing.T) {
	home := newHome(t)
	stubDownloader()

	_, stderr, code := runCLI(t, home, "scope", "delete", "global", "--force")
	if code != 1 {
		t.Fatalf("expected exit code 1 when deleting global, got %d", code)
	}
	if !strings.Contains(stderr, "global") {
		t.Errorf("expected stderr to mention global, got: %s", stderr)
	}
}

func TestCLI_ScopeDelete_NotFound(t *testing.T) {
	home := newHome(t)
	stubDownloader()

	// Initializing store
	_, _, _ = runCLI(t, home, "init")

	_, _, code := runCLI(t, home, "scope", "delete", "project:nonexistent", "--force")
	if code != 2 {
		t.Fatalf("expected exit code 2 (not found) when deleting nonexistent scope, got %d", code)
	}
}

func TestCLI_ScopeList(t *testing.T) {
	home := newHome(t)
	stubDownloader()

	_, _, _ = runCLI(t, home, "put", "--scope", "project:baz", "--type", "note", "--content", "Test note")

	stdout, stderr, code := runCLI(t, home, "scope", "list")
	if code != 0 {
		t.Fatalf("scope list failed with code %d: stderr=%s", code, stderr)
	}

	var res struct {
		Ok     bool  `json:"ok"`
		Scopes []any `json:"scopes"`
	}
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("unmarshal scope list output: %v; stdout=%s", err, stdout)
	}
	if !res.Ok || len(res.Scopes) == 0 {
		t.Errorf("expected non-empty scopes list: %+v", res)
	}
}
