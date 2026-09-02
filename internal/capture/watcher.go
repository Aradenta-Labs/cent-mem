package capture

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// WatcherConfig configures the incremental transcript watcher.
type WatcherConfig struct {
	TranscriptPath      string
	Harness             string
	PollInterval        time.Duration
	DebounceDuration    time.Duration
	StateFilePath       string
	Categories          []string
	ConfidenceThreshold float64
	Home                string
}

// WatcherState records file parsing progress to avoid re-reading past content.
type WatcherState struct {
	FilePath       string    `json:"file_path"`
	LastByteOffset int64     `json:"last_byte_offset"`
	LastLineNumber int       `json:"last_line_number"`
	LastModified   time.Time `json:"last_modified"`
	SessionID      string    `json:"session_id,omitempty"`
}

// Watcher monitors a transcript file in real time and streams new messages.
type Watcher struct {
	cfg          WatcherConfig
	state        WatcherState
	processChunk func(msgs []TranscriptMessage) error
	mu           sync.Mutex
}

// NewWatcher initializes a Watcher instance.
func NewWatcher(cfg WatcherConfig, processChunk func(msgs []TranscriptMessage) error) (*Watcher, error) {
	if cfg.TranscriptPath == "" {
		return nil, fmt.Errorf("watcher: missing transcript path")
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 250 * time.Millisecond
	}
	if cfg.DebounceDuration <= 0 {
		cfg.DebounceDuration = 100 * time.Millisecond
	}
	if cfg.StateFilePath == "" && cfg.Home != "" {
		hash := sha256.Sum256([]byte(cfg.TranscriptPath))
		cfg.StateFilePath = filepath.Join(cfg.Home, fmt.Sprintf("watcher-%s.json", hex.EncodeToString(hash[:8])))
	}

	w := &Watcher{
		cfg:          cfg,
		processChunk: processChunk,
	}

	if err := w.loadState(); err != nil {
		// Non-fatal, start from fresh state
		w.state = WatcherState{
			FilePath: cfg.TranscriptPath,
		}
	}

	return w, nil
}

// GetState returns a copy of the current watcher state.
func (w *Watcher) GetState() WatcherState {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.state
}

// SetSessionID assigns an active session ID to the watcher state.
func (w *Watcher) SetSessionID(sessionID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.state.SessionID = sessionID
}

func (w *Watcher) loadState() error {
	if w.cfg.StateFilePath == "" {
		return nil
	}
	data, err := os.ReadFile(w.cfg.StateFilePath)
	if err != nil {
		return err
	}
	var s WatcherState
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s.FilePath == w.cfg.TranscriptPath {
		w.state = s
	}
	return nil
}

// SaveState persists the watcher state to disk.
func (w *Watcher) SaveState() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.cfg.StateFilePath == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(w.cfg.StateFilePath), 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(w.state, "", "  ")
	if err != nil {
		return err
	}

	tmpFile := w.cfg.StateFilePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpFile, w.cfg.StateFilePath)
}

// PollOnce inspects the file for new content and processes any appended messages.
func (w *Watcher) PollOnce() (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	info, err := os.Stat(w.cfg.TranscriptPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("watcher stat: %w", err)
	}

	size := info.Size()
	// Detect file truncation or rotation
	if size < w.state.LastByteOffset {
		w.state.LastByteOffset = 0
		w.state.LastLineNumber = 0
	}

	if size == w.state.LastByteOffset {
		return 0, nil
	}

	f, err := os.Open(w.cfg.TranscriptPath)
	if err != nil {
		return 0, fmt.Errorf("watcher open: %w", err)
	}
	defer f.Close()

	if w.state.LastByteOffset > 0 {
		if _, err := f.Seek(w.state.LastByteOffset, io.SeekStart); err != nil {
			return 0, fmt.Errorf("watcher seek: %w", err)
		}
	}

	var buf bytes.Buffer
	n, err := io.Copy(&buf, f)
	if err != nil {
		return 0, fmt.Errorf("watcher copy: %w", err)
	}

	if n == 0 {
		return 0, nil
	}

	msgs, err := ConvertTranscript(w.cfg.Harness, &buf)
	if err != nil {
		return 0, fmt.Errorf("watcher convert: %w", err)
	}

	w.state.LastByteOffset += n
	w.state.LastModified = info.ModTime()

	if len(msgs) > 0 && w.processChunk != nil {
		if err := w.processChunk(msgs); err != nil {
			return len(msgs), fmt.Errorf("watcher process chunk: %w", err)
		}
	}

	_ = w.saveStateLocked()
	return len(msgs), nil
}

func (w *Watcher) saveStateLocked() error {
	if w.cfg.StateFilePath == "" {
		return nil
	}
	_ = os.MkdirAll(filepath.Dir(w.cfg.StateFilePath), 0700)
	data, err := json.MarshalIndent(w.state, "", "  ")
	if err != nil {
		return err
	}
	tmpFile := w.cfg.StateFilePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpFile, w.cfg.StateFilePath)
}

// Watch runs the polling loop until the context is canceled.
func (w *Watcher) Watch(ctx context.Context) error {
	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	// Initial poll
	_, _ = w.PollOnce()

	for {
		select {
		case <-ctx.Done():
			_ = w.SaveState()
			return ctx.Err()
		case <-ticker.C:
			if _, err := w.PollOnce(); err != nil {
				// Log error or continue
			}
		}
	}
}
