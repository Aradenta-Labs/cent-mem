package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCLI_Config_MissingSubcommand(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	_, stderr, code := runCLI(t, home, "config")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "missing config subcommand") {
		t.Errorf("stderr %q should mention missing config subcommand", stderr)
	}
}

func TestCLI_ConfigSet_ValidKey(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	stdout, stderr, code := runCLI(t, home, "config", "set", "capture.harness", "claude-code")
	if code != 0 {
		t.Fatalf("set failed (exit %d): stderr=%s", code, stderr)
	}

	var res struct {
		OK    bool   `json:"ok"`
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("unmarshal stdout: %v (stdout=%s)", err, stdout)
	}
	if !res.OK || res.Key != "capture.harness" || res.Value != "claude-code" {
		t.Errorf("unexpected set output: %+v", res)
	}

	// Verify with get
	stdoutGet, _, codeGet := runCLI(t, home, "config", "get", "capture.harness")
	if codeGet != 0 {
		t.Fatalf("get failed (exit %d)", codeGet)
	}
	var resGet struct {
		OK    bool   `json:"ok"`
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal([]byte(stdoutGet), &resGet); err != nil {
		t.Fatalf("unmarshal get stdout: %v", err)
	}
	if resGet.Value != "claude-code" {
		t.Errorf("expected claude-code, got %q", resGet.Value)
	}
}

func TestCLI_ConfigSet_Categories(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	stdout, stderr, code := runCLI(t, home, "config", "set", "capture.categories", "decision,fact,preference")
	if code != 0 {
		t.Fatalf("set categories failed (exit %d): %s", code, stderr)
	}

	var res struct {
		OK    bool     `json:"ok"`
		Key   string   `json:"key"`
		Value []string `json:"value"`
	}
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}
	if len(res.Value) != 3 || res.Value[0] != "decision" {
		t.Errorf("unexpected categories: %+v", res.Value)
	}
}

func TestCLI_ConfigSet_InvalidKey(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	_, stderr, code := runCLI(t, home, "config", "set", "invalid.key", "value")
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr, "unknown config key") {
		t.Errorf("stderr %q should mention unknown config key", stderr)
	}
}

func TestCLI_ConfigSet_InvalidValue(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	_, stderr, code := runCLI(t, home, "config", "set", "capture.confidence_threshold", "2.5")
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr, "confidence_threshold") {
		t.Errorf("stderr %q should mention confidence_threshold validation", stderr)
	}
}

func TestCLI_ConfigGet_SingleKey(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	stdout, stderr, code := runCLI(t, home, "config", "get", "capture.backend")
	if code != 0 {
		t.Fatalf("config get failed (exit %d): %s", code, stderr)
	}

	var res struct {
		OK    bool   `json:"ok"`
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("unmarshal stdout: %v", err)
	}
	if !res.OK || res.Key != "capture.backend" || res.Value != "heuristic" {
		t.Errorf("unexpected get result: %+v", res)
	}
}

func TestCLI_ConfigGet_All(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	stdout, stderr, code := runCLI(t, home, "config", "get")
	if code != 0 {
		t.Fatalf("config get all failed (exit %d): %s", code, stderr)
	}

	var res struct {
		OK     bool `json:"ok"`
		Config struct {
			Capture struct {
				Harness string `json:"harness"`
				Backend string `json:"backend"`
			} `json:"capture"`
			Retention struct {
				NoteSummarizeAfterDays int `json:"note_summarize_after_days"`
			} `json:"retention"`
		} `json:"config"`
	}
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("unmarshal all config: %v", err)
	}
	if !res.OK || res.Config.Capture.Backend == "" || res.Config.Retention.NoteSummarizeAfterDays == 0 {
		t.Errorf("unexpected full config result: %+v", res)
	}
}

func TestCLI_ConfigGet_NotFound(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	_, stderr, code := runCLI(t, home, "config", "get", "nonexistent.key")
	if code != 2 {
		t.Fatalf("expected exit code 2 (NOT_FOUND), got %d (stderr=%s)", code, stderr)
	}
	if !strings.Contains(stderr, "NOT_FOUND") {
		t.Errorf("stderr %q should contain NOT_FOUND", stderr)
	}
}
