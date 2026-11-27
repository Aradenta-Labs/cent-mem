package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCLI_Golden_TagsListAndMerge(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	// 1. Seed memories for tags list golden test.
	runCLI(t, home, "put", "--scope", "project:golden-tags", "--type", "note", "--content", "SQLite WAL enabled", "--tags", "database,db")
	runCLI(t, home, "put", "--scope", "project:golden-tags", "--type", "note", "--content", "Indexes tuned", "--tags", "database")

	stdout, stderr, code := runCLI(t, home, "tags", "list", "--scope", "project:golden-tags")
	if code != 0 {
		t.Fatalf("tags list code = %d, want 0, stderr: %s", code, stderr)
	}

	var gotList map[string]any
	if err := json.Unmarshal([]byte(stdout), &gotList); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	// Normalize dynamic last_used timestamps before golden comparison.
	if tagsSlice, ok := gotList["tags"].([]any); ok {
		for _, item := range tagsSlice {
			if m, ok := item.(map[string]any); ok {
				if _, exists := m["last_used"]; exists {
					m["last_used"] = "<TIMESTAMP>"
				}
			}
		}
	}

	wantBytes, err := os.ReadFile(filepath.Join(goldenDir(), "tags_list.golden.json"))
	if err != nil {
		t.Fatalf("read tags_list golden: %v", err)
	}
	var wantList map[string]any
	_ = json.Unmarshal(wantBytes, &wantList)

	gotJSON, _ := json.Marshal(gotList)
	wantJSON, _ := json.Marshal(wantList)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("tags list golden mismatch:\ngot:  %s\nwant: %s", gotJSON, wantJSON)
	}

	// 2. Seed memories for tags merge (--dry-run and live) golden test.
	runCLI(t, home, "put", "--scope", "project:golden-merge", "--type", "note", "--content", "DB choice", "--tags", "db")
	runCLI(t, home, "put", "--scope", "project:golden-merge", "--type", "note", "--content", "SQLite config", "--tags", "sqlite,database")

	dryOut, dryErr, dryCode := runCLI(t, home, "tags", "merge", "--scope", "project:golden-merge", "--from", "db,sqlite", "--to", "database", "--dry-run")
	if dryCode != 0 {
		t.Fatalf("tags merge --dry-run code = %d, stderr: %s", dryCode, dryErr)
	}
	var dryGot map[string]any
	_ = json.Unmarshal([]byte(dryOut), &dryGot)
	if dryGot["dry_run"] != true || dryGot["affected_count"] != float64(2) {
		t.Errorf("dry-run response = %v, want dry_run=true affected_count=2", dryGot)
	}

	mergeOut, mergeErr, mergeCode := runCLI(t, home, "tags", "merge", "--scope", "project:golden-merge", "--from", "db,sqlite", "--to", "database")
	if mergeCode != 0 {
		t.Fatalf("tags merge code = %d, stderr: %s", mergeCode, mergeErr)
	}
	var gotMerge map[string]any
	_ = json.Unmarshal([]byte(mergeOut), &gotMerge)

	wantMergeBytes, err := os.ReadFile(filepath.Join(goldenDir(), "tags_merge.golden.json"))
	if err != nil {
		t.Fatalf("read tags_merge golden: %v", err)
	}
	var wantMerge map[string]any
	_ = json.Unmarshal(wantMergeBytes, &wantMerge)

	gotMergeJSON, _ := json.Marshal(gotMerge)
	wantMergeJSON, _ := json.Marshal(wantMerge)
	if string(gotMergeJSON) != string(wantMergeJSON) {
		t.Errorf("tags merge golden mismatch:\ngot:  %s\nwant: %s", gotMergeJSON, wantMergeJSON)
	}
}

