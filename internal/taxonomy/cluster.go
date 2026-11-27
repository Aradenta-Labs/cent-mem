package taxonomy

import (
	"context"
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/aradenta-labs/cent-mem/internal/embed"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

// DefaultSimilarityThreshold is the default cosine similarity threshold for Pass 2 semantic clustering.
const DefaultSimilarityThreshold = 0.86

// Options configures tag clustering passes.
type Options struct {
	// SimilarityThreshold sets the minimum cosine similarity for Pass 2 semantic clustering.
	// Defaults to DefaultSimilarityThreshold (0.86) when <= 0.
	SimilarityThreshold float64
	// Embedder is used for Pass 2 semantic clustering. When nil or a hash-based stub
	// (embed.IsStub), Pass 2 is skipped to avoid random hash-collision false positives.
	Embedder embed.Embedder
}

// Cluster represents a group of synonymous or near-duplicate tags with a chosen canonical form.
type Cluster struct {
	Canonical     string   `json:"canonical"`
	Aliases       []string `json:"aliases"`
	Members       []string `json:"members"`
	AffectedCount int      `json:"affected_count"`
	MatchMethod   string   `json:"match_method"` // "lexical" | "semantic" | "hybrid"
	Similarity    float64  `json:"similarity,omitempty"`
}

// ClusterTags groups fragmented tags from stats using:
//   - Pass 1 (lexical, offline): case folding, separator normalization, singular/plural stemming,
//     and Levenshtein distance <= 1 for tags with rune length >= 5.
//   - Pass 2 (semantic, optional embedder): cosine similarity >= threshold (default 0.86).
//
// Only clusters containing at least 2 distinct tags are returned, sorted deterministically by
// AffectedCount DESC, then Canonical ASC.
func ClusterTags(ctx context.Context, stats []store.TagStat, opts Options) ([]Cluster, error) {
	if len(stats) < 2 {
		return []Cluster{}, nil
	}

	// Copy and sort input deterministically so Union-Find order never affects output.
	items := make([]store.TagStat, 0, len(stats))
	seen := make(map[string]int, len(stats))
	for _, st := range stats {
		if strings.TrimSpace(st.Tag) == "" {
			continue
		}
		if idx, ok := seen[st.Tag]; ok {
			items[idx].Count += st.Count
			if st.LastUsed.After(items[idx].LastUsed) {
				items[idx].LastUsed = st.LastUsed
			}
			continue
		}
		seen[st.Tag] = len(items)
		items = append(items, st)
	}
	if len(items) < 2 {
		return []Cluster{}, nil
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Tag < items[j].Tag
	})

	n := len(items)
	uf := newUnionFind(n)
	lexicalMerged := make([]bool, n)
	semanticMerged := make([]bool, n)
	maxSim := make([]float64, n)

	// Precompute normalized forms for Pass 1.
	normSep := make([]string, n)
	stemmed := make([]string, n)
	runeLen := make([]int, n)
	for i, it := range items {
		ns := normalizeSeparators(it.Tag)
		normSep[i] = ns
		stemmed[i] = stemSingular(ns)
		runeLen[i] = utf8.RuneCountInString(ns)
	}

	// Pass 1: Lexical normalization.
	// shortcut: O(N^2) pairwise scan over distinct tags (fine up to ~2k tags), index by normalized/stemmed key and BK-tree when tag vocabularies grow larger.
	for i := 0; i < n; i++ {
		if normSep[i] == "" {
			continue
		}
		for j := i + 1; j < n; j++ {
			if normSep[j] == "" {
				continue
			}
			if normSep[i] == normSep[j] ||
				stemmed[i] == stemmed[j] ||
				(runeLen[i] >= 5 && runeLen[j] >= 5 && levenshteinAtMost1(normSep[i], normSep[j])) {
				uf.union(i, j)
				lexicalMerged[i] = true
				lexicalMerged[j] = true
			}
		}
	}

	// Pass 2: Semantic similarity via embedder (when non-nil and not a hash stub).
	threshold := opts.SimilarityThreshold
	if threshold <= 0 {
		threshold = DefaultSimilarityThreshold
	}
	if opts.Embedder != nil && !embed.IsStub(opts.Embedder) {
		texts := make([]string, n)
		for i, ns := range normSep {
			// Present tag words cleanly to sentence-transformer models.
			texts[i] = strings.ReplaceAll(ns, "-", " ")
		}
		vecs, err := opts.Embedder.Embed(ctx, texts)
		if err != nil {
			return nil, err
		}
		if len(vecs) == n {
			// shortcut: O(N^2) pairwise cosine similarity (fine up to ~2k tags), switch to ANN/HNSW or LSH bucketing if distinct tags exceed a few thousand.
			for i := 0; i < n; i++ {
				if isZeroVec(vecs[i]) {
					continue
				}
				for j := i + 1; j < n; j++ {
					if isZeroVec(vecs[j]) {
						continue
					}
					sim := cosineSimilarity(vecs[i], vecs[j])
					if sim >= threshold {
						uf.union(i, j)
						semanticMerged[i] = true
						semanticMerged[j] = true
						if sim > maxSim[i] {
							maxSim[i] = sim
						}
						if sim > maxSim[j] {
							maxSim[j] = sim
						}
					}
				}
			}
		}
	}

	// Group indices by connected component root.
	groups := make(map[int][]int)
	for i := 0; i < n; i++ {
		r := uf.find(i)
		groups[r] = append(groups[r], i)
	}

	clusters := make([]Cluster, 0)
	for _, idxs := range groups {
		if len(idxs) < 2 {
			continue
		}

		// Pick canonical: highest frequency; tie-break: lowercase kebab-case, shortest length, lexicographic.
		canonIdx := idxs[0]
		for _, idx := range idxs[1:] {
			if isBetterCanonical(items[idx], items[canonIdx]) {
				canonIdx = idx
			}
		}

		canonical := items[canonIdx].Tag
		aliases := make([]string, 0, len(idxs)-1)
		members := make([]string, 0, len(idxs))
		affected := 0
		hasLex := false
		hasSem := false
		bestSim := 0.0

		for _, idx := range idxs {
			tag := items[idx].Tag
			members = append(members, tag)
			if idx != canonIdx {
				aliases = append(aliases, tag)
				affected += items[idx].Count
			}
			if lexicalMerged[idx] {
				hasLex = true
			}
			if semanticMerged[idx] {
				hasSem = true
				if maxSim[idx] > bestSim {
					bestSim = maxSim[idx]
				}
			}
		}

		sort.Strings(aliases)
		sort.Strings(members)

		method := "lexical"
		if hasLex && hasSem {
			method = "hybrid"
		} else if hasSem {
			method = "semantic"
		}

		c := Cluster{
			Canonical:     canonical,
			Aliases:       aliases,
			Members:       members,
			AffectedCount: affected,
			MatchMethod:   method,
		}
		if hasSem {
			c.Similarity = math.Round(bestSim*1000) / 1000
		}
		clusters = append(clusters, c)
	}

	sort.Slice(clusters, func(i, j int) bool {
		if clusters[i].AffectedCount != clusters[j].AffectedCount {
			return clusters[i].AffectedCount > clusters[j].AffectedCount
		}
		return clusters[i].Canonical < clusters[j].Canonical
	})

	return clusters, nil
}

