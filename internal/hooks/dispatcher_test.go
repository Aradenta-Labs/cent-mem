package hooks

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/config"
)

func TestDispatcher_Emit_DeliveryAndHMAC(t *testing.T) {
	var mu sync.Mutex
	var gotMethod, gotContentType, gotUserAgent, gotEvent, gotSig string
	var gotBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		gotUserAgent = r.Header.Get("User-Agent")
		gotEvent = r.Header.Get("X-Centmem-Event")
		gotSig = r.Header.Get("X-Centmem-Signature-256")
		gotBody = body
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	secret := "whsec_test_secret_123"
	d := NewDispatcher(config.HooksConfig{
		Enabled:          true,
		ProposalsCreated: srv.URL,
		Secret:           secret,
		TimeoutSeconds:   5,
	})

	d.Emit(EventProposalsCreated, Payload{
		Scope: "project:cent-mem",
		Count: 1,
		Proposals: []ProposalSummary{
			{ID: 12, Type: "merge", Title: "Merge duplicate auth notes"},
		},
	})

	flushCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	d.Flush(flushCtx)

	mu.Lock()
	defer mu.Unlock()

	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
	if gotUserAgent != UserAgent {
		t.Errorf("User-Agent = %q, want %q", gotUserAgent, UserAgent)
	}
	if gotEvent != EventProposalsCreated {
		t.Errorf("X-Centmem-Event = %q, want %q", gotEvent, EventProposalsCreated)
	}
	wantSig := SignPayload(secret, gotBody)
	if gotSig != wantSig {
		t.Errorf("X-Centmem-Signature-256 = %q, want %q", gotSig, wantSig)
	}

	var decoded Payload
	if err := json.Unmarshal(gotBody, &decoded); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if decoded.Event != EventProposalsCreated || decoded.Scope != "project:cent-mem" || decoded.Count != 1 {
		t.Errorf("unexpected payload: %+v", decoded)
	}
	if decoded.Timestamp == "" {
		t.Errorf("expected non-empty RFC3339 timestamp")
	}
	if len(decoded.Proposals) != 1 || decoded.Proposals[0].ID != 12 || decoded.Proposals[0].Type != "merge" {
		t.Errorf("unexpected proposals: %+v", decoded.Proposals)
	}
}

func TestDispatcher_Emit_SecretFromEnvAndOmittedWhenEmpty(t *testing.T) {
	t.Setenv("CENTMEM_TEST_HOOK_SECRET", "env-resolved-secret")

	var mu sync.Mutex
	var sigs []string
	var bodies [][]byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		sigs = append(sigs, r.Header.Get("X-Centmem-Signature-256"))
		bodies = append(bodies, b)
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	// 1. Secret resolved from env var name
	dEnv := NewDispatcher(config.HooksConfig{
		Enabled:          true,
		ProposalsApplied: srv.URL,
		Secret:           "CENTMEM_TEST_HOOK_SECRET",
		TimeoutSeconds:   5,
	})
	dEnv.Emit(EventProposalsApplied, Payload{Scope: "global", Proposals: []ProposalSummary{{ID: 1, Type: "link", Title: "Link"}}})
	dEnv.Flush(context.Background())

	// 2. Empty secret -> no signature header
	dNoSecret := NewDispatcher(config.HooksConfig{
		Enabled:            true,
		ProposalsDismissed: srv.URL,
		Secret:             "",
		TimeoutSeconds:     5,
	})
	dNoSecret.Emit(EventProposalsDismissed, Payload{Scope: "global", Proposals: []ProposalSummary{{ID: 2, Type: "merge", Title: "Merge"}}})
	dNoSecret.Flush(context.Background())

	mu.Lock()
	defer mu.Unlock()
	if len(sigs) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(sigs))
	}
	if want := SignPayload("env-resolved-secret", bodies[0]); sigs[0] != want {
		t.Errorf("env secret sig = %q, want %q", sigs[0], want)
	}
	if sigs[1] != "" {
		t.Errorf("expected empty signature header when secret unset, got %q", sigs[1])
	}
}

