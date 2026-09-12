package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/embed"
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

func TestCLI_Compact_DryRun(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")
	runCLI(t, home, "put", "--scope", "project:cent-mem", "--type", "note", "--content", "alpha note")
	runCLI(t, home, "put", "--scope", "project:cent-mem", "--type", "log", "--content", "alpha log")

	stdout, _, code := runCLI(t, home, "compact", "--dry-run")
	if code != 0 {
		t.Fatalf("compact --dry-run exit code = %d, want 0", code)
	}
	m := parseJSON(t, stdout)
	if m["ok"] != true {
		t.Errorf("ok = %v, want true", m["ok"])
	}
	if m["dry_run"] != true {
		t.Errorf("dry_run = %v, want true", m["dry_run"])
	}
	for _, k := range []string{"summarized", "archived"} {
		if _, ok := m[k].(float64); !ok {
			t.Errorf("expected numeric %s field, got %v", k, m[k])
		}
	}
	if ids, ok := m["new_memory_ids"].([]any); !ok {
		t.Errorf("expected new_memory_ids array, got %T", m["new_memory_ids"])
	} else if len(ids) != 0 {
		t.Errorf("expected no new ids on fresh store, got %v", ids)
	}
}

func TestCLI_Doctor(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	stdout, _, code := runCLI(t, home, "doctor")
	if code != 0 {
		t.Fatalf("doctor exit code = %d, want 0", code)
	}
	m := parseJSON(t, stdout)
	if m["ok"] != true {
		t.Errorf("ok = %v, want true", m["ok"])
	}
	checks, ok := m["checks"].([]any)
	if !ok || len(checks) == 0 {
		t.Fatalf("expected checks array, got %v", m["checks"])
	}
	names := map[string]bool{}
	for _, c := range checks {
		cm := c.(map[string]any)
		name := cm["name"].(string)
		names[name] = true
		if name == "ai_agent" {
			if cm["status"] != "ok" && cm["status"] != "warn" {
				t.Errorf("check ai_agent status = %v, want ok or warn", cm["status"])
			}
			continue
		}
		if cm["status"] != "ok" {
			t.Errorf("check %v status = %v, want ok", cm["name"], cm["status"])
		}
	}
	for _, want := range []string{"integrity", "schema_version", "extensions", "model", "embed_queue", "permissions", "ai_agent"} {
		if !names[want] {
			t.Errorf("doctor missing check %q", want)
		}
	}
}

func TestCLI_Backup(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")
	runCLI(t, home, "put", "--scope", "global", "--type", "note", "--content", "backup me")

	backupPath := filepath.Join(home, "backup.db")
	stdout, _, code := runCLI(t, home, "backup", "--to", backupPath)
	if code != 0 {
		t.Fatalf("backup exit code = %d, want 0", code)
	}
	m := parseJSON(t, stdout)
	if m["ok"] != true {
		t.Errorf("ok = %v, want true", m["ok"])
	}
	if m["backup"] != backupPath {
		t.Errorf("backup = %v, want %v", m["backup"], backupPath)
	}
	if _, ok := m["size_mb"].(float64); !ok {
		t.Errorf("expected size_mb field, got %v", m["size_mb"])
	}
	if info, err := os.Stat(backupPath); err != nil || info.Size() == 0 {
		t.Errorf("expected non-empty backup file: %v", err)
	}
}

func TestDoctor_ModelMissing(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")
	// Remove the model file; doctor should report it and exit 1.
	if err := os.Remove(filepath.Join(home, "models", "bge-small-en-v1.5.onnx")); err != nil {
		t.Fatalf("remove model: %v", err)
	}

	stdout, _, code := runCLI(t, home, "doctor")
	if code != 1 {
		t.Fatalf("doctor exit code = %d, want 1", code)
	}
	m := parseJSON(t, stdout)
	if m["ok"] != false {
		t.Errorf("ok = %v, want false", m["ok"])
	}
	found := false
	for _, c := range m["checks"].([]any) {
		cm := c.(map[string]any)
		if cm["name"] == "model" && cm["status"] == "fail" {
			found = true
			if !strings.Contains(cm["detail"].(string), "init") {
				t.Errorf("model detail %q should mention init", cm["detail"])
			}
		}
	}
	if !found {
		t.Errorf("expected model check to fail")
	}
}

