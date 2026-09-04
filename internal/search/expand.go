package search

import (
	"context"
	"database/sql"
	"slices"
	"strings"
	"unicode"
)

const (
	maxEnrichCandidates = 20
	maxEnrichTags       = 3
)

// EnrichQuery returns a copy of q with q.Text augmented by up to 3 most frequent
// tags from up to 20 FTS5 matching memories within the query's scope.
// Tags already appearing as words in q.Text are excluded.
// If q.Text is empty or expansion produces no new tags, q is returned unchanged.
func (s *Searcher) EnrichQuery(ctx context.Context, q Query) (Query, error) {
	text := strings.TrimSpace(q.Text)
	if text == "" {
		return q, nil
	}

	exact, prefix := ftsQueries(text)
	if exact == "" {
		return q, nil
	}

	scopeConds, scopeArgs, err := s.scopeFilter(ctx, q)
	if err != nil {
		return q, err
	}
	if scopeConds == " AND 1 = 0" {
		return q, nil
	}

	sqlText := `
		SELECT m.tags, bm25(memories_fts) AS score
		FROM memories_fts
		JOIN memories m ON m.id = memories_fts.rowid
		WHERE memories_fts MATCH ? AND m.status = 'active'` +
		scopeConds +
		` ORDER BY score LIMIT 20`

	fetchTags := func(ftsQuery string) ([]string, error) {
		args := []any{ftsQuery}
		args = append(args, scopeArgs...)
		rows, err := s.db().QueryContext(ctx, sqlText, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var tagStrings []string
		for rows.Next() {
			var rawTags sql.NullString
			var score float64
			if err := rows.Scan(&rawTags, &score); err != nil {
				return nil, err
			}
			if rawTags.Valid && rawTags.String != "" {
				tagStrings = append(tagStrings, rawTags.String)
			}
		}
		return tagStrings, rows.Err()
	}

	tagStrings, err := fetchTags(exact)
	if err != nil {
		return q, nil
	}
	if len(tagStrings) == 0 && prefix != "" && prefix != exact {
		tagStrings, err = fetchTags(prefix)
		if err != nil {
			return q, nil
		}
	}

	if len(tagStrings) == 0 {
		return q, nil
	}

	existingWords := extractQueryWords(text)

	counts := make(map[string]int)
	for _, raw := range tagStrings {
		seenInRow := make(map[string]bool)
		for _, tag := range splitTags(raw) {
			tag = strings.ToLower(strings.TrimSpace(tag))
			if tag == "" || seenInRow[tag] || isTagInQuery(tag, existingWords) {
				continue
			}
			seenInRow[tag] = true
			counts[tag]++
		}
	}

	if len(counts) == 0 {
		return q, nil
	}

	type tagFreq struct {
		tag   string
		count int
	}

	candidates := make([]tagFreq, 0, len(counts))
	for t, c := range counts {
		candidates = append(candidates, tagFreq{tag: t, count: c})
	}

	slices.SortFunc(candidates, func(a, b tagFreq) int {
		if a.count != b.count {
			return b.count - a.count // higher frequency first
		}
		if a.tag < b.tag {
			return -1
		}
		if a.tag > b.tag {
			return 1
		}
		return 0
	})

	k := maxEnrichTags
	if len(candidates) < k {
		k = len(candidates)
	}

	selected := make([]string, k)
	for i := 0; i < k; i++ {
		selected[i] = candidates[i].tag
	}

	out := q
	out.Text = text + " " + strings.Join(selected, " ")
	return out, nil
}

// extractQueryWords extracts lowercase words and identifiers from query text.
// It normalizes punctuation and extracts individual words from compound terms.
func extractQueryWords(text string) map[string]bool {
	words := make(map[string]bool)
	for _, field := range strings.Fields(strings.ToLower(text)) {
		cleaned := strings.Trim(field, `"'()*,.:;!?[]{}<>`)
		if cleaned != "" {
			words[cleaned] = true
		}
		parts := strings.FieldsFunc(field, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		})
		for _, p := range parts {
			if p != "" {
				words[p] = true
			}
		}
	}
	return words
}

// isTagInQuery returns true if tag or all subparts of a compound tag appear in query words.
func isTagInQuery(tag string, words map[string]bool) bool {
	norm := strings.ToLower(strings.TrimSpace(tag))
	if norm == "" {
		return true
	}
	if words[norm] {
		return true
	}
	parts := strings.FieldsFunc(norm, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	if len(parts) > 1 {
		allPresent := true
		for _, p := range parts {
			if !words[p] {
				allPresent = false
				break
			}
		}
		if allPresent {
			return true
		}
	}
	return false
}
