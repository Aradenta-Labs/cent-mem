package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI_Export_OutputFile(t *testing.T) {
	home := newHome(t)
	stubDownloader()

	_, _, code := runCLI(t, home, "put", "--scope", "project:cli-export", "--type", "note", "--content", "Note for export 1", "--tags", "tag1")
	if code != 0 {
		t.Fatalf("put 1 failed with code %d", code)
	}
	_, _, code = runCLI(t, home, "put", "--scope", "project:cli-export", "--type", "note", "--content", "Note for export 2", "--tags", "tag2")
	if code != 0 {
		t.Fatalf("put 2 failed with code %d", code)
	}

	outFile := filepath.Join(t.TempDir(), "export.json")
	stdout, stderr, code := runCLI(t, home, "export", "--scope", "project:cli-export", "--output", outFile)
	if code != 0 {
		t.Fatalf("export failed with code %d: stderr=%s", code, stderr)
	}

	var status struct {
		Ok        bool   `json:"ok"`
		File      string `json:"file"`
		Total     int    `json:"total"`
		SizeBytes int64  `json:"size_bytes"`
	}
	if err := json.Unmarshal([]byte(stdout), &status); err != nil {
		t.Fatalf("unmarshal status envelope failed: %v\nstdout: %s", err, stdout)
	}
	if !status.Ok || status.Total != 2 || status.File != outFile || status.SizeBytes <= 0 {
		t.Fatalf("unexpected export status: %+v", status)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read exported file failed: %v", err)
	}
	var env struct {
		Format        string `json:"format"`
		FormatVersion int    `json:"format_version"`
		Total         int    `json:"total"`
		Memories      []any  `json:"memories"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("unmarshal exported envelope failed: %v", err)
	}
	if env.Format != "centmem-export" || env.FormatVersion != 1 || env.Total != 2 || len(env.Memories) != 2 {
		t.Fatalf("unexpected envelope content: %+v", env)
	}
}

func TestCLI_Export_Stdout(t *testing.T) {
	home := newHome(t)
	stubDownloader()

	_, _, code := runCLI(t, home, "put", "--scope", "project:cli-stdout", "--type", "note", "--content", "Note for stdout export")
	if code != 0 {
		t.Fatalf("put failed with code %d", code)
	}

	stdout, stderr, code := runCLI(t, home, "export", "--scope", "project:cli-stdout")
	if code != 0 {
		t.Fatalf("export failed with code %d: stderr=%s", code, stderr)
	}

	var env struct {
		Format        string `json:"format"`
		FormatVersion int    `json:"format_version"`
		Total         int    `json:"total"`
		Memories      []any  `json:"memories"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal stdout export failed: %v\nstdout: %s", err, stdout)
	}
	if env.Format != "centmem-export" || env.Total != 1 {
		t.Fatalf("unexpected stdout envelope: %+v", env)
	}

	// Verify stderr received status envelope
	var stStderr struct {
		Ok    bool `json:"ok"`
		Total int  `json:"total"`
	}
	if err := json.Unmarshal([]byte(stderr), &stStderr); err != nil {
		t.Fatalf("unmarshal stderr status failed: %v\nstderr: %s", err, stderr)
	}
	if !stStderr.Ok || stStderr.Total != 1 {
		t.Fatalf("unexpected stderr status: %+v", stStderr)
	}
}

func TestCLI_Export_CSV(t *testing.T) {
	home := newHome(t)
	stubDownloader()

	_, _, code := runCLI(t, home, "put", "--scope", "project:cli-csv", "--type", "note", "--content", "Note for CSV export")
	if code != 0 {
		t.Fatalf("put failed with code %d", code)
	}

	outFile := filepath.Join(t.TempDir(), "export.csv")
	stdout, stderr, code := runCLI(t, home, "export", "--scope", "project:cli-csv", "--format", "csv", "--output", outFile)
	if code != 0 {
		t.Fatalf("export csv failed with code %d: stderr=%s", code, stderr)
	}

	var status struct {
		Ok    bool `json:"ok"`
		Total int  `json:"total"`
	}
	if err := json.Unmarshal([]byte(stdout), &status); err != nil {
		t.Fatalf("unmarshal status failed: %v\nstdout: %s", err, stdout)
	}
	if !status.Ok || status.Total != 1 {
		t.Fatalf("unexpected status: %+v", status)
	}

	content, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read export.csv failed: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 CSV lines (header + 1 row), got %d:\n%s", len(lines), string(content))
	}
	if !strings.HasPrefix(lines[0], "id,scope,type,content,key,value_json,tags") {
		t.Fatalf("unexpected CSV header: %s", lines[0])
	}
}

