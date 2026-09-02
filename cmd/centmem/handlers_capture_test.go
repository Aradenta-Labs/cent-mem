package main

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/capture"
)

func runCLIWithStdin(t *testing.T, home string, stdinContent string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	t.Setenv("CENTMEM_HOME", home)
	t.Setenv("CENTMEM_DB", filepath.Join(home, "centmem.db"))

	oldIn, oldOut, oldErr := os.Stdin, os.Stdout, os.Stderr
	rIn, wIn, _ := os.Pipe()
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()

	os.Stdin = rIn
	os.Stdout = wOut
	os.Stderr = wErr
	osStderr = wErr

	go func() {
		_, _ = wIn.Write([]byte(stdinContent))
		_ = wIn.Close()
	}()

	code = run(args)

	_ = rIn.Close()
	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdin = oldIn
	os.Stdout = oldOut
	os.Stderr = oldErr
	osStderr = oldErr

	outBytes, _ := io.ReadAll(rOut)
	errBytes, _ := io.ReadAll(rErr)
	return string(outBytes), string(errBytes), code
}

func TestCLI_Capture_MissingSubcommand(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	_, stderr, code := runCLI(t, home, "capture")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "missing capture subcommand") {
		t.Errorf("stderr %q should mention missing capture subcommand", stderr)
	}
}

func TestCLI_Capture_UnknownSubcommand(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	_, stderr, code := runCLI(t, home, "capture", "unknownsub")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "unknown capture subcommand") {
		t.Errorf("stderr %q should mention unknown capture subcommand", stderr)
	}
}

func TestCLI_Capture_Help(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	_, stderr, code := runCLI(t, home, "capture", "--help")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stderr, "Usage: centmem capture") {
		t.Errorf("stderr %q should contain usage text", stderr)
	}
}

func TestCLI_Capture_Run_File(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	rawMsgs := []capture.TranscriptMessage{
		{Role: "user", Content: "Let's pick our tech stack."},
		{Role: "assistant", Content: "We decided to use SQLite-vec for vector embeddings."},
		{Role: "assistant", Content: "Here is the config:\n```go\nvar Dims = 384\n```"},
		{Role: "assistant", Content: "Completed initialization of vector store."},
	}
	var sb strings.Builder
	for _, m := range rawMsgs {
		b, _ := json.Marshal(m)
		sb.Write(b)
		sb.WriteByte('\n')
	}
	transcriptPath := filepath.Join(home, "agent_session.jsonl")
	if err := os.WriteFile(transcriptPath, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}

	stdout, stderr, code := runCLI(t, home, "capture", "run", "--transcript", transcriptPath, "--scope", "project:test-proj", "--harness", "antigravity")
	if code != 0 {
		t.Fatalf("capture run exit code = %d, want 0, stderr: %s", code, stderr)
	}

	var summary capture.CaptureSummary
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("invalid summary JSON: %v, raw: %s", err, stdout)
	}

	if !summary.OK {
		t.Errorf("summary.OK = false, want true")
	}
	if summary.SessionID == "" {
		t.Errorf("summary.SessionID is empty")
	}
	if summary.Harness != "antigravity" {
		t.Errorf("summary.Harness = %q, want 'antigravity'", summary.Harness)
	}
	if summary.Captured == 0 {
		t.Errorf("summary.Captured = 0, want > 0")
	}
	if summary.TotalMessages != 4 {
		t.Errorf("summary.TotalMessages = %d, want 4", summary.TotalMessages)
	}

	// Verify the captured memory is in the store and queryable
	recallOut, _, rCode := runCLI(t, home, "recall", "SQLite-vec", "--scope", "project:test-proj")
	if rCode != 0 {
		t.Fatalf("recall exit code = %d, want 0", rCode)
	}
	var recallRes map[string]any
	if err := json.Unmarshal([]byte(recallOut), &recallRes); err != nil {
		t.Fatalf("recall output invalid JSON: %v", err)
	}
	results, ok := recallRes["results"].([]any)
	if !ok || len(results) == 0 {
		t.Fatalf("expected recall results for captured memory, got %v", recallRes)
	}
}

func TestCLI_Capture_Run_Stdin(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	transcript := `{"role":"user","content":"What did we agree on?"}
{"role":"assistant","content":"We agreed on using JSON output for all CLI commands."}
`

	stdout, stderr, code := runCLIWithStdin(t, home, transcript, "capture", "run", "--scope", "project:test-stdin")
	if code != 0 {
		t.Fatalf("capture run via stdin exit code = %d, want 0, stderr: %s", code, stderr)
	}

	var summary capture.CaptureSummary
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("invalid summary JSON: %v, raw: %s", err, stdout)
	}
	if summary.Captured == 0 {
		t.Errorf("summary.Captured = 0, want > 0")
	}
}

