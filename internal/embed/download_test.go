package embed_test

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/farras/cent-mem/internal/embed"
)

// serveModel spins up a local HTTP server that serves content and returns its
// hex sha256. Used to exercise Download without the network.
func serveModel(t *testing.T, content []byte) (url, sum string) {
	t.Helper()
	h := sha256.Sum256(content)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(content)
	}))
	t.Cleanup(srv.Close)
	return srv.URL, hex.EncodeToString(h[:])
}

// TestDownload_Success verifies Download fetches, verifies, and renames the file.
func TestDownload_Success(t *testing.T) {
	content := []byte("model-bytes")
	url, sum := serveModel(t, content)
	embed.ModelCatalog["test-download"] = embed.ModelInfo{
		Name: "test-download", URL: url, SHA256: sum, Dims: 8, File: "test.onnx",
	}
	defer delete(embed.ModelCatalog, "test-download")

	dir := t.TempDir()
	path := filepath.Join(dir, "test.onnx")
	got, err := embed.Download("test-download", path)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if got != path {
		t.Errorf("returned path = %q, want %q", got, path)
	}
	b, err := os.ReadFile(path)
	if err != nil || string(b) != string(content) {
		t.Errorf("file content = %q, err=%v; want %q", b, err, content)
	}
}

// TestDownload_SkipsWhenPresent verifies Download short-circuits when the file
// already exists with the matching checksum.
func TestDownload_SkipsWhenPresent(t *testing.T) {
	content := []byte("model-bytes")
	url, sum := serveModel(t, content)
	embed.ModelCatalog["test-skip"] = embed.ModelInfo{
		Name: "test-skip", URL: url, SHA256: sum, Dims: 8, File: "test.onnx",
	}
	defer delete(embed.ModelCatalog, "test-skip")

	dir := t.TempDir()
	path := filepath.Join(dir, "test.onnx")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	if _, err := embed.Download("test-skip", path); err != nil {
		t.Fatalf("Download with existing valid file: %v", err)
	}
}

// TestDownload_UnknownModel verifies an unknown model name errors.
func TestDownload_UnknownModel(t *testing.T) {
	if _, err := embed.Download("nope-does-not-exist", "/tmp/x.onnx"); err == nil {
		t.Fatal("expected error for unknown model")
	}
}

// TestDownload_BadChecksum verifies a mismatched checksum is rejected.
func TestDownload_BadChecksum(t *testing.T) {
	content := []byte("model-bytes")
	url, _ := serveModel(t, content) // note: discarded correct sum
	embed.ModelCatalog["test-badsum"] = embed.ModelInfo{
		Name: "test-badsum", URL: url, SHA256: strings.Repeat("0", 64), Dims: 8, File: "test.onnx",
	}
	defer delete(embed.ModelCatalog, "test-badsum")

	dir := t.TempDir()
	if _, err := embed.Download("test-badsum", filepath.Join(dir, "test.onnx")); err == nil {
		t.Fatal("expected sha256 mismatch error")
	}
}

// TestIsModelDownloaded verifies detection of an existing, valid model file.
func TestIsModelDownloaded(t *testing.T) {
	content := []byte("model-bytes")
	h := sha256.Sum256(content)
	sum := hex.EncodeToString(h[:])
	embed.ModelCatalog["test-dl-check"] = embed.ModelInfo{
		Name: "test-dl-check", URL: "", SHA256: sum, Dims: 8, File: "test.onnx",
	}
	defer delete(embed.ModelCatalog, "test-dl-check")

	dir := t.TempDir()
	path := filepath.Join(dir, "test.onnx")
	if embed.IsModelDownloaded("test-dl-check", path) {
		t.Fatal("expected false before file exists")
	}
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	if !embed.IsModelDownloaded("test-dl-check", path) {
		t.Fatal("expected true after valid file written")
	}
}

// TestStubDims verifies NewStub reports the configured dimension.
func TestStubDims(t *testing.T) {
	emb := embed.NewStub(512)
	if emb.Dims() != 512 {
		t.Errorf("Dims() = %d, want 512", emb.Dims())
	}
}

// TestCacheDimsClose verifies the caching wrapper's passthrough methods.
func TestCacheDimsClose(t *testing.T) {
	c := embed.NewCachingEmbedder(embed.NewStub(384), 64)
	if c.Dims() != 384 {
		t.Errorf("CachingEmbedder.Dims() = %d, want 384", c.Dims())
	}
	if err := c.Close(); err != nil {
		t.Errorf("CachingEmbedder.Close() = %v, want nil", err)
	}
}