func TestDispatcher_Emit_DisabledAndEmptyURLNoOp(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Disabled dispatcher must make zero network calls even with URL configured
	disabled := NewDispatcher(config.HooksConfig{
		Enabled:          false,
		ProposalsCreated: srv.URL,
		ProposalsApplied: srv.URL,
		TimeoutSeconds:   5,
	})
	disabled.Emit(EventProposalsCreated, Payload{Scope: "global", Proposals: []ProposalSummary{{ID: 1, Type: "merge", Title: "t"}}})
	disabled.Emit(EventProposalsApplied, Payload{Scope: "global", Proposals: []ProposalSummary{{ID: 1, Type: "merge", Title: "t"}}})
	disabled.Flush(context.Background())

	// Enabled dispatcher with empty target URL for dismissed event must make zero calls
	emptyURL := NewDispatcher(config.HooksConfig{
		Enabled:          true,
		ProposalsCreated: srv.URL,
		TimeoutSeconds:   5,
	})
	emptyURL.Emit(EventProposalsDismissed, Payload{Scope: "global", Proposals: []ProposalSummary{{ID: 2, Type: "merge", Title: "t"}}})
	emptyURL.Flush(context.Background())

	if hits.Load() != 0 {
		t.Fatalf("expected 0 webhook hits for disabled/empty URL, got %d", hits.Load())
	}
}

func TestDispatcher_Emit_RetryOn5xx(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := NewDispatcher(config.HooksConfig{
		Enabled:          true,
		ProposalsCreated: srv.URL,
		TimeoutSeconds:   5,
	})
	d.retryBackoff = 25 * time.Millisecond

	d.Emit(EventProposalsCreated, Payload{Scope: "global", Proposals: []ProposalSummary{{ID: 7, Type: "merge", Title: "Retry"}}})
	d.Flush(context.Background())

	if got := attempts.Load(); got != 2 {
		t.Fatalf("expected 2 attempts (1 initial + 1 retry on 5xx), got %d", got)
	}
}

func TestDispatcher_Emit_NoRetryOn4xx(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	d := NewDispatcher(config.HooksConfig{
		Enabled:          true,
		ProposalsApplied: srv.URL,
		TimeoutSeconds:   5,
	})
	d.retryBackoff = 10 * time.Millisecond

	d.Emit(EventProposalsApplied, Payload{Scope: "global", Proposals: []ProposalSummary{{ID: 8, Type: "link", Title: "Bad"}}})
	d.Flush(context.Background())

	if got := attempts.Load(); got != 1 {
		t.Fatalf("expected 1 attempt (no retry on 4xx), got %d", got)
	}
}

func TestDispatcher_TimeoutAndFlushBound(t *testing.T) {
	unblock := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-unblock:
		case <-r.Context().Done():
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := NewDispatcher(config.HooksConfig{
		Enabled:          true,
		ProposalsCreated: srv.URL,
		TimeoutSeconds:   1,
	})
	d.retryBackoff = 10 * time.Millisecond
	defer func() {
		close(unblock)
		d.Flush(context.Background())
	}()

	// Test() honors context timeout
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	res := d.Test(ctx, srv.URL, "")
	if res.OK {
		t.Fatalf("expected timeout failure, got OK: %+v", res)
	}
	if res.Error == "" {
		t.Errorf("expected non-empty error on timeout")
	}

	// Emit + bounded Flush returns within ctx deadline even if target hangs
	d.Emit(EventProposalsCreated, Payload{Scope: "global", Proposals: []ProposalSummary{{ID: 9, Type: "merge", Title: "Slow"}}})
	start := time.Now()
	flushCtx, flushCancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer flushCancel()
	d.Flush(flushCtx)
	if elapsed := time.Since(start); elapsed > 600*time.Millisecond {
		t.Errorf("Flush did not respect deadline; elapsed=%v", elapsed)
	}
}

