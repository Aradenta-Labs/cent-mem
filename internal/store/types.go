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
	Status        string // active | archived | summarized
	SummarizeAt   *int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
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
	ScopePath string
	ScopeIDs  []int64
	Type      string
	Tags      []string
	Status    string // default "active"
	Limit     int
	Offset    int
}

// Stats aggregates store-wide counts.
type Stats struct {
	DBPath           string
	DBSizeMB         float64
	Memories         int64
	ByType           map[string]int64
	ByScope          map[string]int64
	LastCompactAt    *int64
	PendingEmbedding int64
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