// isBetterCanonical reports whether a should be chosen over b as the cluster's canonical tag.
func isBetterCanonical(a, b store.TagStat) bool {
	if a.Count != b.Count {
		return a.Count > b.Count
	}
	aKebab := isLowerKebab(a.Tag)
	bKebab := isLowerKebab(b.Tag)
	if aKebab != bKebab {
		return aKebab
	}
	aLen := utf8.RuneCountInString(a.Tag)
	bLen := utf8.RuneCountInString(b.Tag)
	if aLen != bLen {
		return aLen < bLen
	}
	return a.Tag < b.Tag
}

func isLowerKebab(s string) bool {
	if s == "" {
		return false
	}
	return s == normalizeSeparators(s)
}

// normalizeSeparators lowercases s, maps underscores/hyphens/whitespace to '-', and collapses runs of '-'.
func normalizeSeparators(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	b.Grow(len(s))
	prevDash := true // skip leading dashes
	for _, r := range s {
		if r == '_' || r == '-' || unicode.IsSpace(r) {
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
			continue
		}
		b.WriteRune(r)
		prevDash = false
	}
	return strings.TrimRight(b.String(), "-")
}

// stemSingular applies conservative English singularization to the final hyphen-separated token.
func stemSingular(norm string) string {
	if norm == "" {
		return ""
	}
	parts := strings.Split(norm, "-")
	last := parts[len(parts)-1]
	parts[len(parts)-1] = singularWord(last)
	return strings.Join(parts, "-")
}

