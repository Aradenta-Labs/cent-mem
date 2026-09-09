package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/store"
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

// TestCLI_Recall_SemanticMatched asserts that recall output for a paraphrase
// query carries a result whose matched_by includes "semantic" (the M2 contract),
// and that the stable shape matches the committed golden file. Dynamic fields
// (id, created_at, score) are normalized before comparison.
func TestCLI_Recall_SemanticMatched(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")
	runCLI(t, home, "put", "--scope", "global", "--type", "note", "--content", "we deploy via github actions to fly.io", "--tags", "deploy,ci")

	stdout, _, code := runCLI(t, home, "recall", "how do we ship?", "--scope", "global", "--top", "5")
	if code != 0 {
		t.Fatalf("recall exit code = %d, want 0", code)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("recall output not valid JSON: %v", err)
	}

	// Normalize dynamic fields.
	if results, ok := got["results"].([]any); ok {
		for _, r := range results {
			item := r.(map[string]any)
			item["id"] = "<ID>"
			item["created_at"] = "<TS>"
			item["score"] = "<SCORE>"
			if item["last_accessed_at"] != nil {
				item["last_accessed_at"] = "<TS>"
			}
		}
	}

	// Contract: the top result for a paraphrase query is matched semantically and has v1.5.0 telemetry.
	if results, ok := got["results"].([]any); ok && len(results) > 0 {
		top := results[0].(map[string]any)
		matched, _ := top["matched_by"].([]any)
		if !hasStrAny(matched, "semantic") {
			t.Errorf("expected top result matched_by to include semantic, got %v", top["matched_by"])
		}
		if _, ok := top["access_count"]; !ok {
			t.Errorf("expected access_count in recall item")
		}
		if _, ok := top["last_accessed_at"]; !ok {
			t.Errorf("expected last_accessed_at in recall item")
		}
	}

	wantBytes, err := os.ReadFile(filepath.Join(goldenDir(), "recall_paraphrase.golden.json"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var want map[string]any
	if err := json.Unmarshal(wantBytes, &want); err != nil {
		t.Fatalf("golden not valid JSON: %v", err)
	}

	gotJSON, _ := json.MarshalIndent(got, "", "  ")
	wantJSON, _ := json.MarshalIndent(want, "", "  ")
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("recall output mismatch\ngot:\n%s\nwant:\n%s", gotJSON, wantJSON)
	}
}

func hasStrAny(ss []any, want string) bool {
	for _, v := range ss {
		if v == want {
			return true
		}
	}
	return false
}

func TestCLI_Golden_ConfigSet(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init", "--non-interactive")

	stdout, stderr, code := runCLI(t, home, "config", "set", "capture.harness", "claude-code")
	if code != 0 {
		t.Fatalf("config set exit code = %d, want 0 (stderr=%s)", code, stderr)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("config set output not valid JSON: %v", err)
	}

	wantBytes, err := os.ReadFile(filepath.Join(goldenDir(), "config_set.golden.json"))
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
		t.Errorf("config set output mismatch\ngot:  %s\nwant: %s", gotJSON, wantJSON)
	}
}

func TestCLI_Golden_ConfigGet(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init", "--non-interactive")

	stdout, stderr, code := runCLI(t, home, "config", "get", "capture.backend")
	if code != 0 {
		t.Fatalf("config get exit code = %d, want 0 (stderr=%s)", code, stderr)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("config get output not valid JSON: %v", err)
	}

	wantBytes, err := os.ReadFile(filepath.Join(goldenDir(), "config_get_key.golden.json"))
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
		t.Errorf("config get output mismatch\ngot:  %s\nwant: %s", gotJSON, wantJSON)
	}
}

