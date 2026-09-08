package main

import (
	"syscall"
	"testing"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/cli"
)

func TestCmdUI_Registered(t *testing.T) {
	names := Names()
	found := false
	for _, n := range names {
		if n == "ui" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("command 'ui' not found in Names(): %v", names)
	}
}

func TestCmdUI_InvalidFlag(t *testing.T) {
	code := cmdUI([]string{"--invalid-flag-xyz"})
	if code != cli.ExitError {
		t.Errorf("cmdUI with invalid flag exited %d, want ExitError (%d)", code, cli.ExitError)
	}
}

func TestCmdUI_StartAndSignal(t *testing.T) {
	done := make(chan int, 1)

	go func() {
		// Use ephemeral port 0, bind localhost, no browser open
		code := cmdUI([]string{"--port", "0", "--no-open"})
		done <- code
	}()

	// Allow server to spin up and bind
	time.Sleep(150 * time.Millisecond)

	// Send SIGINT to own process to trigger graceful shutdown of cmdUI
	_ = syscall.Kill(syscall.Getpid(), syscall.SIGINT)

	select {
	case code := <-done:
		if code != cli.ExitOK {
			t.Errorf("cmdUI exited with code %d, want %d", code, cli.ExitOK)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cmdUI did not shut down within timeout")
	}
}

func TestCmdUI_NonLoopbackWithoutToken(t *testing.T) {
	code := cmdUI([]string{"--host", "0.0.0.0", "--port", "0", "--no-open"})
	if code != cli.ExitError {
		t.Errorf("cmdUI with 0.0.0.0 without token exited %d, want ExitError (%d)", code, cli.ExitError)
	}
}

func TestCmdUI_ShortToken(t *testing.T) {
	code := cmdUI([]string{"--token", "too-short", "--port", "0", "--no-open"})
	if code != cli.ExitError {
		t.Errorf("cmdUI with short token exited %d, want ExitError (%d)", code, cli.ExitError)
	}
}