func TestCLI_Import_Success_And_Idempotent(t *testing.T) {
	// 1. Export from source home
	sourceHome := newHome(t)
	stubDownloader()

	_, _, code := runCLI(t, sourceHome, "put", "--scope", "project:roundtrip", "--type", "note", "--content", "Memory A")
	if code != 0 {
		t.Fatalf("put A failed: %d", code)
	}
	_, _, code = runCLI(t, sourceHome, "put", "--scope", "project:roundtrip/agent:bot", "--type", "note", "--content", "Memory B")
	if code != 0 {
		t.Fatalf("put B failed: %d", code)
	}

	dumpFile := filepath.Join(t.TempDir(), "dump.json")
	_, _, code = runCLI(t, sourceHome, "export", "--scope", "project:roundtrip", "--output", dumpFile)
	if code != 0 {
		t.Fatalf("export failed: %d", code)
	}

	// 2. Import into a new target home
	targetHome := newHome(t)

	stdout, stderr, code := runCLI(t, targetHome, "import", dumpFile)
	if code != 0 {
		t.Fatalf("import failed with code %d: stderr=%s", code, stderr)
	}

	var rep struct {
		Ok       bool   `json:"ok"`
		File     string `json:"file"`
		Total    int    `json:"total"`
		Imported int    `json:"imported"`
		Skipped  int    `json:"skipped"`
		Failed   int    `json:"failed"`
	}
	if err := json.Unmarshal([]byte(stdout), &rep); err != nil {
		t.Fatalf("unmarshal import output failed: %v\nstdout: %s", err, stdout)
	}
	if !rep.Ok || rep.Total != 2 || rep.Imported != 2 || rep.Skipped != 0 || rep.Failed != 0 {
		t.Fatalf("unexpected first import result: %+v", rep)
	}

	// 3. Second run: idempotently skipped
	stdout2, stderr2, code := runCLI(t, targetHome, "import", dumpFile)
	if code != 0 {
		t.Fatalf("second import failed with code %d: stderr=%s", code, stderr2)
	}
	var rep2 struct {
		Ok       bool `json:"ok"`
		Total    int  `json:"total"`
		Imported int  `json:"imported"`
		Skipped  int  `json:"skipped"`
		Failed   int  `json:"failed"`
	}
	if err := json.Unmarshal([]byte(stdout2), &rep2); err != nil {
		t.Fatalf("unmarshal second import failed: %v\nstdout: %s", err, stdout2)
	}
	if !rep2.Ok || rep2.Total != 2 || rep2.Imported != 0 || rep2.Skipped != 2 || rep2.Failed != 0 {
		t.Fatalf("unexpected second import result: %+v", rep2)
	}
}

func TestCLI_Import_DryRun(t *testing.T) {
	sourceHome := newHome(t)
	stubDownloader()

	_, _, code := runCLI(t, sourceHome, "put", "--scope", "project:dryrun", "--type", "note", "--content", "Dry run memory")
	if code != 0 {
		t.Fatalf("put failed: %d", code)
	}

	dumpFile := filepath.Join(t.TempDir(), "dryrun.json")
	_, _, code = runCLI(t, sourceHome, "export", "--scope", "project:dryrun", "--output", dumpFile)
	if code != 0 {
		t.Fatalf("export failed: %d", code)
	}

	targetHome := newHome(t)
	stdout, stderr, code := runCLI(t, targetHome, "import", dumpFile, "--dry-run")
	if code != 0 {
		t.Fatalf("dry-run import failed with code %d: stderr=%s", code, stderr)
	}

	var rep struct {
		Ok       bool `json:"ok"`
		DryRun   bool `json:"dry_run"`
		Total    int  `json:"total"`
		Imported int  `json:"imported"`
		Skipped  int  `json:"skipped"`
		Failed   int  `json:"failed"`
	}
	if err := json.Unmarshal([]byte(stdout), &rep); err != nil {
		t.Fatalf("unmarshal dry-run output failed: %v\nstdout: %s", err, stdout)
	}
	if !rep.Ok || !rep.DryRun || rep.Total != 1 || rep.Imported != 1 {
		t.Fatalf("unexpected dry-run report: %+v", rep)
	}

	// Verify target store is still empty
	stdoutCheck, _, code := runCLI(t, targetHome, "list", "--scope", "project:dryrun")
	if code != 0 {
		t.Fatalf("list failed with code %d", code)
	}
	var listCheck struct {
		Memories []any `json:"memories"`
	}
	_ = json.Unmarshal([]byte(stdoutCheck), &listCheck)
	if len(listCheck.Memories) != 0 {
		t.Fatalf("target store should have 0 memories after dry-run, got %d", len(listCheck.Memories))
	}
}