func TestDispatcher_TestEndpointAndInvalidScheme(t *testing.T) {
	var gotEvent, gotSig string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEvent = r.Header.Get("X-Centmem-Event")
		gotSig = r.Header.Get("X-Centmem-Signature-256")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	d := NewDispatcher(config.DefaultHooksConfig())

	// Reject non-http/https schemes
	for _, bad := range []string{"", "ftp://example.com/hook", "file:///etc/passwd", "javascript:alert(1)", "http://"} {
		res := d.Test(context.Background(), bad, "")
		if res.OK {
			t.Errorf("expected Test(%q) to fail, got OK", bad)
		}
		if res.Error == "" {
			t.Errorf("expected error message for Test(%q)", bad)
		}
	}

	// Valid test delivery with secret
	res := d.Test(context.Background(), srv.URL, "my-test-secret")
	if !res.OK {
		t.Fatalf("expected Test to succeed, got %+v", res)
	}
	if res.StatusCode != http.StatusAccepted {
		t.Errorf("StatusCode = %d, want 202", res.StatusCode)
	}
	if gotEvent != EventHooksTest {
		t.Errorf("X-Centmem-Event = %q, want %q", gotEvent, EventHooksTest)
	}
	if want := SignPayload("my-test-secret", gotBody); gotSig != want {
		t.Errorf("signature = %q, want %q", gotSig, want)
	}
}

func TestDispatcher_Emit_RetryOnNetworkError(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n == 1 {
			if hj, ok := w.(http.Hijacker); ok {
				conn, _, err := hj.Hijack()
				if err == nil {
					_ = conn.Close()
					return
				}
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := NewDispatcher(config.HooksConfig{
		Enabled:          true,
		ProposalsCreated: srv.URL,
		TimeoutSeconds:   5,
	})
	d.retryBackoff = 20 * time.Millisecond

	d.Emit(EventProposalsCreated, Payload{
		Scope:     "global",
		Proposals: []ProposalSummary{{ID: 42, Type: "link", Title: "Net retry"}},
	})
	d.Flush(context.Background())

	if got := attempts.Load(); got != 2 {
		t.Fatalf("expected 2 attempts (1 dropped connection + 1 retry), got %d", got)
	}
}

func TestDispatcher_NilReceiverTestSafe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var nilDispatcher *Dispatcher
	res := nilDispatcher.Test(context.Background(), srv.URL, "")
	if !res.OK || res.StatusCode != http.StatusOK {
		t.Fatalf("expected nil receiver Test() to succeed without panicking, got %+v", res)
	}
}

func TestDispatcher_QueueOverflowDropsWithoutBlocking(t *testing.T) {
	unblock := make(chan struct{})
	var delivered atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-unblock
		delivered.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := NewDispatcher(config.HooksConfig{
		Enabled:          true,
		ProposalsCreated: srv.URL,
		TimeoutSeconds:   5,
	})
	d.retryBackoff = 5 * time.Millisecond

	totalToEmit := maxQueued + 10
	start := time.Now()
	for i := 0; i < totalToEmit; i++ {
		d.Emit(EventProposalsCreated, Payload{
			Scope:     "global",
			Proposals: []ProposalSummary{{ID: int64(i + 1), Type: "merge", Title: "Burst"}},
		})
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("Emit blocked under queue saturation; elapsed=%v", elapsed)
	}

	close(unblock)
	flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	d.Flush(flushCtx)

	if got := int(delivered.Load()); got > maxQueued {
		t.Errorf("expected at most %d delivered events due to queue cap, got %d", maxQueued, got)
	}
	if got := delivered.Load(); got == 0 {
		t.Errorf("expected non-zero deliveries before queue cap")
	}
}
