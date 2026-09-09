package daemon

import (
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	centmemv1 "github.com/aradenta-labs/cent-mem/internal/gen/centmem/v1"
)

func TestSyncService_StreamEvents(t *testing.T) {
	_, client, cleanup := setupTestDaemon(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Insert an event prior to subscribing (historical)
	putResp, err := client.Put(ctx, &centmemv1.PutRequest{
		Scope:     "project:sync",
		Type:      "note",
		Content:   "historical memory for sync test",
		NoSuggest: true,
	})
	if err != nil {
		t.Fatalf("Put historical: %v", err)
	}

	// 2. Start streaming events from since_id = 0
	syncClient := client.SyncClient()
	stream, err := syncClient.StreamEvents(ctx, &centmemv1.StreamEventsRequest{
		SinceId: 0,
		Scope:   "project:sync",
	})
	if err != nil {
		t.Fatalf("StreamEvents: %v", err)
	}

	// Read historical event
	ev1, err := stream.Recv()
	if err != nil {
		t.Fatalf("Recv ev1: %v", err)
	}
	if ev1.MemoryId != putResp.Id {
		t.Errorf("ev1.MemoryId = %d, want %d", ev1.MemoryId, putResp.Id)
	}

	// 3. Concurrently insert a new memory while streaming (live event)
	var livePutID int64
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond)
		resp, pErr := client.Put(ctx, &centmemv1.PutRequest{
			Scope:     "project:sync",
			Type:      "note",
			Content:   "live streamed memory",
			NoSuggest: true,
		})
		if pErr == nil {
			livePutID = resp.Id
		}
	}()

	ev2, err := stream.Recv()
	if err != nil {
		t.Fatalf("Recv ev2: %v", err)
	}
	wg.Wait()

	if ev2.MemoryId != livePutID {
		t.Errorf("ev2.MemoryId = %d, want %d", ev2.MemoryId, livePutID)
	}
}

func TestSyncService_PushEventsConflictResolution(t *testing.T) {
	_, client, cleanup := setupTestDaemon(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	syncClient := client.SyncClient()
	stream, err := syncClient.PushEvents(ctx)
	if err != nil {
		t.Fatalf("PushEvents: %v", err)
	}

	// 1. Send Note event
	err = stream.Send(&centmemv1.Event{
		Id:          1,
		Op:          "insert",
		ScopePath:   "project:sync",
		PayloadJson: `{"type":"note","content":"push sync note 1"}`,
		CreatedAt:   time.Now().Unix(),
	})
	if err != nil {
		t.Fatalf("Send note: %v", err)
	}

	// 2. Send duplicate Note event (deduplication by content hash)
	err = stream.Send(&centmemv1.Event{
		Id:          2,
		Op:          "insert",
		ScopePath:   "project:sync",
		PayloadJson: `{"type":"note","content":"push sync note 1"}`,
		CreatedAt:   time.Now().Unix(),
	})
	if err != nil {
		t.Fatalf("Send duplicate note: %v", err)
	}

	// 3. Send Fact event (older timestamp)
	tNow := time.Now()
	err = stream.Send(&centmemv1.Event{
		Id:          3,
		Op:          "set",
		ScopePath:   "project:sync",
		PayloadJson: `{"type":"fact","key":"cluster_state","value":"active"}`,
		CreatedAt:   tNow.Unix(),
	})
	if err != nil {
		t.Fatalf("Send fact: %v", err)
	}

	// 4. Send conflicting Fact event with older timestamp (should be dropped as conflict via LWW)
	err = stream.Send(&centmemv1.Event{
		Id:          4,
		Op:          "set",
		ScopePath:   "project:sync",
		PayloadJson: `{"type":"fact","key":"cluster_state","value":"stale"}`,
		CreatedAt:   tNow.Add(-1 * time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("Send older fact: %v", err)
	}

	summary, err := stream.CloseAndRecv()
	if err != nil && err != io.EOF {
		t.Fatalf("CloseAndRecv: %v", err)
	}

	if !summary.Ok {
		t.Fatalf("Push summary not ok: %+v", summary)
	}
	if summary.Received != 4 {
		t.Errorf("summary.Received = %d, want 4", summary.Received)
	}
	if summary.Applied != 2 {
		t.Errorf("summary.Applied = %d, want 2", summary.Applied)
	}
	if summary.Conflicts != 2 {
		t.Errorf("summary.Conflicts = %d, want 2", summary.Conflicts)
	}

	// Verify the fact has the newer value "active"
	getResp, err := client.Get(ctx, &centmemv1.GetRequest{
		Scope: "project:sync",
		Key:   "cluster_state",
	})
	if err != nil {
		t.Fatalf("Get fact: %v", err)
	}
	if getResp.Value != `"active"` && getResp.Value != "active" {
		t.Errorf("expected fact value active, got %s", getResp.Value)
	}
	_ = fmt.Sprint()
}