func TestCLI_Golden_CaptureSummary(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	transcript := `{"role":"user","content":"Let's decide on the database."}
{"role":"assistant","content":"We decided to use SQLite-vec for local embeddings"}
`
	transcriptPath := filepath.Join(home, "golden_transcript.jsonl")
	_ = os.WriteFile(transcriptPath, []byte(transcript), 0644)

	stdout, _, code := runCLI(t, home, "capture", "run", "--transcript", transcriptPath, "--harness", "antigravity")
	if code != 0 {
		t.Fatalf("capture run code = %d, want 0", code)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	// Normalize dynamic fields
	got["session_id"] = "<SESSION_ID>"
	got["started_at"] = "<TIMESTAMP>"
	got["ended_at"] = "<TIMESTAMP>"

	wantBytes, err := os.ReadFile(filepath.Join(goldenDir(), "capture_summary.golden.json"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var want map[string]any
	_ = json.Unmarshal(wantBytes, &want)

	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("summary golden mismatch:\ngot:  %s\nwant: %s", gotJSON, wantJSON)
	}
}

func TestCLI_Golden_CaptureRunDecision(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	transcript := `{"role":"assistant","content":"We decided to use SQLite-vec for local embeddings"}`
	transcriptPath := filepath.Join(home, "golden_run_decision.jsonl")
	_ = os.WriteFile(transcriptPath, []byte(transcript), 0644)

	stdout, stderr, code := runCLI(t, home, "capture", "run", "--transcript", transcriptPath, "--harness", "antigravity")
	if code != 0 {
		t.Fatalf("capture run code = %d, want 0, stderr: %s", code, stderr)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	// Normalize dynamic fields
	got["session_id"] = "<SESSION_ID>"
	got["started_at"] = "<TIMESTAMP>"
	got["ended_at"] = "<TIMESTAMP>"

	wantBytes, err := os.ReadFile(filepath.Join(goldenDir(), "capture_run_decision.golden.json"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var want map[string]any
	_ = json.Unmarshal(wantBytes, &want)

	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("capture run decision golden mismatch:\ngot:  %s\nwant: %s", gotJSON, wantJSON)
	}
}

func TestCLI_Golden_CaptureConvert(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	transcript := `User: What is the roadmap?
Assistant: We chose to release v1.3.0 next.
`
	transcriptPath := filepath.Join(home, "convert_golden.txt")
	_ = os.WriteFile(transcriptPath, []byte(transcript), 0644)

	stdout, _, code := runCLI(t, home, "capture", "convert", "--harness", "cursor", "--input", transcriptPath)
	if code != 0 {
		t.Fatalf("capture convert code = %d, want 0", code)
	}

	var got map[string]any
	_ = json.Unmarshal([]byte(stdout), &got)
	got["output"] = "<OUTPUT_PATH>"

	wantBytes, err := os.ReadFile(filepath.Join(goldenDir(), "capture_convert.golden.json"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var want map[string]any
	_ = json.Unmarshal(wantBytes, &want)

	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("convert golden mismatch:\ngot:  %s\nwant: %s", gotJSON, wantJSON)
	}
}

func TestCLI_Golden_CaptureCategories(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	stdout, _, code := runCLI(t, home, "capture", "categories", "--list")
	if code != 0 {
		t.Fatalf("capture categories code = %d, want 0", code)
	}

	var got map[string]any
	_ = json.Unmarshal([]byte(stdout), &got)

	wantBytes, err := os.ReadFile(filepath.Join(goldenDir(), "capture_categories.golden.json"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var want map[string]any
	_ = json.Unmarshal(wantBytes, &want)

	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("categories golden mismatch:\ngot:  %s\nwant: %s", gotJSON, wantJSON)
	}
}

func TestCLI_Golden_AskOffline(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	stdout, stderr, code := runCLI(t, home, "ask", "offline test question", "--scope", "project:golden")
	if code != 0 {
		t.Fatalf("ask code = %d, want 0, stderr: %s", code, stderr)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	got["answer"] = "<ANSWER>"
	got["conversation_id"] = "<CONVERSATION_ID>"

	wantBytes, err := os.ReadFile(filepath.Join(goldenDir(), "ask_offline.golden.json"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var want map[string]any
	_ = json.Unmarshal(wantBytes, &want)

	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("ask golden mismatch:\ngot:  %s\nwant: %s", gotJSON, wantJSON)
	}
}

func TestCLI_Golden_CurateSummary(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	stdout, stderr, code := runCLI(t, home, "curate", "--scope", "project:golden", "--dry-run")
	if code != 0 {
		t.Fatalf("curate code = %d, want 0, stderr: %s", code, stderr)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	wantBytes, err := os.ReadFile(filepath.Join(goldenDir(), "curate_summary.golden.json"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var want map[string]any
	_ = json.Unmarshal(wantBytes, &want)

	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("curate golden mismatch:\ngot:  %s\nwant: %s", gotJSON, wantJSON)
	}
}

func TestCLI_Golden_Summarize(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	stdout, stderr, code := runCLI(t, home, "summarize", "--scope", "project:golden")
	if code != 0 {
		t.Fatalf("summarize code = %d, want 0, stderr: %s", code, stderr)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	wantBytes, err := os.ReadFile(filepath.Join(goldenDir(), "summarize.golden.json"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var want map[string]any
	_ = json.Unmarshal(wantBytes, &want)

	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("summarize golden mismatch:\ngot:  %s\nwant: %s", gotJSON, wantJSON)
	}
}

func TestCLI_Golden_ProposalsList(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	stdout, stderr, code := runCLI(t, home, "proposals", "list", "--scope", "project:golden")
	if code != 0 {
		t.Fatalf("proposals list code = %d, want 0, stderr: %s", code, stderr)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	wantBytes, err := os.ReadFile(filepath.Join(goldenDir(), "proposals_list.golden.json"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var want map[string]any
	_ = json.Unmarshal(wantBytes, &want)

	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("proposals list golden mismatch:\ngot:  %s\nwant: %s", gotJSON, wantJSON)
	}
}

func TestCLI_Golden_ProposalsShow(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	s, err := store.Open(config.Config{DBPath: filepath.Join(home, "centmem.db")})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	_, _, _ = s.PutMemory(ctx, store.MemoryInput{Scope: "project:golden", Type: "note", Content: "mem1"})
	_, _, _ = s.PutMemory(ctx, store.MemoryInput{Scope: "project:golden", Type: "note", Content: "mem2"})

	payload, _ := json.Marshal(store.LinkProposalPayload{FromID: 1, ToID: 2, Relation: "supersedes"})
	propID, err := s.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:golden",
		ProposalType: "link",
		Title:        "Link memory #1 ──supersedes──► memory #2",
		Reasoning:    "Golden test reasoning",
		PayloadJSON:  string(payload),
	})
	if err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}

	stdout, stderr, code := runCLI(t, home, "proposals", "show", fmt.Sprintf("%d", propID))
	if code != 0 {
		t.Fatalf("proposals show code = %d, want 0, stderr: %s", code, stderr)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	if p, ok := got["proposal"].(map[string]any); ok {
		p["created_at"] = "<TIMESTAMP>"
		if p["applied_at"] != nil {
			p["applied_at"] = "<TIMESTAMP>"
		}
	}

	wantBytes, err := os.ReadFile(filepath.Join(goldenDir(), "proposals_show.golden.json"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var want map[string]any
	_ = json.Unmarshal(wantBytes, &want)

	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("proposals show golden mismatch:\ngot:  %s\nwant: %s", gotJSON, wantJSON)
	}
}

func TestCLI_Golden_ProposalsApply(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	s, err := store.Open(config.Config{DBPath: filepath.Join(home, "centmem.db")})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	_, _, _ = s.PutMemory(ctx, store.MemoryInput{Scope: "project:golden", Type: "note", Content: "mem1"})
	_, _, _ = s.PutMemory(ctx, store.MemoryInput{Scope: "project:golden", Type: "note", Content: "mem2"})

	payload, _ := json.Marshal(store.LinkProposalPayload{FromID: 1, ToID: 2, Relation: "supersedes"})
	propID, err := s.CreateProposal(ctx, &store.Proposal{
		ScopePath:    "project:golden",
		ProposalType: "link",
		Title:        "Link memory #1 ──supersedes──► memory #2",
		Reasoning:    "Golden test reasoning",
		PayloadJSON:  string(payload),
	})
	if err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}

	stdout, stderr, code := runCLI(t, home, "proposals", "apply", fmt.Sprintf("%d", propID))
	if code != 0 {
		t.Fatalf("proposals apply code = %d, want 0, stderr: %s", code, stderr)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	if p, ok := got["proposal"].(map[string]any); ok {
		p["created_at"] = "<TIMESTAMP>"
		p["applied_at"] = "<TIMESTAMP>"
	}

	wantBytes, err := os.ReadFile(filepath.Join(goldenDir(), "proposals_apply.golden.json"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var want map[string]any
	_ = json.Unmarshal(wantBytes, &want)

	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("proposals apply golden mismatch:\ngot:  %s\nwant: %s", gotJSON, wantJSON)
	}
}


