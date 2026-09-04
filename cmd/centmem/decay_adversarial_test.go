package main

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestAdversarial_CLI_DecayConfigAndRecall(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	// 1. Default config get
	t.Run("default_search_decay_is_zero", func(t *testing.T) {
		stdout, stderr, code := runCLI(t, home, "config", "get", "search.decay_half_life_days")
		if code != 0 {
			t.Fatalf("config get failed (exit %d): %s", code, stderr)
		}
		var res struct {
			OK    bool `json:"ok"`
			Key   string `json:"key"`
			Value int    `json:"value"`
		}
		if err := json.Unmarshal([]byte(stdout), &res); err != nil {
			t.Fatalf("unmarshal stdout: %v (stdout=%s)", err, stdout)
		}
		if !res.OK || res.Value != 0 {
			t.Errorf("expected default decay_half_life_days=0, got %+v", res)
		}
	})

	// 2. Set valid decay days
	t.Run("config_set_valid_decay_days", func(t *testing.T) {
		stdout, stderr, code := runCLI(t, home, "config", "set", "search.decay_half_life_days", "14")
		if code != 0 {
			t.Fatalf("config set failed (exit %d): %s", code, stderr)
		}
		var res struct {
			OK    bool `json:"ok"`
			Key   string `json:"key"`
			Value int    `json:"value"`
		}
		if err := json.Unmarshal([]byte(stdout), &res); err != nil {
			t.Fatalf("unmarshal stdout: %v", err)
		}
		if !res.OK || res.Value != 14 {
			t.Errorf("expected value 14, got %+v", res)
		}

		// Verify with get
		stdoutGet, _, codeGet := runCLI(t, home, "config", "get", "search.decay_half_life_days")
		if codeGet != 0 {
			t.Fatalf("config get after set failed (exit %d)", codeGet)
		}
		var resGet struct {
			Value int `json:"value"`
		}
		_ = json.Unmarshal([]byte(stdoutGet), &resGet)
		if resGet.Value != 14 {
			t.Errorf("expected persisted value 14, got %d", resGet.Value)
		}
	})

	// 3. Set invalid (negative) decay days rejected
	t.Run("config_set_negative_decay_days_rejected", func(t *testing.T) {
		_, stderr, code := runCLI(t, home, "config", "set", "search.decay_half_life_days", "-10")
		if code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
		if !strings.Contains(stderr, "must be >= 0") && !strings.Contains(stderr, "decay_half_life_days") {
			t.Errorf("stderr %q should mention validation error >= 0", stderr)
		}
	})

	// 4. CLI recall end-to-end rank reordering with CENTMEM_SEARCH_DECAY_HALF_LIFE_DAYS
	t.Run("cli_recall_env_override_reorders_ranks", func(t *testing.T) {
		dbPath := filepath.Join(home, "centmem.db")
		db, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			t.Fatalf("open sqlite: %v", err)
		}
		defer db.Close()

		// Memory 1: Older memory (30 days old), high keyword match
		stdout1, _, code1 := runCLI(t, home, "put", "--type", "note", "--scope", "global",
			"--content", "kubernetes cluster orchestration service mesh istio gateway ingress")
		if code1 != 0 {
			t.Fatalf("put old failed")
		}
		var r1 struct{ ID int64 `json:"id"` }
		_ = json.Unmarshal([]byte(stdout1), &r1)

		// Backdate memory 1 by 30 days
		past30 := time.Now().Add(-30 * 24 * time.Hour).UnixMicro()
		if _, err := db.Exec("UPDATE memories SET created_at = ?, updated_at = ? WHERE id = ?", past30, past30, r1.ID); err != nil {
			t.Fatalf("update past30: %v", err)
		}

		// Memory 2: Brand new memory (10 minutes old), lower keyword match
		stdout2, _, code2 := runCLI(t, home, "put", "--type", "note", "--scope", "global",
			"--content", "kubernetes cluster quick note")
		if code2 != 0 {
			t.Fatalf("put new failed")
		}
		var r2 struct{ ID int64 `json:"id"` }
		_ = json.Unmarshal([]byte(stdout2), &r2)

		// Reset config file to 0 days (disabled)
		runCLI(t, home, "config", "set", "search.decay_half_life_days", "0")

		// Query via CLI recall without env decay: older high-score memory must rank #1
		outNoDecay, _, codeRecall1 := runCLI(t, home, "recall", "kubernetes cluster orchestration service mesh istio")
		if codeRecall1 != 0 {
			t.Fatalf("recall without decay failed")
		}
		var resNoDecay struct {
			OK      bool `json:"ok"`
			Results []struct {
				ID    int64   `json:"id"`
				Score float64 `json:"score"`
			} `json:"results"`
		}
		if err := json.Unmarshal([]byte(outNoDecay), &resNoDecay); err != nil {
			t.Fatalf("unmarshal recall output: %v (stdout=%s)", err, outNoDecay)
		}
		if len(resNoDecay.Results) < 2 {
			t.Fatalf("expected at least 2 results, got %d", len(resNoDecay.Results))
		}
		if resNoDecay.Results[0].ID != r1.ID {
			t.Errorf("without decay: expected older memory %d at rank #1, got %d", r1.ID, resNoDecay.Results[0].ID)
		}

		// Now override via environment variable CENTMEM_SEARCH_DECAY_HALF_LIFE_DAYS=7
		t.Setenv("CENTMEM_SEARCH_DECAY_HALF_LIFE_DAYS", "7")
		outDecay, _, codeRecall2 := runCLI(t, home, "recall", "kubernetes cluster orchestration service mesh istio")
		if codeRecall2 != 0 {
			t.Fatalf("recall with decay env failed")
		}
		var resDecay struct {
			OK      bool `json:"ok"`
			Results []struct {
				ID    int64   `json:"id"`
				Score float64 `json:"score"`
			} `json:"results"`
		}
		if err := json.Unmarshal([]byte(outDecay), &resDecay); err != nil {
			t.Fatalf("unmarshal recall with decay output: %v", err)
		}
		if len(resDecay.Results) < 2 {
			t.Fatalf("expected at least 2 results with decay, got %d", len(resDecay.Results))
		}
		// Newer memory (r2.ID) must overtake older memory (r1.ID)
		if resDecay.Results[0].ID != r2.ID {
			t.Errorf("with CENTMEM_SEARCH_DECAY_HALF_LIFE_DAYS=7: expected newer memory %d at rank #1, got %d (scores: #1=%f, #2=%f)",
				r2.ID, resDecay.Results[0].ID, resDecay.Results[0].Score, resDecay.Results[1].Score)
		}
		if resDecay.Results[1].ID != r1.ID {
			t.Errorf("with decay: expected older memory %d at rank #2, got %d", r1.ID, resDecay.Results[1].ID)
		}
	})
}
