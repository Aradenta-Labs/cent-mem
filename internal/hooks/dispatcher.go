package hooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/config"
)

const (
	EventProposalsCreated   = "proposals.created"
	EventProposalsApplied   = "proposals.applied"
	EventProposalsDismissed = "proposals.dismissed"
	EventHooksTest          = "hooks.test"

	UserAgent             = "centmem/2.2.0"
	DefaultTimeoutSeconds = 5

	maxConcurrency      = 4
	maxQueued           = 64
	defaultRetryBackoff = 500 * time.Millisecond
)

// ProposalSummary is the minimal proposal metadata included in webhook envelopes.
type ProposalSummary struct {
	ID    int64  `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
}

// Payload is the canonical JSON envelope delivered to configured webhook URLs.
type Payload struct {
	Event     string            `json:"event"`
	Timestamp string            `json:"timestamp"`
	Scope     string            `json:"scope"`
	Count     int               `json:"count"`
	Proposals []ProposalSummary `json:"proposals"`
}

// TestResult reports the outcome of a synchronous webhook connectivity test.
type TestResult struct {
	OK         bool   `json:"ok"`
	StatusCode int    `json:"status_code,omitempty"`
	LatencyMs  int64  `json:"latency_ms"`
	Error      string `json:"error,omitempty"`
}

// Dispatcher sends asynchronous, bounded, non-blocking HTTP webhook notifications.
type Dispatcher struct {
	cfg          config.HooksConfig
	client       *http.Client
	sem          chan struct{}
	pending      atomic.Int64
	wg           sync.WaitGroup
	retryBackoff time.Duration
}

// NewDispatcher constructs a webhook dispatcher from HooksConfig.
// When cfg.Enabled is false, Emit is a strict zero-allocation, zero-network no-op.
func NewDispatcher(cfg config.HooksConfig) *Dispatcher {
	timeoutSec := cfg.TimeoutSeconds
	if timeoutSec <= 0 {
		timeoutSec = DefaultTimeoutSeconds
	}
	cfg.TimeoutSeconds = timeoutSec

	return &Dispatcher{
		cfg: cfg,
		client: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
		},
		sem:          make(chan struct{}, maxConcurrency),
		retryBackoff: defaultRetryBackoff,
	}
}

// SignPayload returns the X-Centmem-Signature-256 header value ("sha256=<hex>").
func SignPayload(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func (d *Dispatcher) urlForEvent(event string) string {
	switch event {
	case EventProposalsCreated:
		return strings.TrimSpace(d.cfg.ProposalsCreated)
	case EventProposalsApplied:
		return strings.TrimSpace(d.cfg.ProposalsApplied)
	case EventProposalsDismissed:
		return strings.TrimSpace(d.cfg.ProposalsDismissed)
	default:
		return ""
	}
}

// Emit schedules an asynchronous webhook delivery without blocking the caller.
// It is a strict no-op if the dispatcher is nil, hooks are disabled, or no URL
// is configured for the given event.
func (d *Dispatcher) Emit(event string, payload Payload) {
	if d == nil || !d.cfg.Enabled {
		return
	}

	targetURL := d.urlForEvent(event)
	if targetURL == "" {
		return
	}
	if err := config.ValidateHookURL(targetURL); err != nil {
		fmt.Fprintf(os.Stderr, "[centmem/hooks] skipping invalid webhook URL for %s: %v\n", event, err)
		return
	}

	payload.Event = event
	if payload.Timestamp == "" {
		payload.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	if payload.Proposals == nil {
		payload.Proposals = []ProposalSummary{}
	}
	if payload.Count == 0 && len(payload.Proposals) > 0 {
		payload.Count = len(payload.Proposals)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[centmem/hooks] failed to marshal %s payload: %v\n", event, err)
		return
	}

	if d.pending.Add(1) > maxQueued {
		d.pending.Add(-1)
		fmt.Fprintf(os.Stderr, "[centmem/hooks] webhook queue full (%d); dropping %s event\n", maxQueued, event)
		return
	}

	secretCfg := d.cfg.Secret
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		defer d.pending.Add(-1)
		d.sem <- struct{}{}
		defer func() { <-d.sem }()

		d.deliverWithRetry(targetURL, event, body, secretCfg)
	}()
}

// Flush waits for all in-flight webhook deliveries to finish or until ctx expires.
func (d *Dispatcher) Flush(ctx context.Context) {
	if d == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
	}
}

// Test performs a synchronous dry-run POST with a hooks.test payload.
func (d *Dispatcher) Test(ctx context.Context, targetURL, secret string) TestResult {
	targetURL = strings.TrimSpace(targetURL)
	if targetURL == "" {
		return TestResult{OK: false, Error: "webhook URL is required"}
	}
	if err := config.ValidateHookURL(targetURL); err != nil {
		return TestResult{OK: false, Error: err.Error()}
	}

	payload := Payload{
		Event:     EventHooksTest,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Scope:     "global",
		Count:     0,
		Proposals: []ProposalSummary{},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return TestResult{OK: false, Error: fmt.Sprintf("marshal test payload: %v", err)}
	}

	start := time.Now()
	statusCode, err := d.sendOnce(ctx, targetURL, EventHooksTest, body, secret)
	latencyMs := time.Since(start).Milliseconds()

	if err != nil {
		return TestResult{
			OK:        false,
			LatencyMs: latencyMs,
			Error:     err.Error(),
		}
	}
	if statusCode >= 200 && statusCode < 300 {
		return TestResult{
			OK:         true,
			StatusCode: statusCode,
			LatencyMs:  latencyMs,
		}
	}
	return TestResult{
		OK:         false,
		StatusCode: statusCode,
		LatencyMs:  latencyMs,
		Error:      fmt.Sprintf("HTTP %d %s", statusCode, http.StatusText(statusCode)),
	}
}

func (d *Dispatcher) deliverWithRetry(targetURL, event string, body []byte, secretCfg string) {
	statusCode, err := d.sendOnce(context.Background(), targetURL, event, body, secretCfg)
	if err == nil && statusCode >= 200 && statusCode < 300 {
		return
	}

	if err != nil || (statusCode >= 500 && statusCode <= 599) {
		time.Sleep(d.retryBackoff)
		statusCode, err = d.sendOnce(context.Background(), targetURL, event, body, secretCfg)
		if err == nil && statusCode >= 200 && statusCode < 300 {
			return
		}
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "[centmem/hooks] webhook delivery failed (%s -> %s): %v\n", event, targetURL, err)
	} else {
		fmt.Fprintf(os.Stderr, "[centmem/hooks] webhook delivery failed (%s -> %s): HTTP %d\n", event, targetURL, statusCode)
	}
}

func (d *Dispatcher) sendOnce(ctx context.Context, targetURL, event string, body []byte, secretCfg string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("X-Centmem-Event", event)

	if resolved, _ := config.ResolveAPIKey(secretCfg); resolved != "" {
		req.Header.Set("X-Centmem-Signature-256", SignPayload(resolved, body))
	}

	client := &http.Client{Timeout: DefaultTimeoutSeconds * time.Second}
	if d != nil && d.client != nil {
		client = d.client
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
	}()
	return resp.StatusCode, nil
}
