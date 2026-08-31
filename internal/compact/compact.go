// Package compact implements memory retention policies and summarization.
//
// Long-term growth is controlled by compacting old memories: eligible rows
// (those past their summarize_at threshold) are grouped by scope+type,
// summarized into a single consolidated note, and the originals are archived
// (soft-deleted, with their embeddings/vectors dropped). Archived rows remain
// in the `memories` table and the audit `events` log, but are excluded from
// search.
package compact

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/farras/cent-mem/internal/store"
)

// Summarizer produces a consolidated summary from a group of memories.
type Summarizer interface {
	// Summarize reduces memories into a single summary string. It must be
	// deterministic for the same input (the default HeuristicSummarizer is).
	Summarize(ctx context.Context, memories []store.Memory) (string, error)
}

// Policy captures retention thresholds used to decide which rows are eligible
// for compaction. It mirrors the `[retention]` config block.
type Policy struct {
	// FactKeepForever disables compaction of facts when true (the default).
	FactKeepForever bool
	// NoteSummarizeAfter, LogSummarizeAfter are the number of days a note/log
	// must be older than `summarize_at` before it is eligible. The summary
	// threshold is computed as created_at + these days when summarize_at is
	// not already set.
	NoteSummarizeAfterDays int
	LogSummarizeAfterDays  int
}

// Result reports the outcome of a compaction run.
type Result struct {
	Summarized   int     `json:"summarized"`
	Archived     int     `json:"archived"`
	NewMemoryIDs []int64 `json:"new_memory_ids"`
}

// Options configures a compaction run.
type Options struct {
	// Scope restricts compaction to memories under a single scope path
	// (including its descendants). Empty means all scopes.
	Scope string
	// DryRun reports what would be done without writing anything.
	DryRun bool
	// Now is the reference time for eligibility (summarize_at <= Now).
	// Defaults to time.Now() when zero.
	Now time.Time
	// Policy carries retention thresholds. When nil, a default policy is used
	// (facts kept forever; notes after 30d; logs after 14d).
	Policy *Policy
}

// DefaultPolicy returns the standard retention policy used when none is given.
func DefaultPolicy() Policy {
	return Policy{
		FactKeepForever:        true,
		NoteSummarizeAfterDays: 30,
		LogSummarizeAfterDays:  14,
	}
}

// Compact selects eligible memories, groups them by scope+type, summarizes each
// group into a consolidated note, and archives the originals. When opts.DryRun
// is true it returns the would-be Result without writing anything.
func Compact(ctx context.Context, st *store.Store, opts Options) (Result, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	policy := DefaultPolicy()
	if opts.Policy != nil {
		policy = *opts.Policy
	}

	eligible, err := st.EligibleMemories(ctx, opts.Scope, now)
	if err != nil {
		return Result{}, err
	}
	// Facts are never compacted when the policy says keep them forever.
	if policy.FactKeepForever {
		filtered := eligible[:0]
		for _, m := range eligible {
			if m.Type != "fact" {
				filtered = append(filtered, m)
			}
		}
		eligible = filtered
	}
	if len(eligible) == 0 {
		return Result{}, nil
	}

	// Group by (scope_id, type).
	type groupKey struct {
		scopeID int64
		typ     string
	}
	groups := map[groupKey][]store.Memory{}
	var order []groupKey
	for _, m := range eligible {
		k := groupKey{m.ScopeID, m.Type}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], m)
	}

	sum := NewHeuristicSummarizer(0)
	var res Result

	for _, k := range order {
		mems := groups[k]
		summary, err := sum.Summarize(ctx, mems)
		if err != nil {
			return res, fmt.Errorf("compact: summarize group scope=%d type=%s: %w", k.scopeID, k.typ, err)
		}
		if opts.DryRun {
			res.Summarized += len(mems)
			res.Archived += len(mems)
			continue
		}

		newID, err := st.InsertConsolidatedNote(ctx, mems, summary, now)
		if err != nil {
			return res, err
		}
		res.Summarized += len(mems)
		res.Archived += len(mems)
		res.NewMemoryIDs = append(res.NewMemoryIDs, newID)
	}

	return res, nil
}

// ---------------------------------------------------------------------------
// HeuristicSummarizer
// ---------------------------------------------------------------------------