func TestCLI_CurateTags_ApplyProposal_AndRecallCanonical(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")
	runCLI(t, home, "config", "set", "llm.backend", "disabled")

	// Seed memories with fragmented lexical tag variants.
	runCLI(t, home, "put", "--scope", "project:taxe2e", "--type", "note", "--content", "Architecture decision one about SQLite", "--tags", "decision")
	runCLI(t, home, "put", "--scope", "project:taxe2e", "--type", "note", "--content", "Architecture decision two about FTS5", "--tags", "decision")
	runCLI(t, home, "put", "--scope", "project:taxe2e", "--type", "note", "--content", "Architecture decision three about WAL", "--tags", "decisions")
	runCLI(t, home, "put", "--scope", "project:taxe2e/agent:codex", "--type", "note", "--content", "Architecture decision four about caching", "--tags", "decisions")

	// 1. Run curate --type tags.
	stdout, stderr, code := runCLI(t, home, "curate", "--scope", "project:taxe2e", "--type", "tags")
	if code != 0 {
		t.Fatalf("curate --type tags code = %d, stderr: %s", code, stderr)
	}
	var curateResp struct {
		OK               bool    `json:"ok"`
		ProposalsCreated []int64 `json:"proposals_created"`
		TagClustersFound int     `json:"tag_clusters_found"`
	}
	if err := json.Unmarshal([]byte(stdout), &curateResp); err != nil {
		t.Fatalf("unmarshal curate resp: %v", err)
	}
	if !curateResp.OK || curateResp.TagClustersFound != 1 || len(curateResp.ProposalsCreated) != 1 {
		t.Fatalf("unexpected curate resp: %+v", curateResp)
	}

	// 2. Apply the staged tag_merge proposal.
	propID := curateResp.ProposalsCreated[0]
	_, stderr, code = runCLI(t, home, "proposals", "apply", formatInt64(propID))
	if code != 0 {
		t.Fatalf("proposals apply code = %d, stderr: %s", code, stderr)
	}

	// 3. Verify recall --tags decision --children finds ALL 4 previously fragmented memories.
	recallOut, recallErr, recallCode := runCLI(t, home, "recall", "architecture decision", "--scope", "project:taxe2e", "--tags", "decision", "--children", "--top", "10")
	if recallCode != 0 {
		t.Fatalf("recall code = %d, stderr: %s", recallCode, recallErr)
	}
	var recallResp struct {
		Results []struct {
			ID   int64    `json:"id"`
			Tags []string `json:"tags"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(recallOut), &recallResp); err != nil {
		t.Fatalf("unmarshal recall: %v", err)
	}
	if len(recallResp.Results) != 4 {
		t.Fatalf("expected 4 memories recalled with canonical tag 'decision', got %d: %s", len(recallResp.Results), recallOut)
	}

	// 4. Verify tags list now shows only 'decision' with count 4.
	tagsOut, _, tagsCode := runCLI(t, home, "tags", "list", "--scope", "project:taxe2e")
	if tagsCode != 0 {
		t.Fatalf("tags list code = %d", tagsCode)
	}
	var tagsResp struct {
		Count int `json:"count"`
		Tags  []struct {
			Tag   string `json:"tag"`
			Count int    `json:"count"`
		} `json:"tags"`
	}
	_ = json.Unmarshal([]byte(tagsOut), &tagsResp)
	if tagsResp.Count != 1 || tagsResp.Tags[0].Tag != "decision" || tagsResp.Tags[0].Count != 4 {
		t.Fatalf("expected single canonical tag decision(4), got %+v", tagsResp)
	}
}

func TestCLI_Tags_ValidationErrors(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	cases := [][]string{
		{"tags"},
		{"tags", "unknown"},
		{"tags", "list", "--scope", "bad::scope"},
		{"tags", "merge", "--from", "db", "--to", "database"},
		{"tags", "merge", "--scope", "project:x", "--to", "database"},
		{"tags", "merge", "--scope", "project:x", "--from", "db"},
		{"tags", "merge", "--scope", "project:x", "--from", "database", "--to", "database"},
		{"tags", "merge", "--scope", "project:x", "--from", "db", "--to", "a,b"},
	}
	for _, args := range cases {
		_, _, code := runCLI(t, home, args...)
		if code != 1 {
			t.Errorf("args %v: code = %d, want 1", args, code)
		}
	}
}

func formatInt64(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}
