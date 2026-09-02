package capture

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// CaptureSummary represents the per-session capture report output.
type CaptureSummary struct {
	OK                   bool          `json:"ok"`
	SessionID            string        `json:"session_id"`
	Harness              string        `json:"harness"`
	Backend              string        `json:"backend,omitempty"`
	StartedAt            time.Time     `json:"started_at"`
	EndedAt              time.Time     `json:"ended_at"`
	TotalMessages        int           `json:"total_messages"`
	Captured             int           `json:"captured"`
	SkippedDuplicate     int           `json:"skipped_duplicate"`
	SkippedLowConfidence int           `json:"skipped_low_confidence"`
	Items                []CaptureItem `json:"items"`
}

// SummaryTracker manages in-flight statistics and item recording for a capture session.
type SummaryTracker struct {
	mu      sync.Mutex
	summary CaptureSummary
}

// NewSummaryTracker creates a new SummaryTracker initialized with the given session ID and harness.
func NewSummaryTracker(sessionID, harness, backend string) *SummaryTracker {
	return &SummaryTracker{
		summary: CaptureSummary{
			OK:        true,
			SessionID: sessionID,
			Harness:   harness,
			Backend:   backend,
			StartedAt: time.Now(),
			Items:     make([]CaptureItem, 0),
		},
	}
}

// SetTotalMessages sets the total count of messages processed from the transcript.
func (st *SummaryTracker) SetTotalMessages(n int) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.summary.TotalMessages = n
}

// AddTotalMessages increments the total count of messages processed from the transcript.
func (st *SummaryTracker) AddTotalMessages(n int) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.summary.TotalMessages += n
}

// RecordCaptured increments the captured counter and appends the item to the summary list.
func (st *SummaryTracker) RecordCaptured(item CaptureItem) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.summary.Captured++
	st.summary.Items = append(st.summary.Items, item)
}

// RecordSkippedDuplicate increments the duplicate skip counter.
func (st *SummaryTracker) RecordSkippedDuplicate(item CaptureItem) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.summary.SkippedDuplicate++
}

// RecordSkippedLowConfidence increments the low confidence skip counter.
func (st *SummaryTracker) RecordSkippedLowConfidence(item CaptureItem) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.summary.SkippedLowConfidence++
}

// Finalize marks the end time and returns the completed CaptureSummary snapshot.
func (st *SummaryTracker) Finalize() CaptureSummary {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.summary.EndedAt.IsZero() {
		st.summary.EndedAt = time.Now()
	}
	return st.summary
}

// SaveSummary persists the finalized summary JSON into <homeDir>/capture-summary-<sessionID>.json.
func (st *SummaryTracker) SaveSummary(homeDir string) error {
	summary := st.Finalize()
	return SaveSummary(homeDir, summary)
}

// SaveSummary writes a CaptureSummary to disk.
func SaveSummary(homeDir string, summary CaptureSummary) error {
	if homeDir == "" {
		homeDir = "."
	}
	filePath := filepath.Join(homeDir, fmt.Sprintf("capture-summary-%s.json", summary.SessionID))

	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("summary: marshal: %w", err)
	}

	if err := os.MkdirAll(homeDir, 0700); err != nil {
		return fmt.Errorf("summary: mkdir: %w", err)
	}

	return os.WriteFile(filePath, data, 0600)
}

// LoadSummary reads a CaptureSummary for a given session ID from disk.
func LoadSummary(homeDir, sessionID string) (*CaptureSummary, error) {
	if homeDir == "" {
		homeDir = "."
	}
	filePath := filepath.Join(homeDir, fmt.Sprintf("capture-summary-%s.json", sessionID))

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("summary: read file: %w", err)
	}

	var summary CaptureSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		return nil, fmt.Errorf("summary: unmarshal: %w", err)
	}

	return &summary, nil
}

// FindLatestSummary finds and loads the most recently created capture summary in homeDir.
func FindLatestSummary(homeDir string) (*CaptureSummary, error) {
	if homeDir == "" {
		homeDir = "."
	}
	pattern := filepath.Join(homeDir, "capture-summary-*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("summary: no capture summaries found")
	}

	// Sort by file modification time descending
	type fileInfo struct {
		path    string
		modTime time.Time
	}
	var files []fileInfo
	for _, m := range matches {
		if fi, err := os.Stat(m); err == nil {
			files = append(files, fileInfo{path: m, modTime: fi.ModTime()})
		}
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.After(files[j].modTime)
	})

	latestPath := files[0].path
	data, err := os.ReadFile(latestPath)
	if err != nil {
		return nil, err
	}

	var summary CaptureSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		return nil, err
	}

	return &summary, nil
}