func singularWord(w string) string {
	n := len(w)
	if n < 4 {
		return w
	}
	if strings.HasSuffix(w, "ies") && n >= 5 {
		return w[:n-3] + "y"
	}
	for _, suf := range []string{"sses", "xes", "zes", "shes", "tches", "nches", "rches"} {
		if strings.HasSuffix(w, suf) && n > len(suf) {
			return w[:n-2]
		}
	}
	if strings.HasSuffix(w, "s") &&
		!strings.HasSuffix(w, "ss") &&
		!strings.HasSuffix(w, "us") &&
		!strings.HasSuffix(w, "is") {
		return w[:n-1]
	}
	return w
}

// levenshteinAtMost1 returns true iff the rune edit distance between a and b is <= 1
// and the single differing rune is not a digit (avoiding false merges on version/phase tags
// such as phase-1/phase-2, v2.1.0/v2.2.0, or sprint-1/sprint-10).
func levenshteinAtMost1(a, b string) bool {
	ra := []rune(a)
	rb := []rune(b)
	la, lb := len(ra), len(rb)
	if la-lb > 1 || lb-la > 1 {
		return false
	}
	if la == lb {
		diff := 0
		for i := 0; i < la; i++ {
			if ra[i] != rb[i] {
				if unicode.IsDigit(ra[i]) || unicode.IsDigit(rb[i]) {
					return false
				}
				diff++
				if diff > 1 {
					return false
				}
			}
		}
		return true
	}
	// Ensure ra is the shorter slice.
	if la > lb {
		ra, rb = rb, ra
		la, lb = lb, la
	}
	i, j := 0, 0
	skipped := false
	for i < la && j < lb {
		if ra[i] == rb[j] {
			i++
			j++
			continue
		}
		if skipped || unicode.IsDigit(rb[j]) {
			return false
		}
		skipped = true
		j++
	}
	if !skipped && j < lb && unicode.IsDigit(rb[j]) {
		return false
	}
	return true
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		fa, fb := float64(a[i]), float64(b[i])
		dot += fa * fb
		normA += fa * fa
		normB += fb * fb
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func isZeroVec(v []float32) bool {
	for _, x := range v {
		if x != 0 {
			return false
		}
	}
	return true
}

type unionFind struct {
	parent []int
}

func newUnionFind(n int) *unionFind {
	p := make([]int, n)
	for i := range p {
		p[i] = i
	}
	return &unionFind{parent: p}
}

func (u *unionFind) find(x int) int {
	for u.parent[x] != x {
		u.parent[x] = u.parent[u.parent[x]]
		x = u.parent[x]
	}
	return x
}

func (u *unionFind) union(a, b int) {
	ra, rb := u.find(a), u.find(b)
	if ra == rb {
		return
	}
	if ra < rb {
		u.parent[rb] = ra
	} else {
		u.parent[ra] = rb
	}
}
