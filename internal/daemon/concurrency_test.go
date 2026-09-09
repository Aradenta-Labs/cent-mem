package daemon

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	centmemv1 "github.com/aradenta-labs/cent-mem/internal/gen/centmem/v1"
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
