package search

import (
	"context"
	"strings"
	"unicode"

	"github.com/aradenta-labs/cent-mem/internal/store"
)

// SuggestedLinkCandidate represents a relationship link auto-suggested during memory creation.
type SuggestedLinkCandidate struct {
	ID            int64  `json:"id"`
	FromID        int64  `json:"from_id"`
	ToID          int64  `json:"to_id"`
	Relation      string `json:"relation"`
	TargetContent string `json:"target_content,omitempty"`
	TargetType    string `json:"target_type,omitempty"`
}

var contradictionCues = []string{
	"no longer",
	"deprecated",
	"replaced by",
	"switched from",
	"instead of",
	"obsolete",
	"discontinued",
}

var dependencyCues = []string{
	"depends on",
	"requires",
	"prerequisite",
	"built on top of",
}

var commonStopWords = map[string]bool{
	"about": true, "after": true, "again": true, "against": true, "being": true,
	"between": true, "could": true, "during": true, "first": true, "further": true,
	"having": true, "other": true, "should": true, "their": true, "there": true,
	"these": true, "those": true, "through": true, "under": true, "until": true,
	"which": true, "while": true, "would": true, "where": true, "with": true,
	"from": true, "that": true, "this": true, "have": true, "will": true,
	"all": true, "and": true, "any": true, "are": true, "but": true, "can": true,
	"did": true, "for": true, "get": true, "had": true, "has": true, "her": true,
	"him": true, "his": true, "how": true, "its": true, "let": true, "may": true,
	"nor": true, "not": true, "now": true, "off": true, "old": true, "one": true,
	"our": true, "out": true, "per": true, "say": true, "see": true, "she": true,
	"the": true, "too": true, "top": true, "try": true, "use": true, "via": true,
	"was": true, "way": true, "who": true, "why": true, "yes": true, "yet": true,
	"you": true,
}

// SuggestLinks analyzes a freshly written memory against existing memories in the same
// or ancestor scopes, inferring semantic relationships (supports, refines, contradicts,
// depends-on, supersedes) and saving them as pending suggestions (suggested = 1).
// Fails open silently if candidate search or embedder fails, ensuring writes never abort.
func (s *Searcher) SuggestLinks(ctx context.Context, newMem store.Memory) ([]SuggestedLinkCandidate, error) {
	if s == nil || s.store == nil || strings.TrimSpace(newMem.Content) == "" {
		return nil, nil
	}

	// 1. Candidate Retrieval: evaluate candidates across target scope + ancestor scopes
	q := Query{
		Text:    newMem.Content,
		Scope:   newMem.ScopePath,
		Inherit: true,
		Top:     5,
	}

	candidates, err := s.Recall(ctx, q)
	if err != nil {
		// Fail open silently without failing writes
		return nil, nil
	}

	// 2. Fetch existing links to avoid proposing already existing relationships
	existingLinks := make(map[int64]bool)
	if outLinks, inLinks, err := s.store.GetLinksForMemory(ctx, newMem.ID, true); err == nil {
		for _, l := range outLinks {
			existingLinks[l.ToID] = true
		}
		for _, l := range inLinks {
			existingLinks[l.FromID] = true
		}
	}

	var suggestions []SuggestedLinkCandidate

	for _, cand := range candidates {
		// Filter out self and existing links
		if cand.ID == newMem.ID || existingLinks[cand.ID] {
			continue
		}

		// Require minimum score threshold
		if cand.Score < 0.015 {
			continue
		}

		relation := ClassifyRelation(newMem.Content, newMem.Tags, cand.Content, cand.Tags, cand.SemanticScore, cand.Score)
		if relation == "" {
			continue
		}

		link, err := s.store.CreateLink(ctx, newMem.ID, cand.ID, relation, true)
		if err != nil {
			continue
		}

		suggestions = append(suggestions, SuggestedLinkCandidate{
			ID:            link.ID,
			FromID:        newMem.ID,
			ToID:          cand.ID,
			Relation:      relation,
			TargetContent: cand.Content,
			TargetType:    cand.Type,
		})
	}

	return suggestions, nil
}

// ClassifyRelation determines the semantic relation from source content to target candidate.
func ClassifyRelation(srcContent string, srcTags []string, targetContent string, targetTags []string, semanticScore, rrfScore float64) string {
	srcLower := strings.ToLower(srcContent)

	// Rule 1: Contradiction / Supersession cues
	for _, cue := range contradictionCues {
		if strings.Contains(srcLower, cue) {
			// If target shares tags or significant keywords -> supersedes; otherwise -> contradicts
			if tagOverlapCount(srcTags, targetTags) > 0 || hasKeywordOverlap(srcContent, targetContent) {
				return "supersedes"
			}
			return "contradicts"
		}
	}

	// Rule 2: Dependency cues
	for _, cue := range dependencyCues {
		if strings.Contains(srcLower, cue) {
			return "depends-on"
		}
	}

	// Rule 3: Refinement (strong tag overlap >= 2 and high similarity)
	if tagOverlapCount(srcTags, targetTags) >= 2 && (semanticScore >= 0.75 || rrfScore >= 0.015) {
		return "refines"
	}

	// Rule 4: Support (high semantic or RRF similarity default)
	if semanticScore >= 0.65 || rrfScore >= 0.015 {
		return "supports"
	}

	return ""
}

func tagOverlapCount(a, b []string) int {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	set := make(map[string]bool, len(a))
	for _, t := range a {
		cleaned := strings.ToLower(strings.TrimSpace(t))
		if cleaned != "" {
			set[cleaned] = true
		}
	}
	count := 0
	for _, t := range b {
		cleaned := strings.ToLower(strings.TrimSpace(t))
		if cleaned != "" && set[cleaned] {
			count++
		}
	}
	return count
}

func hasKeywordOverlap(a, b string) bool {
	wordsA := extractKeywords(a)
	wordsB := extractKeywords(b)
	for w := range wordsA {
		if wordsB[w] {
			return true
		}
	}
	return false
}

func extractKeywords(text string) map[string]bool {
	out := make(map[string]bool)
	f := func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsNumber(c)
	}
	for _, token := range strings.FieldsFunc(strings.ToLower(text), f) {
		if len(token) >= 3 && !commonStopWords[token] {
			out[token] = true
		}
	}
	return out
}
