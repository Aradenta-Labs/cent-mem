package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRecallInteractiveFlag(t *testing.T) {
	cmd, ok := commands["recall"]
	if !ok {
		t.Fatal("recall command not found in commands registry")
	}

	hasInteractive := false
	for _, f := range cmd.flags {
		if f == "--interactive" {
			hasInteractive = true
			break
		}
	}
	if !hasInteractive {
		t.Errorf("expected --interactive flag in recall flags slice, got: %v", cmd.flags)
	}

	if !strings.Contains(cmd.usage, "--interactive") {
		t.Errorf("expected --interactive in recall usage string, got: %q", cmd.usage)
	}
}

func TestRecallInteractiveFallback(t *testing.T) {
	home := newHome(t)
	stubDownloader()

	// Put a test memory
	_, _, code := runCLI(t, home, "put", "--scope", "project:test-tui", "--type", "note", "--content", "Interactive fallback testing memory", "--tags", "tui,fallback")
	if code != 0 {
		t.Fatalf("put failed with code %d", code)
	}

	// Recall with query first, then --interactive
	stdout, stderr, code := runCLI(t, home, "recall", "fallback testing", "--scope", "project:test-tui", "--interactive")
	if code != 0 {
		t.Fatalf("recall with --interactive failed with code %d: %s", code, stderr)
	}

	var res struct {
		Ok      bool             `json:"ok"`
		Query   string           `json:"query"`
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("failed to parse JSON stdout: %v\noutput: %s", err, stdout)
	}

	if !res.Ok {
		t.Fatalf("expected ok: true, got %v", res.Ok)
	}
	if len(res.Results) == 0 {
		t.Fatal("expected at least 1 result in fallback recall")
	}

	found := false
	for _, r := range res.Results {
		if content, ok := r["content"].(string); ok && strings.Contains(content, "Interactive fallback testing") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected test note in results, got: %v", res.Results)
	}

	// Recall with --interactive first, then query
	stdout2, stderr2, code2 := runCLI(t, home, "recall", "--interactive", "fallback testing", "--scope", "project:test-tui")
	if code2 != 0 {
		t.Fatalf("recall with --interactive first failed with code %d: %s", code2, stderr2)
	}

	var res2 struct {
		Ok      bool             `json:"ok"`
		Query   string           `json:"query"`
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout2), &res2); err != nil {
		t.Fatalf("failed to parse JSON stdout2: %v\noutput: %s", err, stdout2)
	}
	if !res2.Ok || len(res2.Results) == 0 {
		t.Fatalf("expected ok results when --interactive precedes query, got %v", res2)
	}

	// Recall with --interactive without any query (empty query fallback)
	stdout3, stderr3, code3 := runCLI(t, home, "recall", "--interactive", "--scope", "project:test-tui")
	if code3 != 0 {
		t.Fatalf("recall with --interactive and empty query failed with code %d: %s", code3, stderr3)
	}

	var res3 struct {
		Ok      bool             `json:"ok"`
		Query   string           `json:"query"`
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout3), &res3); err != nil {
		t.Fatalf("failed to parse JSON stdout3: %v\noutput: %s", err, stdout3)
	}
	if !res3.Ok || len(res3.Results) == 0 {
		t.Fatalf("expected ok results when --interactive has no query, got %v", res3)
	}

	// Recall with --interactive and explicit --top 1
	stdout4, stderr4, code4 := runCLI(t, home, "recall", "fallback", "--scope", "project:test-tui", "--interactive", "--top", "1")
	if code4 != 0 {
		t.Fatalf("recall with --interactive and --top 1 failed with code %d: %s", code4, stderr4)
	}

	var res4 struct {
		Ok      bool             `json:"ok"`
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout4), &res4); err != nil {
		t.Fatalf("failed to parse JSON stdout4: %v\noutput: %s", err, stdout4)
	}
	if !res4.Ok || len(res4.Results) != 1 {
		t.Fatalf("expected exactly 1 result with --top 1, got %d", len(res4.Results))
	}
}
