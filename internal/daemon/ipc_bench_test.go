package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/embed"
	centmemv1 "github.com/aradenta-labs/cent-mem/internal/gen/centmem/v1"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

func setupTestDaemon(t testing.TB) (*Server, *Client, func()) {
	t.Helper()
	dir := t.TempDir()
	sockDir, err := os.MkdirTemp("/tmp", "cmd-")
	if err != nil {
		sockDir = dir
	}
	sockPath := filepath.Join(sockDir, "d.sock")

	cfg := config.Config{
		Home:   dir,
		DBPath: filepath.Join(dir, "centmem.db"),
		Daemon: config.DaemonConfig{
			SocketPath: sockPath,
			PIDPath:    filepath.Join(dir, "centmemd.pid"),
		},
		Retention: config.Retention{
			FactKeepDays:           0,
			NoteSummarizeAfterDays: 30,
			LogSummarizeAfterDays:  7,
		},
		Search: config.DefaultSearchConfig(),
	}

	st, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	emb := embed.NewStub(384)

	srv := NewServer(cfg, st, emb)
	if err := srv.Start(); err != nil {
		t.Fatalf("start daemon: %v", err)
	}

	client, err := NewClient(cfg.Daemon.SocketPath)
	if err != nil {
		srv.Stop()
		st.Close()
		_ = os.RemoveAll(sockDir)
		t.Fatalf("new client: %v", err)
	}

	cleanup := func() {
		client.Close()
		srv.Stop()
		st.Close()
		_ = os.RemoveAll(sockDir)
	}

	return srv, client, cleanup
}

func TestIPCLatency(t *testing.T) {
	_, client, cleanup := setupTestDaemon(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Put
	start := time.Now()
	putResp, err := client.Put(ctx, &centmemv1.PutRequest{
		Scope:     "project:bench",
		Type:      "note",
		Content:   "benchmarking unix domain socket ipc latency",
		Tags:      []string{"bench", "ipc"},
		NoSuggest: true,
	})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	putDuration := time.Since(start)
	if !putResp.Ok || putResp.Id <= 0 {
		t.Fatalf("unexpected putResp: %+v", putResp)
	}
	t.Logf("Put IPC roundtrip: %v", putDuration)

	// 2. Recall
	start = time.Now()
	recallResp, err := client.Recall(ctx, &centmemv1.RecallRequest{
		Text:    "socket ipc latency",
		Scope:   "project:bench",
		Top:     5,
		Inherit: true,
	})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	recallDuration := time.Since(start)
	if !recallResp.Ok || len(recallResp.Results) == 0 {
		t.Fatalf("unexpected recallResp: %+v", recallResp)
	}
	t.Logf("Recall IPC roundtrip: %v", recallDuration)

	// 3. Set & Get Fact
	start = time.Now()
	setResp, err := client.Set(ctx, &centmemv1.SetRequest{
		Scope: "project:bench",
		Key:   "env",
		Value: `"production"`,
	})
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !setResp.Ok {
		t.Fatalf("unexpected setResp: %+v", setResp)
	}

	getResp, err := client.Get(ctx, &centmemv1.GetRequest{
		Scope:   "project:bench",
		Key:     "env",
		Inherit: true,
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	getDuration := time.Since(start)
	if !getResp.Ok || getResp.Value != `"production"` {
		t.Fatalf("unexpected getResp: %+v", getResp)
	}
	t.Logf("Set+Get IPC roundtrip: %v", getDuration)
}

func BenchmarkIPCPut(b *testing.B) {
	_, client, cleanup := setupTestDaemon(b)
	defer cleanup()

	ctx := context.Background()
	req := &centmemv1.PutRequest{
		Scope:     "project:bench",
		Type:      "log",
		Content:   "frequent write benchmark item",
		NoSuggest: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.Put(ctx, req)
		if err != nil {
			b.Fatalf("Put failed at %d: %v", i, err)
		}
	}
}

func BenchmarkIPCRecall(b *testing.B) {
	_, client, cleanup := setupTestDaemon(b)
	defer cleanup()

	ctx := context.Background()
	// Insert initial data
	for i := 0; i < 20; i++ {
		_, _ = client.Put(ctx, &centmemv1.PutRequest{
			Scope:     "project:bench",
			Type:      "note",
			Content:   "indexed architecture document with various searchable keywords",
			NoSuggest: true,
		})
	}

	req := &centmemv1.RecallRequest{
		Text:    "searchable architecture keywords",
		Scope:   "project:bench",
		Top:     5,
		Inherit: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.Recall(ctx, req)
		if err != nil {
			b.Fatalf("Recall failed at %d: %v", i, err)
		}
	}
}
