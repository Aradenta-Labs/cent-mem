package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	centmemv1 "github.com/aradenta-labs/cent-mem/internal/gen/centmem/v1"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

func TestConcurrentWrites(t *testing.T) {
	TestConcurrentWriteStress(t)
}

func TestConcurrentWriteStress(t *testing.T) {
	_, client, cleanup := setupTestDaemon(t)
	defer cleanup()

	const numWorkers = 50
	const opsPerWorker = 20

	var wg sync.WaitGroup
	var successCount int64
	var failureCount int64
	errCh := make(chan error, numWorkers*opsPerWorker)

	startSignal := make(chan struct{})

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			<-startSignal

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			for i := 0; i < opsPerWorker; i++ {
				scope := fmt.Sprintf("project:stress/agent:worker-%d", workerID)
				if i%2 == 0 {
					// Put request
					content := fmt.Sprintf("concurrent stress test worker %d op %d timestamp %d", workerID, i, time.Now().UnixNano())
					resp, err := client.Put(ctx, &centmemv1.PutRequest{
						Scope:     scope,
						Type:      "log",
						Content:   content,
						NoSuggest: true,
					})
					if err != nil {
						atomic.AddInt64(&failureCount, 1)
						errCh <- fmt.Errorf("worker %d put %d: %w", workerID, i, err)
						return
					}
					if !resp.Ok || resp.Id <= 0 {
						atomic.AddInt64(&failureCount, 1)
						errCh <- fmt.Errorf("worker %d put %d bad response: %+v", workerID, i, resp)
						return
					}
					atomic.AddInt64(&successCount, 1)
				} else {
					// Set request
					key := fmt.Sprintf("status_%d", i)
					value := fmt.Sprintf(`{"iteration":%d,"worker":%d}`, i, workerID)
					resp, err := client.Set(ctx, &centmemv1.SetRequest{
						Scope: scope,
						Key:   key,
						Value: value,
					})
					if err != nil {
						atomic.AddInt64(&failureCount, 1)
						errCh <- fmt.Errorf("worker %d set %d: %w", workerID, i, err)
						return
					}
					if !resp.Ok || resp.Id <= 0 {
						atomic.AddInt64(&failureCount, 1)
						errCh <- fmt.Errorf("worker %d set %d bad response: %+v", workerID, i, resp)
						return
					}
					atomic.AddInt64(&successCount, 1)
				}
			}
		}(w)
	}

	close(startSignal)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrency error: %v", err)
	}

	expectedSuccess := int64(numWorkers * opsPerWorker)
	if successCount != expectedSuccess {
		t.Fatalf("expected %d successful writes, got %d (failures: %d)", expectedSuccess, successCount, failureCount)
	}

	// Verify stats show correct counts
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stats, err := client.Stats(ctx, &centmemv1.StatsRequest{})
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if stats.TotalMemories < expectedSuccess {
		t.Fatalf("expected at least %d memories in stats, got %d", expectedSuccess, stats.TotalMemories)
	}
}

