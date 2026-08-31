package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// binaryPath is the path to a once-built centmem binary used by the e2e tests
// that shell out to the real binary (validating distribution behavior).
var (
	binOnce sync.Once
	binPath string
	binErr  error
)

// builtBinary returns the path to a compiled centmem binary, building it once.
func builtBinary(t *testing.T) string {
	t.Helper()
	binOnce.Do(func() {
		dir := os.TempDir()
		binPath = filepath.Join(dir, "centmem-test-bin")
		cmd := exec.Command("go", "build", "-tags", "fts5", "-o", binPath, "./cmd/centmem")
		cmd.Dir = repoRoot()
		out, err := cmd.CombinedOutput()
		if err != nil {
			binErr = err
			t.Fatalf("go build binary: %v\n%s", err, out)
		}
	})
	if binErr != nil {
		t.Fatalf("binary build failed: %v", binErr)
	}
	return binPath
}

// runBinary runs the built centmem binary with a temp home, capturing stdout.
func runBinary(t *testing.T, home string, args ...string) (string, int) {
	t.Helper()
	bin := builtBinary(t)
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(),
		"CENTMEM_HOME="+home,
		"CENTMEM_DB="+filepath.Join(home, "centmem.db"),
	)
	out, err := cmd.Output()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("run binary: %v", err)
		}
	}
	return string(out), code
}

// TestE2E_TwoAgentsShareMemory: Agent A (claude) writes a decision; Agent B
// (codex) recalls it across agents via the shared store (inherit default).
func TestE2E_TwoAgentsShareMemory(t *testing.T) {
	bin := builtBinary(t)
	home := t.TempDir()

	// Agent A writes.
	_, code := runBinary(t, home, "put", "--scope", "project:cent-mem", "--type", "note",
		"--content", "Deploy to Fly.io via GitHub Actions on merge to main.",
		"--tags", "deploy", "--source-agent", "claude", "--source-session", "sessA")
	if code != 0 {
		t.Fatalf("Agent A put exit code = %d, want 0", code)
	}

	// Agent B recalls (no --agent filter => sees all agents).
	out, code := runBinary(t, home, "recall", "how do we deploy?", "--scope", "project:cent-mem", "--top", "5")
	if code != 0 {
		t.Fatalf("Agent B recall exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "Fly.io") {
		t.Fatalf("Agent B did not retrieve Agent A's memory.\nrecall: %s", out)
	}
	_ = bin
}

// TestE2E_SessionIsolation: a session-scoped write is not visible at the project
// scope unless --children is passed.
func TestE2E_SessionIsolation(t *testing.T) {
	builtBinary(t)
	home := t.TempDir()

	// Write a session-scoped log.
	_, code := runBinary(t, home, "put", "--scope", "project:cent-mem/agent:codex/session:sessB",
		"--type", "log", "--content", "private session checkpoint", "--source-agent", "codex", "--source-session", "sessB")
	if code != 0 {
		t.Fatalf("session put exit code = %d, want 0", code)
	}

	// Without --children, a project recall must NOT see the session memory.
	out, code := runBinary(t, home, "recall", "session checkpoint", "--scope", "project:cent-mem", "--top", "5")
	if code != 0 {
		t.Fatalf("recall (no children) exit code = %d, want 0", code)
	}
	if strings.Contains(out, "private session checkpoint") {
		t.Errorf("session memory leaked to project recall without --children:\n%s", out)
	}

	// With --children, the session memory IS visible.
	out, code = runBinary(t, home, "recall", "session checkpoint", "--scope", "project:cent-mem", "--top", "5", "--children")
	if code != 0 {
		t.Fatalf("recall (--children) exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "private session checkpoint") {
		t.Errorf("session memory not visible with --children:\n%s", out)
	}
}

// TestE2E_InstallThenUse: installing the skill (via install.sh) into a temp
// HOME lands SKILL.md, and the built binary supports the full put/set/recall/get
// flow.
func TestE2E_InstallThenUse(t *testing.T) {
	builtBinary(t)
	installer := filepath.Join(repoRoot(), "skill", "install.sh")
	home := t.TempDir()

	// Install into the temp HOME (also verifies the installer runs cleanly).
	cmd := exec.Command("bash", installer)
	cmd.Env = append(os.Environ(), "HOME="+home)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, out)
	}
	skillPath := filepath.Join(home, ".claude", "skills", "centmem", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("SKILL.md not installed at %s: %v", skillPath, err)
	}

	// Full recall/set/put/get flow against the binary.
	_, code := runBinary(t, home, "set", "--scope", "project:demo", "--key", "user.timezone", "--value", `"Asia/Jakarta"`)
	if code != 0 {
		t.Fatalf("set exit code = %d, want 0", code)
	}
	_, code = runBinary(t, home, "put", "--scope", "project:demo", "--type", "note", "--content", "test note for install flow")
	if code != 0 {
		t.Fatalf("put exit code = %d, want 0", code)
	}
	out, code := runBinary(t, home, "get", "--scope", "project:demo", "--key", "user.timezone")
	if code != 0 {
		t.Fatalf("get exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "Asia/Jakarta") {
		t.Errorf("get did not return stored fact: %s", out)
	}
	out, code = runBinary(t, home, "recall", "test note", "--scope", "project:demo", "--top", "5")
	if code != 0 {
		t.Fatalf("recall exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "test note for install flow") {
		t.Errorf("recall did not return stored note: %s", out)
	}
}
