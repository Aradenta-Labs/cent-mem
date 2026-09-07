package capture_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/capture"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

func TestCaptureComments_BasicAndKeywords(t *testing.T) {
	dir := t.TempDir()
	st := setupTestStore(t)
	ctx := context.Background()

	codeFile := filepath.Join(dir, "main.go")
	content := `package main

// TODO: implement graceful shutdown
func main() {
	/*
	 * SECURITY: always validate user input
	 */
	// NOTE: this is important context
	println("hello")
}
`
	if err := os.WriteFile(codeFile, []byte(content), 0644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	res, err := capture.CaptureComments(ctx, st, capture.CommentsCaptureConfig{
		Dir:    dir,
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("capture comments: %v", err)
	}
	if res.Scanned != 1 {
		t.Fatalf("expected 1 file scanned, got %d", res.Scanned)
	}
	if len(res.Items) != 3 {
		t.Fatalf("expected 3 comment items, got %d", len(res.Items))
	}
	if res.MemoriesCreated != 3 {
		t.Fatalf("expected 3 memories created, got %d", res.MemoriesCreated)
	}

	// Verify custom keywords flag
	customRes, err := capture.CaptureComments(ctx, st, capture.CommentsCaptureConfig{
		Dir:      dir,
		Keywords: []string{"SECURITY"},
		DryRun:   true,
	})
	if err != nil {
		t.Fatalf("custom keywords capture: %v", err)
	}
	if len(customRes.Items) != 1 {
		t.Fatalf("expected 1 item with custom keyword SECURITY, got %d", len(customRes.Items))
	}
	if !strings.Contains(customRes.Items[0].Content, "[SECURITY]") {
		t.Fatalf("unexpected content: %s", customRes.Items[0].Content)
	}
}

func TestCaptureComments_LineShift(t *testing.T) {
	dir := t.TempDir()
	st := setupTestStore(t)
	ctx := context.Background()

	codeFile := filepath.Join(dir, "app.py")
	contentV1 := `# TODO: add telemetry metrics
def run():
    pass
`
	if err := os.WriteFile(codeFile, []byte(contentV1), 0644); err != nil {
		t.Fatalf("write app.py: %v", err)
	}

	res1, err := capture.CaptureComments(ctx, st, capture.CommentsCaptureConfig{
		Dir:    dir,
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("capture v1: %v", err)
	}
	if res1.MemoriesCreated != 1 {
		t.Fatalf("expected 1 memory created, got %d", res1.MemoriesCreated)
	}

	// Re-run unchanged: duplicate no-op
	resDup, err := capture.CaptureComments(ctx, st, capture.CommentsCaptureConfig{
		Dir:    dir,
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("capture dup: %v", err)
	}
	if resDup.MemoriesCreated != 0 || resDup.MemoriesUpdated != 0 {
		t.Fatalf("expected 0 created/updated for unchanged file, got created=%d updated=%d",
			resDup.MemoriesCreated, resDup.MemoriesUpdated)
	}

	// Shift lines by inserting empty lines at top
	contentV2 := `import sys
import os

# TODO: add telemetry metrics
def run():
    pass
`
	if err := os.WriteFile(codeFile, []byte(contentV2), 0644); err != nil {
		t.Fatalf("write app.py v2: %v", err)
	}

	res2, err := capture.CaptureComments(ctx, st, capture.CommentsCaptureConfig{
		Dir:    dir,
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("capture v2: %v", err)
	}
	if res2.MemoriesUpdated != 1 {
		t.Fatalf("expected 1 memory updated on line shift, got %d", res2.MemoriesUpdated)
	}

	// Verify updated line number in store
	mems, err := st.List(ctx, store.ListQuery{
		ScopePath: capture.ResolveDefaultScope(dir),
		Status:    "active",
	})
	if err != nil {
		t.Fatalf("list memories: %v", err)
	}
	if len(mems) != 1 {
		t.Fatalf("expected 1 active memory in store, got %d", len(mems))
	}
	if !strings.Contains(mems[0].Content, "[file: app.py:4]") {
		t.Fatalf("expected line 4 in updated memory content, got %s", mems[0].Content)
	}
}

func TestCaptureComments_PythonDocstringAndCode(t *testing.T) {
	dir := t.TempDir()
	st := setupTestStore(t)
	ctx := context.Background()

	codeFile := filepath.Join(dir, "worker.py")
	content := `"""Single line docstring."""
config_val = "TODO: this is a string in code, not a comment"
# TODO: real comment in python
`
	if err := os.WriteFile(codeFile, []byte(content), 0644); err != nil {
		t.Fatalf("write worker.py: %v", err)
	}

	res, err := capture.CaptureComments(ctx, st, capture.CommentsCaptureConfig{
		Dir:    dir,
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("capture comments: %v", err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("expected exactly 1 comment captured, got %d: %+v", len(res.Items), res.Items)
	}
	if !strings.Contains(res.Items[0].Content, "real comment in python") {
		t.Errorf("unexpected captured item: %s", res.Items[0].Content)
	}
}

func TestCaptureComments_LanguageAwarenessAndURLSafety(t *testing.T) {
	dir := t.TempDir()
	st := setupTestStore(t)
	ctx := context.Background()

	// Go file with URL and comment
	goFile := filepath.Join(dir, "api.go")
	goContent := `package main
const authEndpoint = "https://TODO:secret@example.com/api"
// NOTE: valid go comment
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("write api.go: %v", err)
	}

	// Python file with integer division
	pyFile := filepath.Join(dir, "math_utils.py")
	pyContent := `val = total // TODO / 2
# HACK: real hack in python
`
	if err := os.WriteFile(pyFile, []byte(pyContent), 0644); err != nil {
		t.Fatalf("write math_utils.py: %v", err)
	}

	res, err := capture.CaptureComments(ctx, st, capture.CommentsCaptureConfig{
		Dir:    dir,
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("capture comments: %v", err)
	}
	if len(res.Items) != 2 {
		t.Fatalf("expected exactly 2 items captured, got %d: %+v", len(res.Items), res.Items)
	}

	foundGoNote := false
	foundPyHack := false
	for _, it := range res.Items {
		if strings.Contains(it.Content, "[NOTE] valid go comment") {
			foundGoNote = true
		}
		if strings.Contains(it.Content, "[HACK] real hack in python") {
			foundPyHack = true
		}
		if strings.Contains(it.Content, "secret@example.com") {
			t.Errorf("URL false positive captured: %s", it.Content)
		}
		if strings.Contains(it.Content, "/ 2") {
			t.Errorf("Python int division false positive captured: %s", it.Content)
		}
	}
	if !foundGoNote || !foundPyHack {
		t.Errorf("expected to find both valid comments, got %+v", res.Items)
	}
}