func TestDoctor_CorruptDB(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")
	// Overwrite the DB with garbage so it cannot be opened.
	if err := os.WriteFile(filepath.Join(home, "centmem.db"), []byte("not a sqlite database"), 0600); err != nil {
		t.Fatalf("corrupt db: %v", err)
	}
	_, stderr, code := runCLI(t, home, "doctor")
	if code != 1 {
		t.Fatalf("doctor exit code = %d, want 1\nstderr: %s", code, stderr)
	}
}

func TestDoctor_AIAgentStates(t *testing.T) {
	stubDownloader()

	t.Run("disabled_in_config", func(t *testing.T) {
		home := newHome(t)
		runCLI(t, home, "init")
		runCLI(t, home, "config", "set", "agent.enabled", "false")

		stdout, _, code := runCLI(t, home, "doctor")
		if code != 0 {
			t.Fatalf("doctor exit code = %d, want 0", code)
		}
		m := parseJSON(t, stdout)
		checks := m["checks"].([]any)
		var aiCheck map[string]any
		for _, c := range checks {
			cm := c.(map[string]any)
			if cm["name"] == "ai_agent" {
				aiCheck = cm
				break
			}
		}
		if aiCheck == nil {
			t.Fatalf("missing ai_agent check")
		}
		if aiCheck["status"] != "ok" || aiCheck["detail"] != "disabled in config" {
			t.Errorf("expected ok / disabled in config, got %+v", aiCheck)
		}
	})

	t.Run("connected_to_mock_llm", func(t *testing.T) {
		mockLLM := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[{"id":"mock-model"}]}`))
		}))
		defer mockLLM.Close()

		home := newHome(t)
		runCLI(t, home, "init")
		runCLI(t, home, "config", "set", "llm.endpoint", mockLLM.URL)
		runCLI(t, home, "config", "set", "llm.model", "mock-model")

		stdout, _, code := runCLI(t, home, "doctor")
		if code != 0 {
			t.Fatalf("doctor exit code = %d, want 0", code)
		}
		m := parseJSON(t, stdout)
		checks := m["checks"].([]any)
		var aiCheck map[string]any
		for _, c := range checks {
			cm := c.(map[string]any)
			if cm["name"] == "ai_agent" {
				aiCheck = cm
				break
			}
		}
		if aiCheck == nil {
			t.Fatalf("missing ai_agent check")
		}
		if aiCheck["status"] != "ok" {
			t.Errorf("expected status ok, got %v", aiCheck["status"])
		}
		detail, _ := aiCheck["detail"].(string)
		if !strings.Contains(detail, "mock-model") || !strings.Contains(detail, "endpoint=") {
			t.Errorf("expected detail to contain model and endpoint, got %q", detail)
		}
	})

	t.Run("unreachable_soft_warning", func(t *testing.T) {
		home := newHome(t)
		runCLI(t, home, "init")
		runCLI(t, home, "config", "set", "llm.endpoint", "http://127.0.0.1:59996/v1")

		stdout, _, code := runCLI(t, home, "doctor")
		if code != 0 {
			t.Fatalf("doctor exit code = %d, want 0 (soft warning)", code)
		}
		m := parseJSON(t, stdout)
		if m["ok"] != true {
			t.Errorf("expected ok=true (soft warning), got %v", m["ok"])
		}
		checks := m["checks"].([]any)
		var aiCheck map[string]any
		for _, c := range checks {
			cm := c.(map[string]any)
			if cm["name"] == "ai_agent" {
				aiCheck = cm
				break
			}
		}
		if aiCheck == nil {
			t.Fatalf("missing ai_agent check")
		}
		if aiCheck["status"] != "warn" {
			t.Errorf("expected status warn, got %v", aiCheck["status"])
		}
		warnings, _ := m["warnings"].([]any)
		if len(warnings) == 0 {
			t.Errorf("expected warnings array to be populated")
		}
	})
}

func TestBackup_RoundTrip(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")
	runCLI(t, home, "put", "--scope", "global", "--type", "note", "--content", "roundtrip memory")

	backupPath := filepath.Join(home, "snapshot.db")
	if _, _, code := runCLI(t, home, "backup", "--to", backupPath); code != 0 {
		t.Fatalf("backup failed")
	}
	// Modify the live DB after the backup.
	runCLI(t, home, "put", "--scope", "project:beta", "--type", "note", "--content", "after backup")

	// Restore.
	if _, _, code := runCLI(t, home, "restore", "--from", backupPath); code != 0 {
		t.Fatalf("restore failed")
	}

	// The "after backup" memory should be gone; the original should still recall.
	out, _, _ := runCLI(t, home, "recall", "roundtrip", "--scope", "global", "--top", "5")
	m := parseJSON(t, out)
	if results, ok := m["results"].([]any); !ok || len(results) == 0 {
		t.Errorf("expected roundtrip memory after restore, got %v", m["results"])
	}
}

func TestRestore_SafetyCopy(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")
	runCLI(t, home, "put", "--scope", "global", "--type", "note", "--content", "safety copy")

	backupPath := filepath.Join(home, "snapshot.db")
	if _, _, code := runCLI(t, home, "backup", "--to", backupPath); code != 0 {
		t.Fatalf("backup failed")
	}
	if _, _, code := runCLI(t, home, "restore", "--from", backupPath); code != 0 {
		t.Fatalf("restore failed")
	}
	if _, err := os.Stat(filepath.Join(home, "centmem.db.pre-restore.bak")); err != nil {
		t.Errorf("expected .pre-restore.bak safety copy: %v", err)
	}
}

func TestRestore_RejectsCorrupt(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	badPath := filepath.Join(home, "bad.db")
	if err := os.WriteFile(badPath, []byte("garbage not a db"), 0600); err != nil {
		t.Fatalf("write bad backup: %v", err)
	}
	_, _, code := runCLI(t, home, "restore", "--from", badPath)
	if code != 1 {
		t.Fatalf("restore exit code = %d, want 1", code)
	}
	// The live DB must be untouched.
	if _, err := os.Stat(filepath.Join(home, "centmem.db")); err != nil {
		t.Errorf("live db should remain: %v", err)
	}
}

func TestCLI_Recall_Importance_AccessCountAndStats(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	putOut, _, code := runCLI(t, home, "put", "--scope", "global", "--type", "note", "--content", "testing recall importance access tracking", "--tags", "test,importance")
	if code != 0 {
		t.Fatalf("put failed: %v", code)
	}
	putJSON := parseJSON(t, putOut)
	memID := int64(putJSON["id"].(float64))

	// First recall: access_count should be 0 before this recall, and last_accessed_at should be null
	stdout1, _, code := runCLI(t, home, "recall", "testing recall importance", "--scope", "global")
	if code != 0 {
		t.Fatalf("recall 1 failed: %d", code)
	}
	m1 := parseJSON(t, stdout1)
	results1, ok := m1["results"].([]any)
	if !ok || len(results1) == 0 {
		t.Fatalf("expected results in recall 1: %v", m1)
	}
	item1 := results1[0].(map[string]any)
	if int64(item1["id"].(float64)) != memID {
		t.Fatalf("expected id %d, got %v", memID, item1["id"])
	}
	if item1["access_count"].(float64) != 0 {
		t.Errorf("expected initial recall access_count = 0, got %v", item1["access_count"])
	}
	if item1["last_accessed_at"] != nil {
		t.Errorf("expected initial recall last_accessed_at = null, got %v", item1["last_accessed_at"])
	}

	// Second recall: after the first recall executed RecordAccessAsync and s.Close() waited,
	// access_count should now be 1 and last_accessed_at should be a non-nil unix timestamp
	stdout2, _, code := runCLI(t, home, "recall", "testing recall importance", "--scope", "global")
	if code != 0 {
		t.Fatalf("recall 2 failed: %d", code)
	}
	m2 := parseJSON(t, stdout2)
	results2 := m2["results"].([]any)
	item2 := results2[0].(map[string]any)
	if item2["access_count"].(float64) != 1 {
		t.Errorf("expected second recall access_count = 1, got %v", item2["access_count"])
	}
	if item2["last_accessed_at"] == nil {
		t.Errorf("expected second recall last_accessed_at != nil")
	}

	// Verify stats includes importance_distribution
	statsOut, _, code := runCLI(t, home, "stats")
	if code != 0 {
		t.Fatalf("stats failed: %d", code)
	}
	statsJSON := parseJSON(t, statsOut)
	dist, ok := statsJSON["importance_distribution"].(map[string]any)
	if !ok {
		t.Fatalf("expected importance_distribution object in stats: %v", statsJSON)
	}
	if dist["low_access_1_5"].(float64) < 1 {
		t.Errorf("expected low_access_1_5 >= 1, got %v", dist["low_access_1_5"])
	}
}
