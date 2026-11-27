package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/hooks"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

func TestWebhookCLI_CurateApplyDismiss_AndDisabledNoOp(t *testing.T) {
	stubDownloader()
	home := newHome(t)
	runCLI(t, home, "init")

	// Force offline curation so dedup creates deterministic merge proposals without LLM
	runCLI(t, home, "config", "set", "llm.backend", "disabled")

	type receivedEvent struct {
		Event   string
		Sig     string
		RawBody []byte
		Payload hooks.Payload
	}
	var mu sync.Mutex
	var events []receivedEvent

	hookSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var p hooks.Payload
		_ = json.Unmarshal(body, &p)
		mu.Lock()
		events = append(events, receivedEvent{
			Event:   r.Header.Get("X-Centmem-Event"),
			Sig:     r.Header.Get("X-Centmem-Signature-256"),
			RawBody: body,
			Payload: p,
		})
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer hookSrv.Close()

	secret := "cli-hook-secret-abc"
	for _, kv := range [][2]string{
		{"hooks.enabled", "true"},
		{"hooks.proposals_created", hookSrv.URL},
		{"hooks.proposals_applied", hookSrv.URL},
		{"hooks.proposals_dismissed", hookSrv.URL},
		{"hooks.secret", secret},
		{"hooks.timeout_seconds", "5"},
	} {
		_, stderr, code := runCLI(t, home, "config", "set", kv[0], kv[1])
		if code != 0 {
			t.Fatalf("config set %s failed (code %d): %s", kv[0], code, stderr)
		}
	}

	// Seed two duplicate memories so offline curate creates a merge proposal
	s, err := store.Open(config.Config{DBPath: filepath.Join(home, "centmem.db")})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	m1, _, err := s.PutMemory(context.Background(), store.MemoryInput{
		Scope:   "project:hooks",
		Type:    "note",
		Content: "Duplicate architecture note for webhook test",
	})
	if err != nil {
		t.Fatalf("put dup 1: %v", err)
	}
	_, err = s.DB().Exec(`UPDATE memories SET updated_at = ? WHERE id = ?`, time.Now().Add(-5*time.Minute).UnixMicro(), m1)
	if err != nil {
		t.Fatalf("age dup 1: %v", err)
	}
	_, _, err = s.PutMemory(context.Background(), store.MemoryInput{
		Scope:   "project:hooks",
		Type:    "note",
		Content: "Duplicate architecture note for webhook test",
	})
	if err != nil {
		t.Fatalf("put dup 2: %v", err)
	}
	_ = s.Close()

	// 1. Run `centmem curate` -> emits proposals.created and flushes before CLI exits
	stdout, stderr, code := runCLI(t, home, "curate", "--scope", "project:hooks", "--type", "dedup")
	if code != 0 {
		t.Fatalf("curate failed (code %d): %s", code, stderr)
	}
	var curateResp struct {
		OK               bool    `json:"ok"`
		ProposalsCreated []int64 `json:"proposals_created"`
	}
	if err := json.Unmarshal([]byte(stdout), &curateResp); err != nil {
		t.Fatalf("unmarshal curate: %v", err)
	}
	if len(curateResp.ProposalsCreated) != 1 {
		t.Fatalf("expected 1 created proposal, got %v", curateResp.ProposalsCreated)
	}
	propID := curateResp.ProposalsCreated[0]

	mu.Lock()
	if len(events) != 1 || events[0].Event != hooks.EventProposalsCreated {
		mu.Unlock()
		t.Fatalf("expected 1 proposals.created event, got %+v", events)
	}
	if want := hooks.SignPayload(secret, events[0].RawBody); events[0].Sig != want {
		mu.Unlock()
		t.Fatalf("HMAC mismatch on curate: got %q want %q", events[0].Sig, want)
	}
	if events[0].Payload.Count != 1 || events[0].Payload.Proposals[0].ID != propID {
		mu.Unlock()
		t.Fatalf("unexpected curate webhook payload: %+v", events[0].Payload)
	}
	mu.Unlock()

	// 2. Run `centmem proposals apply <id>` -> emits proposals.applied
	_, stderr, code = runCLI(t, home, "proposals", "apply", fmt.Sprintf("%d", propID))
	if code != 0 {
		t.Fatalf("proposals apply failed (code %d): %s", code, stderr)
	}
	mu.Lock()
	if len(events) != 2 || events[1].Event != hooks.EventProposalsApplied {
		mu.Unlock()
		t.Fatalf("expected proposals.applied event, got %+v", events)
	}
	mu.Unlock()

	// 3. Seed another proposal and run `centmem proposals dismiss <id>` -> emits proposals.dismissed
	s, err = store.Open(config.Config{DBPath: filepath.Join(home, "centmem.db")})
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	dismissID, err := s.CreateProposal(context.Background(), &store.Proposal{
		ScopePath:    "project:hooks",
		ProposalType: "link",
		Title:        "Dismiss target",
		Reasoning:    "test",
		PayloadJSON:  `{"from_id":1,"to_id":2,"relation":"supports"}`,
	})
	if err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}
	disabledID, err := s.CreateProposal(context.Background(), &store.Proposal{
		ScopePath:    "project:hooks",
		ProposalType: "link",
		Title:        "Disabled target",
		Reasoning:    "test",
		PayloadJSON:  `{"from_id":1,"to_id":2,"relation":"supports"}`,
	})
	if err != nil {
		t.Fatalf("CreateProposal disabled: %v", err)
	}
	_ = s.Close()

	_, stderr, code = runCLI(t, home, "proposals", "dismiss", fmt.Sprintf("%d", dismissID))
	if code != 0 {
		t.Fatalf("proposals dismiss failed (code %d): %s", code, stderr)
	}
	mu.Lock()
	if len(events) != 3 || events[2].Event != hooks.EventProposalsDismissed {
		mu.Unlock()
		t.Fatalf("expected proposals.dismissed event, got %+v", events)
	}
	mu.Unlock()

	// 4. Disable hooks -> zero additional webhook calls
	_, stderr, code = runCLI(t, home, "config", "set", "hooks.enabled", "false")
	if code != 0 {
		t.Fatalf("disable hooks failed: %s", stderr)
	}
	_, stderr, code = runCLI(t, home, "proposals", "dismiss", fmt.Sprintf("%d", disabledID))
	if code != 0 {
		t.Fatalf("dismiss when disabled failed: %s", stderr)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(events) != 3 {
		t.Fatalf("expected zero calls when hooks disabled, got total %d", len(events))
	}
}
