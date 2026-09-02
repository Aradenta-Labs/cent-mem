package capture_test

import (
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/capture"
)

func TestSummaryTracker_TrackingAndSerialization(t *testing.T) {
	tempHome := t.TempDir()
	sessionID := "session-12345"

	tracker := capture.NewSummaryTracker(sessionID, "antigravity", "heuristic")
	tracker.SetTotalMessages(10)

	item1 := capture.CaptureItem{Category: "decision", Content: "Use WAL mode", Confidence: 0.85}
	item2 := capture.CaptureItem{Category: "fact", Content: "v1.2.0", Key: "version", Confidence: 0.90}
	dupItem := capture.CaptureItem{Category: "decision", Content: "Use WAL mode", Confidence: 0.85}
	lowConfItem := capture.CaptureItem{Category: "note", Content: "Maybe do this", Confidence: 0.3}

	tracker.RecordCaptured(item1)
	tracker.RecordCaptured(item2)
	tracker.RecordSkippedDuplicate(dupItem)
	tracker.RecordSkippedLowConfidence(lowConfItem)

	summary := tracker.Finalize()

	if !summary.OK {
		t.Errorf("expected OK to be true")
	}
	if summary.SessionID != sessionID {
		t.Errorf("expected session ID %q, got %q", sessionID, summary.SessionID)
	}
	if summary.Harness != "antigravity" {
		t.Errorf("expected harness 'antigravity', got %q", summary.Harness)
	}
	if summary.Backend != "heuristic" {
		t.Errorf("expected backend 'heuristic', got %q", summary.Backend)
	}
	if summary.TotalMessages != 10 {
		t.Errorf("expected 10 total messages, got %d", summary.TotalMessages)
	}
	if summary.Captured != 2 {
		t.Errorf("expected 2 captured, got %d", summary.Captured)
	}
	if summary.SkippedDuplicate != 1 {
		t.Errorf("expected 1 duplicate skipped, got %d", summary.SkippedDuplicate)
	}
	if summary.SkippedLowConfidence != 1 {
		t.Errorf("expected 1 low confidence skipped, got %d", summary.SkippedLowConfidence)
	}
	if len(summary.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(summary.Items))
	}

	// Test Save and Load
	if err := tracker.SaveSummary(tempHome); err != nil {
		t.Fatalf("SaveSummary failed: %v", err)
	}

	loaded, err := capture.LoadSummary(tempHome, sessionID)
	if err != nil {
		t.Fatalf("LoadSummary failed: %v", err)
	}
	if loaded.SessionID != sessionID || loaded.Captured != 2 {
		t.Errorf("loaded summary mismatch: %+v", loaded)
	}

	// Test FindLatestSummary
	latest, err := capture.FindLatestSummary(tempHome)
	if err != nil {
		t.Fatalf("FindLatestSummary failed: %v", err)
	}
	if latest.SessionID != sessionID {
		t.Errorf("expected latest session ID %q, got %q", sessionID, latest.SessionID)
	}
}

func TestSummary_FindLatestEmpty(t *testing.T) {
	tempHome := t.TempDir()
	_, err := capture.FindLatestSummary(tempHome)
	if err == nil {
		t.Errorf("expected error when no summaries exist, got nil")
	}
}
