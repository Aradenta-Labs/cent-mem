package taxonomy

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/embed"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

type mockEmbedder struct {
	vectors map[string][]float32
	err     error
}

func (m *mockEmbedder) Dims() int { return 4 }
func (m *mockEmbedder) Close() error { return nil }
func (m *mockEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	if m.err != nil {
		return nil, m.err
	}
	out := make([][]float32, len(texts))
	for i, t := range texts {
		if v, ok := m.vectors[t]; ok {
			out[i] = v
		} else {
			out[i] = []float32{0, 0, 0, 0}
		}
	}
	return out, nil
}

func TestClusterTags_EmptyAndSingle(t *testing.T) {
	ctx := context.Background()
	c, err := ClusterTags(ctx, nil, Options{})
	if err != nil || len(c) != 0 {
		t.Fatalf("nil input: clusters=%v err=%v", c, err)
	}
	c, err = ClusterTags(ctx, []store.TagStat{{Tag: "database", Count: 5}}, Options{})
	if err != nil || len(c) != 0 {
		t.Fatalf("single input: clusters=%v err=%v", c, err)
	}
}

func TestClusterTags_LexicalPass(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	stats := []store.TagStat{
		{Tag: "decision", Count: 10, LastUsed: now},
		{Tag: "decisions", Count: 3, LastUsed: now},
		{Tag: "auth-flow", Count: 4, LastUsed: now},
		{Tag: "auth_flow", Count: 4, LastUsed: now},
		{Tag: "Auth_Flow", Count: 1, LastUsed: now},
		{Tag: "database", Count: 8, LastUsed: now},
		{Tag: "databse", Count: 2, LastUsed: now}, // Levenshtein 1, len >= 5
		{Tag: "memories", Count: 2, LastUsed: now},
		{Tag: "memory", Count: 6, LastUsed: now},
		// Short tags (< 5 chars) must NOT be merged by Levenshtein distance 1.
		{Tag: "api", Count: 5, LastUsed: now},
		{Tag: "app", Count: 5, LastUsed: now},
		// Words ending in ss/us/is must not be mangled into unrelated clusters.
		{Tag: "redis", Count: 3, LastUsed: now},
		{Tag: "status", Count: 3, LastUsed: now},
	}

	clusters, err := ClusterTags(ctx, stats, Options{})
	if err != nil {
		t.Fatalf("ClusterTags: %v", err)
	}

	byCanon := make(map[string]Cluster)
	for _, cl := range clusters {
		byCanon[cl.Canonical] = cl
	}

	if len(clusters) != 4 {
		t.Fatalf("expected 4 lexical clusters, got %d: %+v", len(clusters), clusters)
	}

	// decision / decisions
	if cl, ok := byCanon["decision"]; !ok || !slices.Equal(cl.Aliases, []string{"decisions"}) || cl.AffectedCount != 3 {
		t.Errorf("decision cluster = %+v", cl)
	}
	// auth-flow / auth_flow / Auth_Flow (tie between auth-flow and auth_flow -> lowercase kebab-case wins)
	if cl, ok := byCanon["auth-flow"]; !ok || !slices.Equal(cl.Aliases, []string{"Auth_Flow", "auth_flow"}) || cl.AffectedCount != 5 {
		t.Errorf("auth-flow cluster = %+v", cl)
	}
	// database / databse
	if cl, ok := byCanon["database"]; !ok || !slices.Equal(cl.Aliases, []string{"databse"}) || cl.AffectedCount != 2 {
		t.Errorf("database cluster = %+v", cl)
	}
	// memory / memories
	if cl, ok := byCanon["memory"]; !ok || !slices.Equal(cl.Aliases, []string{"memories"}) || cl.AffectedCount != 2 {
		t.Errorf("memory cluster = %+v", cl)
	}
}