func TestCLI_Capture_Run_Deduplication(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	transcript := `{"role":"user","content":"Architecture plan"}
{"role":"assistant","content":"We chose Go for high concurrency and single-binary deployment."}
`
	transcriptPath := filepath.Join(home, "dedup_session.jsonl")
	if err := os.WriteFile(transcriptPath, []byte(transcript), 0644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}

	// Run pass 1
	stdout1, _, code1 := runCLI(t, home, "capture", "run", "--transcript", transcriptPath, "--scope", "project:dedup")
	if code1 != 0 {
		t.Fatalf("pass 1 exit code = %d, want 0", code1)
	}
	var sum1 capture.CaptureSummary
	_ = json.Unmarshal([]byte(stdout1), &sum1)
	if sum1.Captured == 0 {
		t.Fatalf("pass 1 expected captured > 0, got 0")
	}

	// Run pass 2 on same transcript
	stdout2, _, code2 := runCLI(t, home, "capture", "run", "--transcript", transcriptPath, "--scope", "project:dedup")
	if code2 != 0 {
		t.Fatalf("pass 2 exit code = %d, want 0", code2)
	}
	var sum2 capture.CaptureSummary
	_ = json.Unmarshal([]byte(stdout2), &sum2)
	if sum2.Captured != 0 {
		t.Errorf("pass 2 captured = %d, want 0 (duplicate)", sum2.Captured)
	}
	if sum2.SkippedDuplicate == 0 {
		t.Errorf("pass 2 skipped_duplicate = 0, want > 0")
	}
}

func TestCLI_Capture_Run_MissingTranscript(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	_, stderr, code := runCLI(t, home, "capture", "run", "--scope", "project:test")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "missing transcript") {
		t.Errorf("stderr %q should mention missing transcript", stderr)
	}
}

func TestCLI_Capture_Summary_LatestAndSession(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	transcript := `{"role":"user","content":"Decision check"}
{"role":"assistant","content":"We decided to store summaries on disk as JSON files."}
`
	transcriptPath := filepath.Join(home, "summary_session.jsonl")
	_ = os.WriteFile(transcriptPath, []byte(transcript), 0644)

	stdoutRun, _, codeRun := runCLI(t, home, "capture", "run", "--transcript", transcriptPath, "--scope", "project:sum")
	if codeRun != 0 {
		t.Fatalf("capture run failed: %d", codeRun)
	}
	var runSummary capture.CaptureSummary
	_ = json.Unmarshal([]byte(stdoutRun), &runSummary)

	// 1. Fetch latest summary without --session
	stdoutLatest, _, codeLatest := runCLI(t, home, "capture", "summary")
	if codeLatest != 0 {
		t.Fatalf("summary latest exit code = %d, want 0", codeLatest)
	}
	var latestSummary capture.CaptureSummary
	if err := json.Unmarshal([]byte(stdoutLatest), &latestSummary); err != nil {
		t.Fatalf("invalid latest summary JSON: %v", err)
	}
	if latestSummary.SessionID != runSummary.SessionID {
		t.Errorf("latest session ID = %q, want %q", latestSummary.SessionID, runSummary.SessionID)
	}

	// 2. Fetch specific session summary by --session <id>
	stdoutSpecific, _, codeSpecific := runCLI(t, home, "capture", "summary", "--session", runSummary.SessionID)
	if codeSpecific != 0 {
		t.Fatalf("summary specific exit code = %d, want 0", codeSpecific)
	}
	var specificSummary capture.CaptureSummary
	_ = json.Unmarshal([]byte(stdoutSpecific), &specificSummary)
	if specificSummary.SessionID != runSummary.SessionID {
		t.Errorf("specific session ID = %q, want %q", specificSummary.SessionID, runSummary.SessionID)
	}
}

func TestCLI_Capture_Summary_NotFound(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	// 1. No summaries exist
	_, stderr1, code1 := runCLI(t, home, "capture", "summary")
	if code1 != 2 {
		t.Fatalf("exit code = %d, want 2 (not found), stderr: %s", code1, stderr1)
	}

	// 2. Nonexistent session ID
	_, stderr2, code2 := runCLI(t, home, "capture", "summary", "--session", "nonexistent-sess-999")
	if code2 != 2 {
		t.Fatalf("exit code = %d, want 2 (not found), stderr: %s", code2, stderr2)
	}
}

