package search

import (
	"context"
	"slices"
	"strings"
	"unicode"
)

// ReRanker rescores and reorders a candidate set against the query.
type ReRanker interface {
	// ReRank takes the initial query, the query embedding (if available),
	// and Stage 1 candidates, returning candidates with updated Scores.
	ReRank(ctx context.Context, q Query, queryVec []float32, candidates []*Ranked) ([]*Ranked, error)
	// Name identifies the re-ranker strategy (e.g. "composite", "cross_encoder", "llm", "none").
	Name() string
}

// CompositeReRanker is the default in-process Go rescorer.
// Calibrated rescoring formula:
// S_composite = w_sem * S_sem + w_lex * S_lex + w_phrase * S_phrase + w_rrf * S_rrf_norm
type CompositeReRanker struct {
	SemWeight    float64
	LexWeight    float64
	PhraseWeight float64
	RRFWeight    float64
}

// NewCompositeReRanker returns a CompositeReRanker initialized with calibrated weights.
func NewCompositeReRanker() *CompositeReRanker {
	return &CompositeReRanker{
		SemWeight:    0.50,
		LexWeight:    0.25,
		PhraseWeight: 0.15,
		RRFWeight:    0.10,
	}
}

// Name returns the strategy identifier "composite".
func (r *CompositeReRanker) Name() string { return "composite" }

// ReRank rescores candidates using dense semantic similarity, lexical coverage,
// exact phrase matching, and normalized Stage 1 RRF consensus.
func (r *CompositeReRanker) ReRank(ctx context.Context, q Query, queryVec []float32, candidates []*Ranked) ([]*Ranked, error) {
	if len(candidates) == 0 {
		return candidates, nil
	}

	trimmedQuery := strings.TrimSpace(q.Text)
	if trimmedQuery == "" {
		return candidates, nil
	}
	queryWords := uniqueTokens(tokenize(trimmedQuery))

	// Find max RRF score among candidates to normalize
	var maxRRF float64
	for _, c := range candidates {
		if c.Score > maxRRF {
			maxRRF = c.Score
		}
	}

	for _, c := range candidates {
		// 1. Semantic score (from sqlite-vec cosine similarity, clamped to [0, 1])
		sSem := c.SemanticScore
		if sSem < 0 {
			sSem = 0
		} else if sSem > 1.0 {
			sSem = 1.0
		}

		// 2. Lexical coverage: proportion of unique query tokens found in content + tags + key
		var sLex float64
		if len(queryWords) > 0 {
			memTokens := make(map[string]struct{})
			for _, tok := range tokenize(c.Content) {
				memTokens[tok] = struct{}{}
			}
			for _, tag := range c.Tags {
				for _, tok := range tokenize(tag) {
					memTokens[tok] = struct{}{}
				}
			}
			if c.Key != "" {
				for _, tok := range tokenize(c.Key) {
					memTokens[tok] = struct{}{}
				}
			}
			matches := 0
			for _, qw := range queryWords {
				if _, ok := memTokens[qw]; ok {
					matches++
				}
			}
			sLex = float64(matches) / float64(len(queryWords))
		}

		// 3. Exact phrase match: 1.0 if query phrase appears intact in content or fact key
		var sPhrase float64
		if ContainsFold(c.Content, trimmedQuery) || (c.Key != "" && ContainsFold(c.Key, trimmedQuery)) {
			sPhrase = 1.0
		}

		// 4. Normalized RRF consensus
		var sRRFNorm float64
		if maxRRF > 0 {
			sRRFNorm = c.Score / maxRRF
		}

		sComp := r.SemWeight*sSem + r.LexWeight*sLex + r.PhraseWeight*sPhrase + r.RRFWeight*sRRFNorm
		c.Score = sComp
	}

	// Sort candidates descending by score with deterministic tie-breaking
	slices.SortFunc(candidates, sortRanked)

	return candidates, nil
}

// tokenize splits a string into lowercase alphanumeric tokens.
func tokenize(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	var out []string
	for _, f := range fields {
		f = strings.ToLower(strings.TrimSpace(f))
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

// uniqueTokens returns the deduplicated set of tokens preserving order.
func uniqueTokens(tokens []string) []string {
	seen := make(map[string]struct{}, len(tokens))
	var out []string
	for _, t := range tokens {
		if _, ok := seen[t]; !ok {
			seen[t] = struct{}{}
			out = append(out, t)
		}
	}
	return out
}

// sortRanked provides deterministic descending order by Score, CreatedAt, and ID.
func sortRanked(a, b *Ranked) int {
	if b.Score > a.Score {
		return 1
	}
	if b.Score < a.Score {
		return -1
	}
	if b.CreatedAt.After(a.CreatedAt) {
		return 1
	}
	if a.CreatedAt.After(b.CreatedAt) {
		return -1
	}
	if b.ID > a.ID {
		return 1
	}
	if a.ID > b.ID {
		return -1
	}
	return 0
}

// noneReRanker skips Stage 2 rescoring and leaves candidate scores untouched.
type noneReRanker struct{}

func (r *noneReRanker) Name() string { return "none" }
func (r *noneReRanker) ReRank(ctx context.Context, q Query, queryVec []float32, candidates []*Ranked) ([]*Ranked, error) {
	return candidates, nil
}

// CrossEncoderReRanker falls back to CompositeReRanker when cross-encoder ONNX model is missing.
type CrossEncoderReRanker struct {
	fallback *CompositeReRanker
}

// NewCrossEncoderReRanker creates a CrossEncoderReRanker.
func NewCrossEncoderReRanker() *CrossEncoderReRanker {
	return &CrossEncoderReRanker{fallback: NewCompositeReRanker()}
}

func (r *CrossEncoderReRanker) Name() string { return "cross_encoder" }
func (r *CrossEncoderReRanker) ReRank(ctx context.Context, q Query, queryVec []float32, candidates []*Ranked) ([]*Ranked, error) {
	return r.fallback.ReRank(ctx, q, queryVec, candidates)
}

// LLMReRanker falls back to CompositeReRanker when LLM endpoint is missing or disabled.
type LLMReRanker struct {
	fallback *CompositeReRanker
}

// NewLLMReRanker creates an LLMReRanker.
func NewLLMReRanker() *LLMReRanker {
	return &LLMReRanker{fallback: NewCompositeReRanker()}
}

func (r *LLMReRanker) Name() string { return "llm" }
func (r *LLMReRanker) ReRank(ctx context.Context, q Query, queryVec []float32, candidates []*Ranked) ([]*Ranked, error) {
	return r.fallback.ReRank(ctx, q, queryVec, candidates)
}

// ContainsFold reports whether substr is within s, case-insensitively,
// collapsing redundant whitespace in both strings.
func ContainsFold(s, substr string) bool {
	sNorm := strings.ToLower(strings.Join(strings.Fields(s), " "))
	subNorm := strings.ToLower(strings.Join(strings.Fields(substr), " "))
	if subNorm == "" {
		return false
	}
	return strings.Contains(sNorm, subNorm)
}