func TestClusterTags_SemanticPassWithMockEmbedder(t *testing.T) {
	ctx := context.Background()
	emb := &mockEmbedder{
		vectors: map[string][]float32{
			"database": {0.98, 0.20, 0.0, 0.0},
			"db":       {0.97, 0.24, 0.0, 0.0}, // cosine ~0.999 with database
			"sqlite":   {0.94, 0.34, 0.0, 0.0}, // cosine ~0.989 with database
			"frontend": {0.0, 0.0, 1.0, 0.0},   // orthogonal
			"billing":  {0.0, 0.0, 0.0, 1.0},   // orthogonal
		},
	}

	stats := []store.TagStat{
		{Tag: "database", Count: 12},
		{Tag: "db", Count: 4},
		{Tag: "sqlite", Count: 3},
		{Tag: "frontend", Count: 7},
		{Tag: "billing", Count: 5},
	}

	clusters, err := ClusterTags(ctx, stats, Options{
		Embedder:            emb,
		SimilarityThreshold: 0.86,
	})
	if err != nil {
		t.Fatalf("ClusterTags semantic: %v", err)
	}
	if len(clusters) != 1 {
		t.Fatalf("expected 1 semantic cluster, got %d: %+v", len(clusters), clusters)
	}
	cl := clusters[0]
	if cl.Canonical != "database" {
		t.Errorf("canonical = %q, want database", cl.Canonical)
	}
	if !slices.Equal(cl.Aliases, []string{"db", "sqlite"}) {
		t.Errorf("aliases = %v, want [db sqlite]", cl.Aliases)
	}
	if cl.AffectedCount != 7 {
		t.Errorf("affected_count = %d, want 7", cl.AffectedCount)
	}
	if cl.MatchMethod != "semantic" {
		t.Errorf("match_method = %q, want semantic", cl.MatchMethod)
	}
	if cl.Similarity < 0.86 {
		t.Errorf("similarity = %v, want >= 0.86", cl.Similarity)
	}

	// Higher threshold filters out sqlite (cos(database, sqlite) ~ 0.989 -> test with 0.995).
	strict, err := ClusterTags(ctx, stats, Options{
		Embedder:            emb,
		SimilarityThreshold: 0.995,
	})
	if err != nil {
		t.Fatalf("ClusterTags strict: %v", err)
	}
	if len(strict) != 1 || !slices.Equal(strict[0].Aliases, []string{"db"}) {
		t.Errorf("strict clusters = %+v, want only [db] merged into database", strict)
	}
}

func TestClusterTags_CanonicalTieBreak(t *testing.T) {
	ctx := context.Background()
	emb := &mockEmbedder{
		vectors: map[string][]float32{
			"auth":           {1, 0, 0, 0},
			"authentication": {0.99, 0.14, 0, 0},
			"auth flow":      {0, 1, 0, 0},
		},
	}
	stats := []store.TagStat{
		{Tag: "authentication", Count: 5},
		{Tag: "auth", Count: 5},
		{Tag: "auth_flow", Count: 3},
		{Tag: "auth-flow", Count: 3},
	}

	clusters, err := ClusterTags(ctx, stats, Options{Embedder: emb})
	if err != nil {
		t.Fatalf("ClusterTags: %v", err)
	}
	byCanon := make(map[string]Cluster)
	for _, cl := range clusters {
		byCanon[cl.Canonical] = cl
	}
	// Equal counts: shorter lowercase kebab-case wins ("auth" beats "authentication").
	if _, ok := byCanon["auth"]; !ok {
		t.Errorf("expected 'auth' as canonical on tie, got %+v", clusters)
	}
	// Equal counts: lowercase kebab-case wins ("auth-flow" beats "auth_flow").
	if _, ok := byCanon["auth-flow"]; !ok {
		t.Errorf("expected 'auth-flow' as canonical on tie, got %+v", clusters)
	}
}

func TestClusterTags_IdempotentAfterMerge(t *testing.T) {
	ctx := context.Background()
	emb := &mockEmbedder{
		vectors: map[string][]float32{
			"database": {1, 0, 0, 0},
			"db":       {0.99, 0.1, 0, 0},
			"frontend": {0, 1, 0, 0},
		},
	}
	before := []store.TagStat{
		{Tag: "database", Count: 5},
		{Tag: "db", Count: 3},
		{Tag: "frontend", Count: 4},
	}
	clusters, err := ClusterTags(ctx, before, Options{Embedder: emb})
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster before merge, got %d", len(clusters))
	}

	// Simulate applying the merge: db folded into database.
	after := []store.TagStat{
		{Tag: "database", Count: 8},
		{Tag: "frontend", Count: 4},
	}
	clustersAfter, err := ClusterTags(ctx, after, Options{Embedder: emb})
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if len(clustersAfter) != 0 {
		t.Errorf("expected 0 clusters after merge (idempotent), got %+v", clustersAfter)
	}
}

