package store

import "time"

// Memory is a single memory row in the `memories` table. All memory types
// (note/log/fact) share this struct.
type Memory struct {
	ID            int64
	ScopeID       int64
	ScopePath     string
	Type          string // note | log | fact
	Content       string
	Key           string
	ValueJSON     string
	Tags          []string
	SourceAgent   string
	SourceSession string
	ContentHash   string
	Status         string // active | archived | summarized
	SummarizeAt    *int64
	AccessCount    int
	LastAccessedAt *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Fact is a key/value structured memory (type='fact').
type Fact struct {
	ID        int64
	ScopeID   int64
	ScopePath string
	Key       string
	Value     string // the raw JSON value
	Content   string // derived display content "key = value"
	Tags      []string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Event is an append-only audit/sync log entry.
type Event struct {
	ID        int64
	MemoryID  int64
	Op        string
	ScopePath string
	Payload   string
	CreatedAt time.Time
}

// ListQuery parameterizes List().
type ListQuery struct {
	ScopePath     string
	ScopeIDs      []int64
	Type          string
	Tags          []string
	Status        string // default "active"
	SourceAgent   string
	SourceSession string
	Since         time.Time
	Until         time.Time
	Limit         int
	Offset        int
}

// ImportanceDistribution captures access-frequency distribution across active memories.
type ImportanceDistribution struct {
	ZeroAccess       int64   `json:"zero_access"`
	LowAccess1to5    int64   `json:"low_access_1_5"`
	MedAccess6to20   int64   `json:"medium_access_6_20"`
	HighAccess21Plus int64   `json:"high_access_21_plus"`
	MaxAccessCount   int64   `json:"max_access_count"`
	AvgAccessCount   float64 `json:"avg_access_count"`
}

// Stats aggregates store-wide counts.
type Stats struct {
	DBPath                 string
	DBSizeMB               float64
	Memories               int64
	ByType                 map[string]int64
	ByScope                map[string]int64
	LastCompactAt          *int64
	PendingEmbedding       int64
	ImportanceDistribution ImportanceDistribution
}

// MemoryInput is the write input for PutMemory.
type MemoryInput struct {
	Scope         string // scope path
	Type          string // note | log
	Content       string
	Key           string
	ValueJSON     string
	Tags          []string
	SourceAgent   string
	SourceSession string
	SummarizeAt   *int64
}

// FactInput is the write input for SetFact.
type FactInput struct {
	Scope       string
	Key         string
	Value       string
	Tags        []string
	SourceAgent string
}

// ScopeNode represents a node in the hierarchical scope tree with direct
// and recursive memory counts.
type ScopeNode struct {
	ID         int64        `json:"id"`
	Path       string       `json:"path"`
	ParentPath string       `json:"parent_path,omitempty"`
	Kind       string       `json:"kind"`
	Name       string       `json:"name"`
	Count      int64        `json:"count"`       // direct active memories
	TotalCount int64        `json:"total_count"` // direct + descendant active memories
	Children   []*ScopeNode `json:"children"`
}

// ValidLinkRelations defines the allowed values for relation in memory_links.
var ValidLinkRelations = []string{"supports", "refines", "contradicts", "depends-on", "supersedes"}

// IsValidLinkRelation returns true if rel is one of the 5 canonical relations.
func IsValidLinkRelation(rel string) bool {
	for _, r := range ValidLinkRelations {
		if r == rel {
			return true
		}
	}
	return false
}

// Link represents a directional relationship between two memories.
type Link struct {
	ID        int64     `json:"id"`
	FromID    int64     `json:"from_id"`
	ToID      int64     `json:"to_id"`
	Relation  string    `json:"relation"`
	Suggested bool      `json:"suggested"`
	CreatedAt time.Time `json:"created_at"`
}

// LinkWithContent embeds Link along with metadata of the linked memories.
type LinkWithContent struct {
	Link
	SourceContent string `json:"source_content,omitempty"`
	TargetContent string `json:"target_content,omitempty"`
	SourceType    string `json:"source_type,omitempty"`
	TargetType    string `json:"target_type,omitempty"`
	SourceScope   string `json:"source_scope,omitempty"`
	TargetScope   string `json:"target_scope,omitempty"`
}

// MemoryLinksResult holds both outgoing and incoming links for a memory.
type MemoryLinksResult struct {
	MemoryID int64             `json:"memory_id"`
	Outgoing []LinkWithContent `json:"outgoing"`
	Incoming []LinkWithContent `json:"incoming"`
}

