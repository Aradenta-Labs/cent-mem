package embed_test

import (
	"context"
	"math"
	"reflect"
	"testing"

	"github.com/farras/cent-mem/internal/embed"
)

func TestEmbedShape(t *testing.T) {
	ctx := context.Background()
	dims := 384
	emb := embed.NewStub(dims)

	vec, err := emb.Embed(ctx, "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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

	vec1, err := emb.Embed(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error on first embed: %v", err)
	}

	vec2, err := emb.Embed(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error on second embed: %v", err)
	}

	if !reflect.DeepEqual(vec1, vec2) {
		t.Errorf("expected deterministic embedding vectors to match identically")
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