func TestClusterTags_SkipsStubEmbedderAndPropagatesRealError(t *testing.T) {
	ctx := context.Background()
	stats := []store.TagStat{
		{Tag: "alpha", Count: 2},
		{Tag: "omega", Count: 2},
	}

	// StubEmbedder (directly or wrapped in CachingEmbedder) should be skipped safely.
	c, err := ClusterTags(ctx, stats, Options{Embedder: embed.NewCachingEmbedder(embed.NewStub(384), 16)})
	if err != nil || len(c) != 0 {
		t.Fatalf("stub embedder run: clusters=%v err=%v", c, err)
	}

	// Real embedder errors are returned.
	_, err = ClusterTags(ctx, stats, Options{Embedder: &mockEmbedder{err: errors.New("boom")}})
	if err == nil {
		t.Fatal("expected embedder error to propagate")
	}
}

func TestClusterTags_SkipsONNXStubFallbackOnManyUnrelatedTags(t *testing.T) {
	ctx := context.Background()
	modelPath := filepath.Join(t.TempDir(), "bge-small-en-v1.5.onnx")
	if err := os.WriteFile(modelPath, []byte("placeholder"), 0600); err != nil {
		t.Fatalf("write model placeholder: %v", err)
	}
	emb, err := embed.New(modelPath, 384, "")
	if err != nil {
		t.Fatalf("embed.New: %v", err)
	}
	defer emb.Close()

	if !embed.IsStub(emb) {
		t.Fatalf("expected embed.IsStub to be true for stub-backed ONNXEmbedder")
	}

	// 45 unrelated tags would collide under 384-dim FNV-32 stub hashing (birthday paradox).
	words := []string{
		"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel", "india",
		"juliet", "kilo", "lima", "mike", "november", "oscar", "papa", "quebec", "romeo",
		"sierra", "tango", "uniform", "victor", "whiskey", "xray", "yankee", "zulu",
		"billing", "caching", "deploy", "frontend", "graphql", "harness", "ingress", "journal",
		"kernel", "logging", "metrics", "network", "observability", "payments", "queue",
		"routing", "security", "telemetry", "webhooks",
	}
	stats := make([]store.TagStat, 0, len(words))
	for _, w := range words {
		stats = append(stats, store.TagStat{Tag: w, Count: 2})
	}

	clusters, err := ClusterTags(ctx, stats, Options{
		Embedder: embed.NewCachingEmbedder(emb, 64),
	})
	if err != nil {
		t.Fatalf("ClusterTags: %v", err)
	}
	if len(clusters) != 0 {
		t.Fatalf("expected 0 clusters for unrelated tags with ONNX stub fallback, got %+v", clusters)
	}
}

func TestClusterTags_DoesNotMergeVersionOrNumberedTags(t *testing.T) {
	ctx := context.Background()
	stats := []store.TagStat{
		{Tag: "phase-1", Count: 5},
		{Tag: "phase-2", Count: 4},
		{Tag: "phase-3", Count: 3},
		{Tag: "v2.1.0", Count: 2},
		{Tag: "v2.2.0", Count: 2},
		{Tag: "sprint-1", Count: 3},
		{Tag: "sprint-2", Count: 3},
		{Tag: "sprint-10", Count: 3},
		{Tag: "http2", Count: 2},
		{Tag: "http3", Count: 2},
		{Tag: "pr-101", Count: 1},
		{Tag: "pr-102", Count: 1},
	}

	clusters, err := ClusterTags(ctx, stats, Options{})
	if err != nil {
		t.Fatalf("ClusterTags: %v", err)
	}
	if len(clusters) != 0 {
		t.Fatalf("expected 0 clusters for numbered/versioned tags, got %+v", clusters)
	}
}
