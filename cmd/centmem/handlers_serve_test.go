package main

import (
	"strings"
	"testing"
)

func TestCmdServe_MCPHandshakeAndList(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`
	listReq := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`
	input := initReq + "\n" + listReq + "\n"

	stdout, stderr, code := runCLIWithStdin(t, home, input, "serve")
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d. Stderr: %s", code, stderr)
	}

	if !strings.Contains(stdout, `"protocolVersion":"2024-11-05"`) {
		t.Errorf("expected protocolVersion 2024-11-05 in stdout, got: %s", stdout)
	}
	if !strings.Contains(stdout, `"name":"centmem_recall"`) {
		t.Errorf("expected centmem_recall in stdout tools list, got: %s", stdout)
	}
	if !strings.Contains(stdout, `"name":"centmem_put"`) {
		t.Errorf("expected centmem_put in stdout tools list, got: %s", stdout)
	}
}

func TestCmdServe_ToolsCallWorkflow(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`
	putReq := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"centmem_put","arguments":{"content":"End-to-end CLI serve test passed","scope":"project:cent-mem","type":"note","tags":["test"]}}}`
	recallReq := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"centmem_recall","arguments":{"query":"CLI serve test","scope":"project:cent-mem"}}}`
	timelineReq := `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"centmem_timeline","arguments":{"scope":"project:cent-mem"}}}`
	input := initReq + "\n" + putReq + "\n" + recallReq + "\n" + timelineReq + "\n"

	stdout, stderr, code := runCLIWithStdin(t, home, input, "serve")
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d. Stderr: %s", code, stderr)
	}

	if !strings.Contains(stdout, `\"ok\":true`) {
		t.Errorf("expected ok:true in stdout, got: %s", stdout)
	}
	if !strings.Contains(stdout, "End-to-end CLI serve test passed") {
		t.Errorf("expected recalled memory content in stdout, got: %s", stdout)
	}
	if !strings.Contains(stdout, `\"entries\":`) {
		t.Errorf("expected entries in timeline output, got: %s", stdout)
	}
}

func TestCmdServe_InvalidFlag(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	_, stderr, code := runCLI(t, home, "serve", "--unknown-flag")
	if code == 0 {
		t.Fatalf("expected error exit code for unknown flag, got 0")
	}
	if !strings.Contains(stderr, "flag parse") && !strings.Contains(stderr, "ERR_INVALID_FLAG") {
		t.Errorf("expected flag parse error in stderr, got: %s", stderr)
	}
}