func TestCLI_Capture_Categories_List_Add_Remove(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	// 1. List default categories
	stdoutList, _, codeList := runCLI(t, home, "capture", "categories", "--list")
	if codeList != 0 {
		t.Fatalf("categories list exit code = %d, want 0", codeList)
	}
	var listRes struct {
		OK         bool     `json:"ok"`
		Categories []string `json:"categories"`
	}
	if err := json.Unmarshal([]byte(stdoutList), &listRes); err != nil {
		t.Fatalf("invalid categories JSON: %v", err)
	}
	if len(listRes.Categories) != 7 {
		t.Errorf("expected 7 default categories, got %d (%v)", len(listRes.Categories), listRes.Categories)
	}

	// 2. Add custom categories
	stdoutAdd, _, codeAdd := runCLI(t, home, "capture", "categories", "--add", "architecture,pattern")
	if codeAdd != 0 {
		t.Fatalf("categories add exit code = %d, want 0", codeAdd)
	}
	var addRes struct {
		OK         bool     `json:"ok"`
		Categories []string `json:"categories"`
	}
	_ = json.Unmarshal([]byte(stdoutAdd), &addRes)
	if !containsStr(addRes.Categories, "architecture") || !containsStr(addRes.Categories, "pattern") {
		t.Errorf("added categories missing: %v", addRes.Categories)
	}

	// 3. Remove a category
	stdoutRem, _, codeRem := runCLI(t, home, "capture", "categories", "--remove", "pattern")
	if codeRem != 0 {
		t.Fatalf("categories remove exit code = %d, want 0", codeRem)
	}
	var remRes struct {
		OK         bool     `json:"ok"`
		Categories []string `json:"categories"`
	}
	_ = json.Unmarshal([]byte(stdoutRem), &remRes)
	if containsStr(remRes.Categories, "pattern") {
		t.Errorf("expected 'pattern' removed, but still present in %v", remRes.Categories)
	}
	if !containsStr(remRes.Categories, "architecture") {
		t.Errorf("expected 'architecture' preserved, but missing in %v", remRes.Categories)
	}
}

func TestCLI_Capture_Categories_InvalidName(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	_, stderr, code := runCLI(t, home, "capture", "categories", "--add", "invalid cat name with spaces!")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "invalid category name") {
		t.Errorf("stderr %q should mention invalid category name", stderr)
	}
}

func TestCLI_Capture_Convert(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	rawTranscript := `User: Hello, how do we deploy?
Assistant: We deploy via GitHub Actions.
`
	inputPath := filepath.Join(home, "raw_transcript.txt")
	if err := os.WriteFile(inputPath, []byte(rawTranscript), 0644); err != nil {
		t.Fatalf("write raw transcript: %v", err)
	}

	// 1. Convert with default output path
	stdoutDef, stderrDef, codeDef := runCLI(t, home, "capture", "convert", "--harness", "cursor", "--input", inputPath)
	if codeDef != 0 {
		t.Fatalf("convert exit code = %d, want 0, stderr: %s", codeDef, stderrDef)
	}
	var convRes struct {
		OK       bool   `json:"ok"`
		Output   string `json:"output"`
		Messages int    `json:"messages"`
		Harness  string `json:"harness"`
	}
	if err := json.Unmarshal([]byte(stdoutDef), &convRes); err != nil {
		t.Fatalf("invalid convert JSON: %v", err)
	}
	if !convRes.OK || convRes.Messages != 2 || convRes.Harness != "cursor" {
		t.Errorf("convert result unexpected: %+v", convRes)
	}
	if convRes.Output != inputPath+".centmem.jsonl" {
		t.Errorf("output path = %q, want %q", convRes.Output, inputPath+".centmem.jsonl")
	}

	// Verify file was written with valid JSONL lines
	f, err := os.Open(convRes.Output)
	if err != nil {
		t.Fatalf("open converted file: %v", err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	lines := 0
	for scanner.Scan() {
		lines++
		var msg capture.TranscriptMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			t.Fatalf("converted line is not valid JSON TranscriptMessage: %v", err)
		}
		if msg.Role == "" || msg.Content == "" {
			t.Fatalf("unexpected empty role or content: %+v", msg)
		}
	}
	if lines != 2 {
		t.Errorf("converted lines count = %d, want 2", lines)
	}

	// 2. Convert with explicit output path
	explicitOut := filepath.Join(home, "custom_converted.jsonl")
	_, _, codeExp := runCLI(t, home, "capture", "convert", "--harness", "antigravity", "--input", inputPath, "--output", explicitOut)
	if codeExp != 0 {
		t.Fatalf("convert explicit exit code = %d, want 0", codeExp)
	}
	if _, err := os.Stat(explicitOut); err != nil {
		t.Errorf("expected explicit output file %s to exist: %v", explicitOut, err)
	}

	// 3. Convert with missing --input
	_, _, codeMissing := runCLI(t, home, "capture", "convert")
	if codeMissing != 1 {
		t.Fatalf("convert missing input exit code = %d, want 1", codeMissing)
	}
}

func containsStr(slice []string, val string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, val) {
			return true
		}
	}
	return false
}

