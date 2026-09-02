package capture

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/aradenta-labs/cent-mem/internal/store"
)

// ItemHash computes a deterministic SHA256 hex string for a capture item.
func ItemHash(item CaptureItem) string {
	h := sha256.New()
	h.Write([]byte(strings.ToLower(strings.TrimSpace(item.Category))))
	h.Write([]byte(":"))
	h.Write([]byte(strings.ToLower(strings.TrimSpace(item.Key))))
	h.Write([]byte(":"))
	h.Write([]byte(strings.TrimSpace(item.Content)))
	return hex.EncodeToString(h.Sum(nil))
}

// SessionDedup tracks captured items within a single agent session to avoid duplicates.
type SessionDedup struct {
	filePath string
	mu       sync.RWMutex
	hashes   map[string]bool
}

// sessionDedupData is the JSON structure saved on disk.
type sessionDedupData struct {
	SessionID string   `json:"session_id"`
	Hashes    []string `json:"hashes"`
}

// NewSessionDedup creates or restores a SessionDedup tracker for the given session ID.
func NewSessionDedup(homeDir, sessionID string) (*SessionDedup, error) {
	if homeDir == "" {
		homeDir = "."
	}
	filePath := filepath.Join(homeDir, fmt.Sprintf("session-%s.dedup.json", sessionID))

	sd := &SessionDedup{
		filePath: filePath,
		hashes:   make(map[string]bool),
	}

	if data, err := os.ReadFile(filePath); err == nil {
		var d sessionDedupData
		if err := json.Unmarshal(data, &d); err == nil {
			for _, h := range d.Hashes {
				sd.hashes[h] = true
			}
		}
	}

	return sd, nil
}

// IsSessionDuplicate returns true if the item has already been saved during this session.
func (s *SessionDedup) IsSessionDuplicate(item CaptureItem) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hashes[ItemHash(item)]
}

// MarkSaved records that an item has been saved and syncs the hash to disk.
func (s *SessionDedup) MarkSaved(item CaptureItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	hash := ItemHash(item)
	s.hashes[hash] = true

	var hashList []string
	for h := range s.hashes {
		hashList = append(hashList, h)
	}

	data := sessionDedupData{
		Hashes: hashList,
	}

	bytesData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("dedup: marshal session dedup data: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(s.filePath), 0700); err != nil {
		return fmt.Errorf("dedup: create dir: %w", err)
	}

	return os.WriteFile(s.filePath, bytesData, 0600)
}

// Cleanup removes the session dedup file from disk.
func (s *SessionDedup) Cleanup() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(s.filePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// IsStoreDuplicate checks whether an identical memory already exists in the persistent store.
func IsStoreDuplicate(ctx context.Context, item CaptureItem, scopePath string, st *store.Store) (bool, error) {
	if st == nil {
		return false, nil
	}

	cat := strings.ToLower(strings.TrimSpace(item.Category))
	if cat == "fact" || cat == "dependency" {
		key := item.Key
		if cat == "dependency" && !strings.HasPrefix(key, "dep.") {
			key = "dep." + key
		}
		if key != "" {
			existingFact, err := st.GetFact(ctx, scopePath, key, false)
			if err == nil && existingFact != nil {
				// Same key and same value -> duplicate
				if strings.TrimSpace(existingFact.Value) == strings.TrimSpace(item.Content) {
					return true, nil
				}
			}
		}
	}

	// For notes/logs, check if any active memory in scope has identical content
	mems, err := st.List(ctx, store.ListQuery{
		ScopePath: scopePath,
		Status:    "active",
		Limit:     500,
	})
	if err != nil {
		return false, err
	}

	normContent := strings.TrimSpace(item.Content)
	for _, m := range mems {
		if strings.TrimSpace(m.Content) == normContent {
			return true, nil
		}
	}

	return false, nil
}