// HeuristicSummarizer is the default, deterministic, offline summarizer. It
// extracts the most frequent unique tokens across the input memories and emits
// a compact, stateless summary header plus the top sentences.
type HeuristicSummarizer struct {
	// MaxSentences caps how many input sentences are echoed in the summary.
	MaxSentences int
}

// NewHeuristicSummarizer returns a HeuristicSummarizer with a sane default.
func NewHeuristicSummarizer(maxSentences int) *HeuristicSummarizer {
	if maxSentences <= 0 {
		maxSentences = 3
	}
	return &HeuristicSummarizer{MaxSentences: maxSentences}
}

// Summarize implements Summarizer. It is deterministic: identical input yields
// identical output for a fixed MaxSentences.
func (h *HeuristicSummarizer) Summarize(ctx context.Context, memories []store.Memory) (string, error) {
	if len(memories) == 0 {
		return "", nil
	}

	// Sort by created_at so the header range and sentence order are stable.
	sorted := make([]store.Memory, len(memories))
	copy(sorted, memories)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].CreatedAt.Before(sorted[j].CreatedAt) })

	first := sorted[0].CreatedAt
	last := sorted[len(sorted)-1].CreatedAt

	scope := sorted[0].ScopePath
	from := first.Format("2006-01-02")
	to := last.Format("2006-01-02")

	// Frequency-ranked unique tokens (stopword-lite: keep it cheap).
	freq := map[string]int{}
	for _, m := range sorted {
		for _, w := range strings.Fields(strings.ToLower(m.Content)) {
			w = strings.Trim(w, `.,;:!?"'()[]`)
			if w == "" || isStopword(w) {
				continue
			}
			freq[w]++
		}
	}
	type kv struct {
		w string
		n int
	}
	toks := make([]kv, 0, len(freq))
	for w, n := range freq {
		toks = append(toks, kv{w, n})
	}
	sort.Slice(toks, func(i, j int) bool {
		if toks[i].n != toks[j].n {
			return toks[i].n > toks[j].n
		}
		return toks[i].w < toks[j].w
	})
	topWords := make([]string, 0, 5)
	for i := 0; i < len(toks) && i < 5; i++ {
		topWords = append(topWords, toks[i].w)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Summary of %d memories (scope=%s, %s..%s).", len(sorted), scope, from, to)
	if len(topWords) > 0 {
		fmt.Fprintf(&b, " Topics: %s.", strings.Join(topWords, ", "))
	}
	b.WriteString(" ")

	// Echo the top-N sentences from the originals, in chronological order.
	echoed := 0
	for _, m := range sorted {
		if echoed >= h.MaxSentences {
			break
		}
		sent := firstSentence(m.Content)
		if sent == "" {
			continue
		}
		if !strings.HasSuffix(sent, ".") {
			sent += "."
		}
		b.WriteString(sent + " ")
		echoed++
	}
	return strings.TrimSpace(b.String()), nil
}

// firstSentence returns the first sentence (up to the first '.' or the whole
// trimmed content if no terminator is found).
func firstSentence(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.IndexAny(s, ".\n"); i >= 0 {
		return strings.TrimSpace(s[:i+1])
	}
	return s
}

var stopwords = map[string]bool{
	"a": true, "an": true, "the": true, "and": true, "or": true, "of": true,
	"to": true, "in": true, "for": true, "on": true, "with": true, "is": true,
	"are": true, "was": true, "were": true, "be": true, "we": true, "it": true,
	"this": true, "that": true, "from": true, "by": true, "at": true, "as": true,
}

func isStopword(w string) bool { return stopwords[w] }

// ---------------------------------------------------------------------------
// LLMSummarizer
// ---------------------------------------------------------------------------

// LLMSummarizer is an optional pluggable summarizer hook. It POSTs the input
// texts to a local/remote endpoint. No API is implemented here (v1 relies on
// the heuristic); it exists so the Summarizer interface has a second,
// future-proof implementation.
type LLMSummarizer struct {
	Endpoint string
	APIKey   string
}

// Summarize returns an error since an LLM endpoint is not configured for v1.
// Implementers may override this by providing their own Summarizer.
func (l *LLMSummarizer) Summarize(ctx context.Context, memories []store.Memory) (string, error) {
	return "", fmt.Errorf("compact: LLMSummarizer is a hook; configure a custom Summarizer instead")
}

// Ensure interface conformance.
var _ Summarizer = (*HeuristicSummarizer)(nil)
var _ Summarizer = (*LLMSummarizer)(nil)
