package export

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

// Envelope is the canonical export file format.
type Envelope struct {
	Format        string         `json:"format"`
	FormatVersion int            `json:"format_version"`
	ExportedAt    time.Time      `json:"exported_at"`
	Scope         string         `json:"scope"`
	Total         int            `json:"total"`
	Memories      []MemoryRecord `json:"memories"`
}

// MemoryRecord is a portable memory (no internal IDs except for traceability).
type MemoryRecord struct {
	ID            int64    `json:"id"`
	Scope         string   `json:"scope"`
	Type          string   `json:"type"`
	Content       string   `json:"content"`
	Key           string   `json:"key,omitempty"`
	ValueJSON     string   `json:"value_json,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	SourceAgent   string   `json:"source_agent,omitempty"`
	SourceSession string   `json:"source_session,omitempty"`
	CreatedAt     int64    `json:"created_at"`
}

// ImportReport is returned by Import and --dry-run.
type ImportReport struct {
	Total    int      `json:"total"`
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"` // duplicate content_hash already present
	Failed   int      `json:"failed"`
	Errors   []string `json:"errors,omitempty"`
}

var (
	// ErrUnsupportedFormat is returned when an imported file does not match centmem-export v1.
	ErrUnsupportedFormat = errors.New("import: unsupported file format (expected centmem-export v1)")
	// ErrEmptyMemories is returned when an export file contains zero memories.
	ErrEmptyMemories = errors.New("import: empty memories list")
)

// ValidateEnvelope checks that the envelope header matches centmem-export v1 and memories is non-empty.
func ValidateEnvelope(env Envelope) error {
	if env.Format != "centmem-export" || env.FormatVersion != 1 {
		return ErrUnsupportedFormat
	}
	if len(env.Memories) == 0 {
		return ErrEmptyMemories
	}
	return nil
}

// ReadEnvelope decodes and validates a canonical export envelope from reader.
func ReadEnvelope(r io.Reader) (Envelope, error) {
	var env Envelope
	dec := json.NewDecoder(r)
	if err := dec.Decode(&env); err != nil {
		return Envelope{}, fmt.Errorf("%w: %v", ErrUnsupportedFormat, err)
	}
	if err := ValidateEnvelope(env); err != nil {
		return Envelope{}, err
	}
	return env, nil
}

// WriteEnvelope serializes an envelope as indented JSON to writer.
func WriteEnvelope(w io.Writer, env Envelope) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(env)
}