func TestCLI_Import_Stdin(t *testing.T) {
	targetHome := newHome(t)
	stubDownloader()

	exportJSON := `{
  "format": "centmem-export",
  "format_version": 1,
  "exported_at": "2026-09-14T10:00:00Z",
  "scope": "project:stdin-test",
  "total": 1,
  "memories": [
    {
      "scope": "project:stdin-test",
      "type": "note",
      "content": "Piped via stdin"
    }
  ]
}`

	stdout, stderr, code := runCLIWithStdin(t, targetHome, exportJSON, "import", "-")
	if code != 0 {
		t.Fatalf("import from stdin failed with code %d: stderr=%s", code, stderr)
	}

	var rep struct {
		Ok       bool `json:"ok"`
		Total    int  `json:"total"`
		Imported int  `json:"imported"`
		Skipped  int  `json:"skipped"`
	}
	if err := json.Unmarshal([]byte(stdout), &rep); err != nil {
		t.Fatalf("unmarshal stdin import output failed: %v\nstdout: %s", err, stdout)
	}
	if !rep.Ok || rep.Total != 1 || rep.Imported != 1 {
		t.Fatalf("unexpected stdin import result: %+v", rep)
	}
}

func TestCLI_Import_RejectBadCSV(t *testing.T) {
	home := newHome(t)
	badCSVFile := filepath.Join(t.TempDir(), "bad.csv")
	_ = os.WriteFile(badCSVFile, []byte("id,scope,type,content\n1,global,note,hi\n"), 0600)

	_, stderr, code := runCLI(t, home, "import", badCSVFile)
	if code != 1 {
		t.Fatalf("expected code 1 for CSV import, got %d", code)
	}
	if !strings.Contains(stderr, "unsupported file format") {
		t.Fatalf("expected error to mention unsupported file format, got stderr:\n%s", stderr)
	}
}

func TestCLI_Registry_ExportImport(t *testing.T) {
	reg := buildRegistry()
	if !reg.Has("export") {
		t.Error("registry missing export command")
	}
	if !reg.Has("import") {
		t.Error("registry missing import command")
	}

	exp, ok := commands["export"]
	if !ok {
		t.Fatal("commands map missing export")
	}
	for _, expectedFlag := range []string{"--scope", "--format", "--type", "--tags", "--since", "--until", "--agent", "--output"} {
		found := false
		for _, f := range exp.flags {
			if f == expectedFlag {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("export missing flag %s", expectedFlag)
		}
	}

	imp, ok := commands["import"]
	if !ok {
		t.Fatal("commands map missing import")
	}
	found := false
	for _, f := range imp.flags {
		if f == "--dry-run" {
			found = true
			break
		}
	}
	if !found {
		t.Error("import missing --dry-run flag")
	}
}

func TestCLI_Export_NestedDirectory(t *testing.T) {
	home := newHome(t)
	stubDownloader()

	_, _, code := runCLI(t, home, "put", "--scope", "project:nested", "--type", "note", "--content", "Note for nested dir export")
	if code != 0 {
		t.Fatalf("put failed: %d", code)
	}

	nestedFile := filepath.Join(t.TempDir(), "deep", "sub", "dir", "export.json")
	stdout, stderr, code := runCLI(t, home, "export", "--scope", "project:nested", "--output", nestedFile)
	if code != 0 {
		t.Fatalf("export to nested dir failed (code %d): %s", code, stderr)
	}

	var st struct {
		Ok        bool   `json:"ok"`
		File      string `json:"file"`
		Total     int    `json:"total"`
		SizeBytes int64  `json:"size_bytes"`
	}
	if err := json.Unmarshal([]byte(stdout), &st); err != nil {
		t.Fatalf("unmarshal stdout failed: %v", err)
	}
	if !st.Ok || st.Total != 1 || st.SizeBytes <= 0 {
		t.Fatalf("unexpected export status: %+v", st)
	}
	if _, err := os.Stat(nestedFile); err != nil {
		t.Fatalf("nested file does not exist: %v", err)
	}
}

func TestCLI_Import_PartialFailure(t *testing.T) {
	targetHome := newHome(t)
	stubDownloader()

	exportJSON := `{
  "format": "centmem-export",
  "format_version": 1,
  "exported_at": "2026-09-14T10:00:00Z",
  "scope": "project:partial-cli",
  "total": 2,
  "memories": [
    {
      "scope": "project:partial-cli",
      "type": "note",
      "content": "Valid note"
    },
    {
      "scope": "invalid ::: scope",
      "type": "note",
      "content": "Invalid scope note"
    }
  ]
}`

	stdout, stderr, code := runCLIWithStdin(t, targetHome, exportJSON, "import", "-")
	if code != 0 {
		t.Fatalf("expected code 0 reporting partial failure, got %d: stderr=%s", code, stderr)
	}

	var rep struct {
		Ok       bool     `json:"ok"`
		Total    int      `json:"total"`
		Imported int      `json:"imported"`
		Failed   int      `json:"failed"`
		Errors   []string `json:"errors"`
	}
	if err := json.Unmarshal([]byte(stdout), &rep); err != nil {
		t.Fatalf("unmarshal import output: %v", err)
	}
	if rep.Ok || rep.Total != 2 || rep.Imported != 1 || rep.Failed != 1 || len(rep.Errors) != 1 {
		t.Fatalf("unexpected partial import response: %+v", rep)
	}
}
