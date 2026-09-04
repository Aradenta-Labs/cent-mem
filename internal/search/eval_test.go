package search_test

import (
	"context"
	"fmt"
	"math"
	"sort"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/embed"
	"github.com/aradenta-labs/cent-mem/internal/search"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

// goldQuery defines a test query with known ground-truth relevance judgements.
// Relevance scale: 3 = exact answer, 2 = highly relevant, 1 = related, 0 = irrelevant.
type goldQuery struct {
	text          string
	scope         string
	callerAgent   string
	inherit       bool
	isSessionTest bool
	relevance     map[string]int // maps expected content substring or key to relevance grade
}

// TestEvaluation_MRR_NDCG evaluates Phase C recall accuracy on a synthetic
// multi-agent, multi-session corpus.
// Target SLAs:
// - MRR@5 >= 0.85
// - NDCG@5 >= 0.80
// - Caller Session Precision@1 >= 0.90
func TestEvaluation_MRR_NDCG(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{DBPath: dir + "/centmem.db"}
	s, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer s.Close()

	ctx := context.Background()

	// Seed 20+ curated memories spanning multiple agents, sessions, and tags.
	type seedMem struct {
		scopePath   string
		typ         string
		key         string
		content     string
		tags        []string
		sourceAgent string
		hoursAgo    int
	}

	seeds := []seedMem{
		// SQLite / Database pool
		{
			scopePath:   "project:cent-mem",
			typ:         "fact",
			key:         "db.pool_size",
			content:     "Set max_open_conns to 25 to prevent SQLite lock contention.",
			tags:        []string{"database", "sqlite", "performance", "config"},
			sourceAgent: "claude",
			hoursAgo:    10,
		},
		{
			scopePath:   "project:cent-mem",
			typ:         "note",
			content:     "SQLite busy_timeout configured to 5000ms for WAL concurrency.",
			tags:        []string{"database", "sqlite", "wal"},
			sourceAgent: "codex",
			hoursAgo:    12,
		},
		{
			scopePath:   "project:cent-mem/agent:claude/session:task-1",
			typ:         "note",
			content:     "Active investigation into sqlite database pool exhaustion during concurrent benchmarks.",
			tags:        []string{"database", "sqlite", "benchmark"},
			sourceAgent: "claude",
			hoursAgo:    1,
		},
		// Architecture / Search
		{
			scopePath:   "project:cent-mem",
			typ:         "note",
			content:     "Hybrid search combines BM25 keyword matching, facts lookup, and sqlite-vec cosine KNN.",
			tags:        []string{"search", "architecture", "hybrid"},
			sourceAgent: "trae",
			hoursAgo:    20,
		},
		{
			scopePath:   "project:cent-mem",
			typ:         "note",
			content:     "Phase C implements two-stage composite reranker blending dense semantic and lexical coverage.",
			tags:        []string{"search", "rerank", "v1.4.4"},
			sourceAgent: "claude",
			hoursAgo:    2,
		},
		{
			scopePath:   "project:cent-mem/agent:claude/session:task-2",
			typ:         "note",
			content:     "Session specific debug notes for two-stage composite reranker score calibration.",
			tags:        []string{"search", "rerank", "debug"},
			sourceAgent: "claude",
			hoursAgo:    1,
		},
		// Security / API keys
		{
			scopePath:   "global",
			typ:         "fact",
			key:         "security.auth_header",
			content:     "Always pass Bearer token in Authorization header for internal endpoints.",
			tags:        []string{"security", "auth", "api"},
			sourceAgent: "security-bot",
			hoursAgo:    50,
		},
		{
			scopePath:   "project:cent-mem",
			typ:         "note",
			content:     "Never persist plaintext API keys to config.toml; use capture.api_key_env instead.",
			tags:        []string{"security", "config", "byok"},
			sourceAgent: "codex",
			hoursAgo:    15,
		},
		// Retention / Compaction
		{
			scopePath:   "project:cent-mem",
			typ:         "fact",
			key:         "retention.notes_days",
			content:     "Notes summarize after 30 days of inactivity.",
			tags:        []string{"retention", "compact", "notes"},
			sourceAgent: "trae",
			hoursAgo:    40,
		},
		{
			scopePath:   "project:cent-mem",
			typ:         "note",
			content:     "Compaction daemon scans eligible memories and consolidates daily notes.",
			tags:        []string{"compaction", "summarization", "daemon"},
			sourceAgent: "codex",
			hoursAgo:    8,
		},
		// Session Proximity specific seeds
		{
			scopePath:   "project:cent-mem/agent:claude/session:task-3",
			typ:         "note",
			content:     "Immediate working memory: currently refactoring boost calculation in internal/search/boost.go",
			tags:        []string{"search", "boost", "refactor"},
			sourceAgent: "claude",
			hoursAgo:    0,
		},
		{
			scopePath:   "project:cent-mem",
			typ:         "note",
			content:     "General documentation on boost calculation and proximity tiers across scopes.",
			tags:        []string{"search", "boost", "docs"},
			sourceAgent: "trae",
			hoursAgo:    24,
		},
		// Paraphrase & Embeddings
		{
			scopePath:   "project:cent-mem",
			typ:         "note",
			content:     "Embedding version 2 uses structured prefix formatting with tags and key.",
			tags:        []string{"embedding", "schema", "v1.4.4"},
			sourceAgent: "claude",
			hoursAgo:    5,
		},
		{
			scopePath:   "project:cent-mem",
			typ:         "fact",
			key:         "model.default",
			content:     "bge-small-en-v1.5 with 384 dimensions.",
			tags:        []string{"model", "onnx", "embedding"},
			sourceAgent: "trae",
			hoursAgo:    30,
		},
		// Noise memories
		{
			scopePath:   "global",
			typ:         "log",
			content:     "System daemon started successfully at boot time.",
			tags:        []string{"sys", "boot"},
			sourceAgent: "system",
			hoursAgo:    100,
		},
		{
			scopePath:   "global",
			typ:         "log",
			content:     "Heartbeat check passed with zero errors.",
			tags:        []string{"sys", "heartbeat"},
			sourceAgent: "system",
			hoursAgo:    50,
		},
		{
			scopePath:   "project:other-proj",
			typ:         "note",
			content:     "Unrelated project notes regarding front-end react UI component styling.",
			tags:        []string{"frontend", "react"},
			sourceAgent: "alice",
			hoursAgo:    5,
		},
	}

	for _, sm := range seeds {
		if sm.typ == "fact" {
			_, _, err := s.SetFact(ctx, store.FactInput{
				Scope:       sm.scopePath,
				Key:         sm.key,
				Value:       `"` + sm.content + `"`,
				Tags:        sm.tags,
				SourceAgent: sm.sourceAgent,
			})
			if err != nil {
				t.Fatalf("seed fact %s: %v", sm.key, err)
			}
		} else {
			_, _, err := s.PutMemory(ctx, store.MemoryInput{
				Scope:       sm.scopePath,
				Type:        sm.typ,
				Content:     sm.content,
				Tags:        sm.tags,
				SourceAgent: sm.sourceAgent,
			})
			if err != nil {
				t.Fatalf("seed memory: %v", err)
			}
		}
	}

	// Drain queue with deterministic stub embedder
	stubEmb := embed.NewStub(384)
	q := embed.NewQueue(s, stubEmb)
	q.MaxTime = -1
	if _, err := q.Drain(ctx); err != nil {
		t.Fatalf("Drain queue: %v", err)
	}

	searcher := search.New(s).
		WithEmbedder(stubEmb).
		WithSessionBoost(1.25).
		WithAgentBoost(1.15)

	queries := []goldQuery{
		{
			text:        "sqlite pool size setting",
			scope:       "project:cent-mem",
			callerAgent: "claude",
			inherit:     true,
			relevance: map[string]int{
				"db.pool_size": 3,
				"busy_timeout": 1,
			},
		},
		{
			text:        "two-stage composite reranker",
			scope:       "project:cent-mem",
			callerAgent: "claude",
			inherit:     true,
			relevance: map[string]int{
				"composite reranker blending": 3,
				"reranker score calibration":  2,
				"Hybrid search":               1,
			},
		},
		{
			text:        "refactoring boost calculation",
			scope:       "project:cent-mem/agent:claude/session:task-3",
			callerAgent: "claude",
			inherit:     true,
			isSessionTest: true,
			relevance: map[string]int{
				"Immediate working memory": 3,
				"General documentation":    1,
			},
		},
		{
			text:        "security auth header token",
			scope:       "project:cent-mem",
			callerAgent: "codex",
			inherit:     true,
			relevance: map[string]int{
				"security.auth_header": 3,
				"plaintext API keys":   1,
			},
		},
		{
			text:        "never persist plaintext API keys",
			scope:       "project:cent-mem",
			callerAgent: "codex",
			inherit:     true,
			relevance: map[string]int{
				"plaintext API keys":   3,
				"security.auth_header": 1,
			},
		},
		{
			text:        "notes summarize after days",
			scope:       "project:cent-mem",
			callerAgent: "trae",
			inherit:     true,
			relevance: map[string]int{
				"retention.notes_days": 3,
				"Compaction daemon":    1,
			},
		},
		{
			text:        "structured prefix formatting tags",
			scope:       "project:cent-mem",
			callerAgent: "claude",
			inherit:     true,
			relevance: map[string]int{
				"structured prefix formatting": 3,
				"model.default":                1,
			},
		},
		{
			text:        "sqlite lock contention",
			scope:       "project:cent-mem",
			callerAgent: "claude",
			inherit:     true,
			relevance: map[string]int{
				"db.pool_size": 3,
				"busy_timeout": 1,
			},
		},
		{
			text:        "compaction daemon consolidates daily notes",
			scope:       "project:cent-mem",
			callerAgent: "codex",
			inherit:     true,
			relevance: map[string]int{
				"Compaction daemon":    3,
				"retention.notes_days": 1,
			},
		},
		{
			text:        "hybrid search bm25 keyword",
			scope:       "project:cent-mem",
			callerAgent: "trae",
			inherit:     true,
			relevance: map[string]int{
				"Hybrid search":               3,
				"composite reranker blending": 1,
			},
		},
	}

	var totalRR float64
	var totalNDCG float64
	var sessionHits int
	var sessionTotal int

	for _, gq := range queries {
		results, err := searcher.Recall(ctx, search.Query{
			Text:        gq.text,
			Scope:       gq.scope,
			CallerAgent: gq.callerAgent,
			Inherit:     gq.inherit,
			Top:         5,
		})
		if err != nil {
			t.Fatalf("query %q: %v", gq.text, err)
		}

		// Calculate MRR@5 and NDCG@5
		var rr float64
		dcg := 0.0
		var grades []int

		for rank, r := range results {
			rel := 0
			for pattern, grade := range gq.relevance {
				if r.Key == pattern || containsSubstring(r.Content, pattern) {
					if grade > rel {
						rel = grade
					}
				}
			}
			grades = append(grades, rel)

			if rel > 0 && rr == 0 {
				rr = 1.0 / float64(rank+1)
			}
			if rank < 5 {
				dcg += (math.Pow(2, float64(rel)) - 1) / math.Log2(float64(rank+2))
			}
		}

		// Calculate Ideal DCG@5
		idealGrades := make([]int, 0, len(gq.relevance))
		for _, g := range gq.relevance {
			idealGrades = append(idealGrades, g)
		}
		sort.Slice(idealGrades, func(i, j int) bool { return idealGrades[i] > idealGrades[j] })
		idcg := 0.0
		for i := 0; i < min(5, len(idealGrades)); i++ {
			idcg += (math.Pow(2, float64(idealGrades[i])) - 1) / math.Log2(float64(i+2))
		}

		ndcg := 1.0
		if idcg > 0 {
			ndcg = dcg / idcg
		}

		totalRR += rr
		totalNDCG += ndcg

		if gq.isSessionTest {
			sessionTotal++
			if len(results) > 0 && results[0].Scope == gq.scope {
				sessionHits++
			}
		}
	}

	mrr5 := totalRR / float64(len(queries))
	ndcg5 := totalNDCG / float64(len(queries))

	t.Logf("=== Evaluation Benchmark Results ===")
	t.Logf("MRR@5:  %.4f (target >= 0.85)", mrr5)
	t.Logf("NDCG@5: %.4f (target >= 0.80)", ndcg5)
	if sessionTotal > 0 {
		sessionPrecision := float64(sessionHits) / float64(sessionTotal)
		t.Logf("Session Precision@1: %.4f (target >= 0.90)", sessionPrecision)
		if sessionPrecision < 0.90 {
			t.Errorf("Session Precision@1 = %.4f < 0.90", sessionPrecision)
		}
	}

	if mrr5 < 0.85 {
		t.Errorf("MRR@5 = %.4f < target 0.85", mrr5)
	}
	if ndcg5 < 0.80 {
		t.Errorf("NDCG@5 = %.4f < target 0.80", ndcg5)
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(substr) > 0 && len(s) > 0 && fmt.Sprintf("%s", s) != "" && containsFold(s, substr)))
}

func containsFold(s, substr string) bool {
	return search.ContainsFold(s, substr)
}
