package capture_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/capture"
)

func TestSession_LifecycleAndLocking(t *testing.T) {
	tempHome := t.TempDir()
	cfg := capture.DefaultCaptureConfig()

	// 1. Start Session
	sess, err := capture.StartSession(tempHome, "antigravity", cfg)
	if err != nil {
		t.Fatalf("StartSession failed: %v", err)
	}

	if sess.ID == "" {
		t.Errorf("expected non-empty session ID")
	}

	// Verify lock file exists
	lockPath := filepath.Join(tempHome, "capture-session.lock")
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Errorf("expected lock file %s to exist", lockPath)
	}

	// 2. Attempt starting another concurrent session (should fail)
	_, err = capture.StartSession(tempHome, "antigravity", cfg)
	if err == nil {
		t.Errorf("expected concurrent session start to fail with lock error")
	}

	// 3. Record something in tracker
	sess.Tracker.RecordCaptured(capture.CaptureItem{
		Category: "decision",
		Content:  "Use session coordinator",
	})

	// 4. End Session
	summary, err := capture.EndSession(sess)
	if err != nil {
		t.Fatalf("EndSession failed: %v", err)
	}

	if summary == nil || summary.Captured != 1 {
		t.Errorf("unexpected summary from EndSession: %+v", summary)
	}

	// Verify lock file was removed
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Errorf("expected lock file to be removed after EndSession")
	}

	// Verify summary file was saved
	summaryPath := filepath.Join(tempHome, "capture-summary-"+sess.ID+".json")
	if _, err := os.Stat(summaryPath); os.IsNotExist(err) {
		t.Errorf("expected summary file %s to exist", summaryPath)
	}

	// 5. Start a new session after end (should succeed now)
	sess2, err := capture.StartSession(tempHome, "antigravity", cfg)
	if err != nil {
		t.Fatalf("StartSession after EndSession failed: %v", err)
	}
	_, _ = capture.EndSession(sess2)
}
