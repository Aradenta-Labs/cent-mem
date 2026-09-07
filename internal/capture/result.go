package capture

// CapturedItem represents an individual memory created or evaluated during capture.
type CapturedItem struct {
	ID      int64    `json:"id,omitempty"`
	Type    string   `json:"type"`
	Tags    []string `json:"tags"`
	Content string   `json:"content"`
	DryRun  bool     `json:"dry_run"`
}

// CaptureResult represents the unified result output across git, docs, shell, and comments capture.
type CaptureResult struct {
	OK              bool           `json:"ok"`
	Command         string         `json:"command"`
	Source          string         `json:"source"`
	Scanned         int            `json:"scanned"`
	CommitsScanned  int            `json:"commits_scanned,omitempty"`
	MemoriesCreated int            `json:"memories_created"`
	MemoriesUpdated int            `json:"memories_updated"`
	Cursor          string         `json:"cursor,omitempty"`
	Items           []CapturedItem `json:"items"`
}
