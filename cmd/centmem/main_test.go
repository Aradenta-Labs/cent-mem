package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/farras/cent-mem/internal/embed"
)

// newHome creates a temp CENTMEM home and sets env for the process.
func newHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("CENTMEM_HOME", home)
	t.Setenv("CENTMEM_DB", filepath.Join(home, "centmem.db"))
	return home
}

// runCLI executes a command within a given home, capturing stdout/stderr/code.
func runCLI(t *testing.T, home string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	t.Setenv("CENTMEM_HOME", home)
	t.Setenv("CENTMEM_DB", filepath.Join(home, "centmem.db"))

	oldOut, oldErr := os.Stdout, os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr
	osStderr = wErr

	code = run(args)

	wOut.Close()
	wErr.Close()
	os.Stdout = oldOut
	os.Stderr = oldErr
	osStderr = oldErr

	outBytes, _ := io.ReadAll(rOut)
	errBytes, _ := io.ReadAll(rErr)
	return string(outBytes), string(errBytes), code
}

// stubDownloader replaces the model downloader with one that writes a tiny
// file (skipping real network + sha256 verification).
func stubDownloader() {
	embed.Downloader = func(name, modelPath string) (string, error) {
		if err := os.MkdirAll(filepath.Dir(modelPath), 0755); err != nil {
			return "", err
		}
		if err := os.WriteFile(modelPath, []byte("fake-model-bytes"), 0644); err != nil {
			return "", err
		}
		return modelPath, nil
	}
}

// parseJSON parses stdout as JSON into a map.
func parseJSON(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("stdout is not valid JSON object: %v\nraw: %s", err, s)
	}
	return m
}

func TestCLI_Init(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	stdout, _, code := runCLI(t, home, "init")
	if code != 0 {
		t.Fatalf("init exit code = %d, want 0", code)
	}
	m := parseJSON(t, stdout)
	if m["ok"] != true {
		t.Errorf("ok = %v, want true", m["ok"])
	}
	if m["model"] != "bge-small-en-v1.5" {
		t.Errorf("model = %v, want bge-small-en-v1.5", m["model"])
	}
	if m["dims"] != float64(384) {
		t.Errorf("dims = %v, want 384", m["dims"])
	}
	if _, ok := m["db"]; !ok {
		t.Errorf("expected db field in output")
	}
	if _, err := os.Stat(filepath.Join(home, "models", "bge-small-en-v1.5.onnx")); err != nil {
		t.Errorf("expected model file present after init: %v", err)
	}
}

func TestCLI_Put_Note(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	stdout, _, code := runCLI(t, home, "put", "--scope", "project:cent-mem", "--type", "note", "--content", "hello note", "--tags", "a,b")
	if code != 0 {
		t.Fatalf("put exit code = %d, want 0", code)
	}
	m := parseJSON(t, stdout)
	if m["ok"] != true {
		t.Errorf("ok = %v, want true", m["ok"])
	}
	if m["status"] != "queued" {
		t.Errorf("status = %v, want queued", m["status"])
	}
	if _, ok := m["id"].(float64); !ok {
		t.Errorf("expected numeric id, got %v", m["id"])
	}
}

func TestCLI_Set_Fact(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	_, _, code := runCLI(t, home, "set", "--scope", "project:cent-mem", "--key", "user.timezone", "--value", `"Asia/Jakarta"`)
	if code != 0 {
		t.Fatalf("set exit code = %d, want 0", code)
	}
	stdout2, _, code := runCLI(t, home, "set", "--scope", "project:cent-mem", "--key", "user.timezone", "--value", `"Asia/Jakarta"`)
	if code != 0 {
		t.Fatalf("set#2 exit code = %d, want 0", code)
	}
	m := parseJSON(t, stdout2)
	if m["status"] != "updated" {
		t.Errorf("repeat status = %v, want updated", m["status"])
	}
}

func TestCLI_Get_Found(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")
	runCLI(t, home, "set", "--scope", "project:cent-mem", "--key", "user.timezone", "--value", `"Asia/Jakarta"`)

	stdout, _, code := runCLI(t, home, "get", "--scope", "project:cent-mem", "--key", "user.timezone")
	if code != 0 {
		t.Fatalf("get exit code = %d, want 0", code)
	}
	m := parseJSON(t, stdout)
	if m["value"] != "Asia/Jakarta" {
		t.Errorf("value = %v, want Asia/Jakarta", m["value"])
	}
}

func TestCLI_Get_NotFound(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	_, stderr, code := runCLI(t, home, "get", "--scope", "project:cent-mem", "--key", "missing.key")
	if code != 2 {
		t.Fatalf("get not-found exit code = %d, want 2", code)
	}
	var em struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(stderr), &em); err != nil {
		t.Fatalf("stderr is not valid JSON error: %v\nraw: %s", err, stderr)
	}
	if em.Error.Code != "NOT_FOUND" {
		t.Errorf("error code = %q, want NOT_FOUND", em.Error.Code)
	}
}

func TestCLI_Recall_Keyword(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")
	runCLI(t, home, "put", "--scope", "global", "--type", "note", "--content", "we deploy via github actions")

	stdout, _, code := runCLI(t, home, "recall", "github", "--scope", "project:cent-mem", "--top", "5")
	if code != 0 {
		t.Fatalf("recall exit code = %d, want 0", code)
	}
	m := parseJSON(t, stdout)
	results, ok := m["results"].([]any)
	if !ok {
		t.Fatalf("expected results array, got %T", m["results"])
	}
	if len(results) == 0 {
		t.Fatal("expected at least one recall result")
	}
	first := results[0].(map[string]any)
	if first["scope"] != "global" {
		t.Errorf("inherited scope = %v, want global", first["scope"])
	}
	if matched, ok := first["matched_by"].([]any); !ok || len(matched) == 0 {
		t.Errorf("expected matched_by array, got %v", first["matched_by"])
	}
}

func TestCLI_UnknownCommand(t *testing.T) {
	home := newHome(t)
	_, _, code := runCLI(t, home, "frobnicate")
	if code != 1 {
		t.Fatalf("unknown command exit code = %d, want 1", code)
	}
}

func TestCLI_Pretty(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")
	plain, _, code := runCLI(t, home, "stats")
	if code != 0 {
		t.Fatal("stats failed")
	}
	pretty, _, code := runCLI(t, home, "stats", "--pretty")
	if code != 0 {
		t.Fatal("stats --pretty failed")
	}
	if plain == pretty {
		t.Error("expected pretty output to differ from compact JSON")
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(plain), &m); err != nil {
		t.Errorf("compact stats not valid JSON: %v", err)
	}
}

func TestCLI_Forget(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")
	stdout, _, _ := runCLI(t, home, "put", "--scope", "global", "--type", "note", "--content", "to be forgotten")
	m := parseJSON(t, stdout)
	id := int64(m["id"].(float64))

	out, _, code := runCLI(t, home, "forget", "--id", strconv.FormatInt(id, 10))
	if code != 0 {
		t.Fatalf("forget exit code = %d, want 0", code)
	}
	fm := parseJSON(t, out)
	if fm["deleted"] != float64(1) {
		t.Errorf("deleted = %v, want 1", fm["deleted"])
	}
}
