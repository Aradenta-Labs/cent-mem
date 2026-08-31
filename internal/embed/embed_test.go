package embed_test

import (
	"context"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/farras/cent-mem/internal/embed"
)

func TestEmbedShape(t *testing.T) {
	ctx := context.Background()
	dims := 384
	emb := embed.NewStub(dims)

	vecs, err := emb.Embed(ctx, []string{"hello world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vecs) != 1 {
		t.Fatalf("expected 1 vector, got %d", len(vecs))
	}
	vec := vecs[0]
	if len(vec) != dims {
		t.Fatalf("expected vector dimensions %d, got %d", dims, len(vec))
	}

	for i, val := range vec {
		if math.IsNaN(float64(val)) || math.IsInf(float64(val), 0) {
			t.Errorf("vector element %d is non-finite: %f", i, val)
		}
	}
}

func TestEmbedDeterministic(t *testing.T) {
	ctx := context.Background()
	dims := 384
	emb := embed.NewStub(dims)

	input := "The quick brown fox jumps over the lazy dog"

	vecs1, err := emb.Embed(ctx, []string{input})
	if err != nil {
		t.Fatalf("unexpected error on first embed: %v", err)
	}
	vecs2, err := emb.Embed(ctx, []string{input})
	if err != nil {
		t.Fatalf("unexpected error on second embed: %v", err)
	}

	if !reflect.DeepEqual(vecs1[0], vecs2[0]) {
		t.Errorf("expected deterministic embedding vectors to match identically")
	}
}

// I.4 — empty text handled (returns a zero vector, not an error).
func TestEmbedder_Empty(t *testing.T) {
	ctx := context.Background()
	emb := embed.NewStub(8)
	vecs, err := emb.Embed(ctx, []string{"", ""})
	if err != nil {
		t.Fatalf("empty input must not error: %v", err)
	}
	for _, v := range vecs {
		if len(v) != 8 {
			t.Fatalf("expected dims 8, got %d", len(v))
		}
		// Zero vector.
		for _, val := range v {
			if val != 0 {
				t.Fatalf("expected zero vector for empty text, got %v", v)
			}
		}
	}
}

// I.5 — overlong text is truncated, not crashed.
func TestEmbedder_Truncate(t *testing.T) {
	ctx := context.Background()
	emb := embed.NewStub(16)
	long := strings.Repeat("a", 100000)
	vecs, err := emb.Embed(ctx, []string{long})
	if err != nil {
		t.Fatalf("overlong text must not crash: %v", err)
	}
	if len(vecs) != 1 || len(vecs[0]) != 16 {
		t.Fatalf("unexpected result shape")
	}
}

// I.3 — no NaN/Inf across a batch.
func TestEmbedder_Finite(t *testing.T) {
	ctx := context.Background()
	emb := embed.NewStub(32)
	vecs, err := emb.Embed(ctx, []string{"alpha beta gamma", "delta", "epsilon zeta eta theta"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, v := range vecs {
		for i, val := range v {
			if math.IsNaN(float64(val)) || math.IsInf(float64(val), 0) {
				t.Errorf("non-finite element %d in %v", i, v)
			}
		}
	}
}

// Batch shape: len(output) == len(input).
func TestEmbedder_BatchShape(t *testing.T) {
	ctx := context.Background()
	emb := embed.NewStub(8)
	in := []string{"one", "two", "three"}
	vecs, err := emb.Embed(ctx, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vecs) != len(in) {
		t.Fatalf("expected %d vectors, got %d", len(in), len(vecs))
	}
}

func TestModelCatalog(t *testing.T) {
	model, exists := embed.ModelCatalog["bge-small-en-v1.5"]
	if !exists {
		t.Fatalf("default model 'bge-small-en-v1.5' not found in ModelCatalog")
	}

	if model.Dims != 384 {
		t.Errorf("expected 384 dimensions, got %d", model.Dims)
	}

	if model.SHA256 == "" {
		t.Errorf("expected non-empty SHA256 checksum in catalog")
	}
}

// New() falls back to the offline stub embedder when no model file is present.
func TestNew_FallsBackToStub(t *testing.T) {
	emb, err := embed.New("/nonexistent/model.onnx", 384, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer emb.Close()
	vecs, err := emb.Embed(context.Background(), []string{"hello"})
	if err != nil {
		t.Fatalf("fallback embedder must work offline: %v", err)
	}
	if len(vecs) != 1 || len(vecs[0]) != 384 {
		t.Fatalf("unexpected fallback result shape")
	}
}
