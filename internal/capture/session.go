package capture

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Session coordinates capture session state, deduplication, summary tracking, and single-instance locking.
type Session struct {
	ID       string
	Harness  string
	HomeDir  string
	LockPath string
	Dedup    *SessionDedup
	Tracker  *SummaryTracker
	Cfg      CaptureConfig
}

// GenerateSessionID creates a unique session identifier based on timestamp and cryptographic random hex.
func GenerateSessionID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%s", time.Now().Unix(), hex.EncodeToString(b))
}

// StartSession initiates a capture session, acquires the session lock, and sets up dedup/summary tracking.
func StartSession(homeDir, harness string, cfg CaptureConfig) (*Session, error) {
	if homeDir == "" {
		homeDir = "."
	}
	if harness == "" {
		harness = cfg.Harness
	}
	if harness == "" {
		harness = "antigravity"
	}

	if err := os.MkdirAll(homeDir, 0700); err != nil {
		return nil, fmt.Errorf("session: create home dir: %w", err)
	}

	lockPath := filepath.Join(homeDir, "capture-session.lock")
	sessionID := GenerateSessionID()

	// Check existing lock
	if lockData, err := os.ReadFile(lockPath); err == nil {
		lockContent := strings.TrimSpace(string(lockData))
		// Format in lock: <pid>:<session_id>:<timestamp>
		parts := strings.Split(lockContent, ":")
		if len(parts) >= 3 {
			lockTimestamp, _ := strconv.ParseInt(parts[2], 10, 64)
			// If lock is older than 5 minutes, consider it stale and allow stealing
			if time.Now().Unix()-lockTimestamp < 300 {
				return nil, fmt.Errorf("session: another capture session is active (lock: %s)", lockContent)
			}
		}
	}

	// Write lock file
	lockPayload := fmt.Sprintf("%d:%s:%d", os.Getpid(), sessionID, time.Now().Unix())
	if err := os.WriteFile(lockPath, []byte(lockPayload), 0600); err != nil {
		return nil, fmt.Errorf("session: write lock: %w", err)
	}

	dedup, err := NewSessionDedup(homeDir, sessionID)
	if err != nil {
		_ = os.Remove(lockPath)
		return nil, fmt.Errorf("session: init dedup: %w", err)
	}

	tracker := NewSummaryTracker(sessionID, harness, cfg.Backend)

	return &Session{
		ID:       sessionID,
		Harness:  harness,
		HomeDir:  homeDir,
		LockPath: lockPath,
		Dedup:    dedup,
		Tracker:  tracker,
		Cfg:      cfg,
	}, nil
}

// EndSession finalizes the session summary, cleans up session deduplication records, and removes the session lock.
func EndSession(s *Session) (*CaptureSummary, error) {
	if s == nil {
		return nil, nil
	}

	summary := s.Tracker.Finalize()
	if err := s.Tracker.SaveSummary(s.HomeDir); err != nil {
		return &summary, fmt.Errorf("session: save summary: %w", err)
	}

	if s.Dedup != nil {
		_ = s.Dedup.Cleanup()
	}

	if s.LockPath != "" {
		_ = os.Remove(s.LockPath)
	}

	return &summary, nil
}
