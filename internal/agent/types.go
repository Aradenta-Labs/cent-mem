package agent

import "github.com/aradenta-labs/cent-mem/internal/store"

// AskResult contains the answer, citations, identified gaps, and telemetry for an inquiry.
type AskResult struct {
	Answer         string           `json:"answer"`
	Citations      []store.Citation `json:"citations"`
	KnowledgeGaps  []string         `json:"knowledge_gaps"`
	ReasoningSteps int              `json:"reasoning_steps"`
	ConversationID string           `json:"conversation_id,omitempty"`
	FallbackUsed   bool             `json:"fallback_used,omitempty"`
}

// CurateResult summarizes autonomous memory curation operations.
type CurateResult struct {
	ProposalsCreated    []int64 `json:"proposals_created"`
	ProposalsApplied    []int64 `json:"proposals_applied"`
	ScannedMemories     int     `json:"scanned_memories"`
	ContradictionsFound int     `json:"contradictions_found"`
	DuplicatesFound     int     `json:"duplicates_found"`
	FallbackUsed        bool    `json:"fallback_used,omitempty"`
}

// SummarizeResult holds a synthesized scope briefing.
type SummarizeResult struct {
	Title           string  `json:"title"`
	SummaryMarkdown string  `json:"summary_markdown"`
	CitedMemoryIDs  []int64 `json:"cited_memory_ids"`
	Scope           string  `json:"scope"`
	SavedID         *int64  `json:"saved_id,omitempty"`
	FallbackUsed    bool    `json:"fallback_used,omitempty"`
}

// InquiryOptions parameterizes an Ask inquiry.
type InquiryOptions struct {
	Scope          string
	Top            int
	ConversationID string
	StreamCallback func(chunk *StreamChunk) error
}

// CurateOptions parameterizes a Curate run.
type CurateOptions struct {
	Scope     string
	Type      string // "contradictions" | "dedup" | "all"
	AutoApply bool
	DryRun    bool
}

// SummarizeOptions parameterizes a Summarize run.
type SummarizeOptions struct {
	Scope  string
	Focus  string
	Save   bool
	Format string // "markdown" | "json"
}