func TestConcurrentWritesAndCuration(t *testing.T) {
	srv, client, cleanup := setupTestDaemon(t)
	defer cleanup()

	st := srv.st
	const numWriteWorkers = 30
	const opsPerWriteWorker = 10
	const numCurateWorkers = 5
	const opsPerCurateWorker = 6

	var wg sync.WaitGroup
	var successWrites int64
	var successCurations int64
	errCh := make(chan error, (numWriteWorkers*opsPerWriteWorker)+(numCurateWorkers*opsPerCurateWorker)+10)

	startSignal := make(chan struct{})

	// 1. Spawn concurrent write workers via Daemon IPC
	for w := 0; w < numWriteWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			<-startSignal

			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			for i := 0; i < opsPerWriteWorker; i++ {
				scope := fmt.Sprintf("project:curatestress/agent:worker-%d", workerID)
				if i%2 == 0 {
					content := fmt.Sprintf("concurrent stress content worker %d op %d nano %d", workerID, i, time.Now().UnixNano())
					resp, err := client.Put(ctx, &centmemv1.PutRequest{
						Scope:     scope,
						Type:      "note",
						Content:   content,
						NoSuggest: true,
					})
					if err != nil {
						errCh <- fmt.Errorf("write worker %d put %d: %w", workerID, i, err)
						return
					}
					if !resp.Ok || resp.Id <= 0 {
						errCh <- fmt.Errorf("write worker %d put %d bad resp: %+v", workerID, i, resp)
						return
					}
					atomic.AddInt64(&successWrites, 1)
				} else {
					key := fmt.Sprintf("curate_key_%d", i)
					value := fmt.Sprintf(`{"iteration":%d,"w":%d}`, i, workerID)
					resp, err := client.Set(ctx, &centmemv1.SetRequest{
						Scope: scope,
						Key:   key,
						Value: value,
					})
					if err != nil {
						errCh <- fmt.Errorf("write worker %d set %d: %w", workerID, i, err)
						return
					}
					if !resp.Ok || resp.Id <= 0 {
						errCh <- fmt.Errorf("write worker %d set %d bad resp: %+v", workerID, i, resp)
						return
					}
					atomic.AddInt64(&successWrites, 1)
				}
			}
		}(w)
	}

	// 2. Spawn concurrent curation workers executing store transactions
	for c := 0; c < numCurateWorkers; c++ {
		wg.Add(1)
		go func(curatorID int) {
			defer wg.Done()
			<-startSignal

			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			for i := 0; i < opsPerCurateWorker; i++ {
				scope := fmt.Sprintf("project:curatestress/agent:curator-%d", curatorID)
				// Seed two memories to link or merge
				m1, _, err1 := st.PutMemory(ctx, store.MemoryInput{
					Scope:   scope,
					Type:    "note",
					Content: fmt.Sprintf("Curator %d item A op %d nano %d", curatorID, i, time.Now().UnixNano()),
				})
				m2, _, err2 := st.PutMemory(ctx, store.MemoryInput{
					Scope:   scope,
					Type:    "note",
					Content: fmt.Sprintf("Curator %d item B op %d nano %d", curatorID, i, time.Now().UnixNano()),
				})
				if err1 != nil || err2 != nil {
					errCh <- fmt.Errorf("curator %d seed memory err: %v / %v", curatorID, err1, err2)
					return
				}

				if i%2 == 0 {
					// Link proposal
					linkPayload, _ := json.Marshal(store.LinkProposalPayload{
						FromID:   m1,
						ToID:     m2,
						Relation: "supersedes",
					})
					propID, err := st.CreateProposal(ctx, &store.Proposal{
						ScopePath:    scope,
						ProposalType: "link",
						Title:        fmt.Sprintf("Link %d -> %d", m1, m2),
						Reasoning:    "Concurrent link stress",
						PayloadJSON:  string(linkPayload),
					})
					if err != nil {
						errCh <- fmt.Errorf("curator %d create link proposal: %w", curatorID, err)
						return
					}
					if err := st.ApplyProposal(ctx, propID); err != nil {
						errCh <- fmt.Errorf("curator %d apply link proposal: %w", curatorID, err)
						return
					}
					atomic.AddInt64(&successCurations, 1)
				} else {
					// Merge proposal
					mergePayload, _ := json.Marshal(store.MergeProposalPayload{
						SourceIDs:     []int64{m1, m2},
						TargetContent: fmt.Sprintf("Consolidated curator %d op %d", curatorID, i),
						TargetTags:    []string{"consolidated"},
					})
					propID, err := st.CreateProposal(ctx, &store.Proposal{
						ScopePath:    scope,
						ProposalType: "merge",
						Title:        fmt.Sprintf("Merge %d and %d", m1, m2),
						Reasoning:    "Concurrent merge stress",
						PayloadJSON:  string(mergePayload),
					})
					if err != nil {
						errCh <- fmt.Errorf("curator %d create merge proposal: %w", curatorID, err)
						return
					}
					if err := st.ApplyProposal(ctx, propID); err != nil {
						errCh <- fmt.Errorf("curator %d apply merge proposal: %w", curatorID, err)
						return
					}
					atomic.AddInt64(&successCurations, 1)
				}
			}
		}(c)
	}

	close(startSignal)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrency error: %v", err)
	}

	expectedWrites := int64(numWriteWorkers * opsPerWriteWorker)
	if successWrites != expectedWrites {
		t.Fatalf("expected %d successful writes, got %d", expectedWrites, successWrites)
	}

	expectedCurations := int64(numCurateWorkers * opsPerCurateWorker)
	if successCurations != expectedCurations {
		t.Fatalf("expected %d successful curations, got %d", expectedCurations, successCurations)
	}
}

