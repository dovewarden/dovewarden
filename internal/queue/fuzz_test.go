package queue

import (
	"context"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"
)

var totalEnqueues, totalDequeues uint64

type FuzzingHandler struct {
	usersProcessed *sync.Map
}

func (h *FuzzingHandler) Handle(ctx context.Context, username string) error {
	if rand.Intn(10) == 0 {
		return fmt.Errorf("simulated handling error for user %v", username)
	}

	h.usersProcessed.Store(username, struct{}{})
	return nil
}

// FuzzQueueDequeue stresses enqueue/dequeue and dynamic worker pool start/stop
// with a large number of users and repeated entries. It verifies we don't panic
// and that the system behaves robustly under randomized inputs.
// As this is testing with parallel workers, tests are non-deterministic.
func FuzzQueueDequeue(f *testing.F) {
	// Seed corpus with a few deterministic seeds
	f.Add([]byte("seed-1"))
	f.Add([]byte("seed-2"))
	f.Add([]byte("another-seed"))

	f.Fuzz(func(t *testing.T, seedBytes []byte) {
		// Derive deterministic seed from input bytes
		h := sha1.Sum(seedBytes)
		seed := int64(binary.BigEndian.Uint64(h[:8]))
		rnd := rand.New(rand.NewSource(seed))

		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
		q, err := NewInMemoryQueue("fuzzns", "", logger)
		if err != nil {
			t.Fatalf("failed to create queue: %v", err)
		}
		defer func() {
			if cerr := q.Close(); cerr != nil {
				t.Fatalf("failed to close queue: %v", cerr)
			}
		}()

		ctx := context.Background()

		var usersAdded = make(map[string]struct{})
		var usersProcessed sync.Map
		numWorkers := 10
		wp := NewWorkerPool(q, numWorkers, logger)
		wp.SetHandler(&FuzzingHandler{usersProcessed: &usersProcessed})
		wp.Start(ctx)

		// Generate a large set of usernames with repeats
		ops := 2000
		userCount := 500

		usernames := make([]string, userCount)
		for i := 0; i < userCount; i++ {
			usernames[i] = fmt.Sprintf("user-%d", i)
		}

		// Random priority factors around 1.0
		priorities := []float64{0.5, 0.8, 1.0, 1.2, 2.0}

		// Perform randomized enqueue operations with repeats
		for i := 0; i < ops; i++ {
			u := usernames[rnd.Intn(len(usernames))]
			pf := priorities[rnd.Intn(len(priorities))]
			if err := q.Enqueue(ctx, u, pf); err != nil {
				t.Fatalf("enqueue failed: %v", err)
			}
			usersAdded[u] = struct{}{}

			// Small random pause to vary timing
			if rnd.Intn(100) < 5 {
				time.Sleep(time.Duration(rnd.Intn(3)) * time.Millisecond)
			}
		}

		// Allow workers some time to drain the queue
		time.Sleep(500 * time.Millisecond)

		// Stop the worker pool gracefully
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := wp.Stop(shutdownCtx); err != nil && err != context.DeadlineExceeded {
			t.Fatalf("failed to stop worker pool: %v", err)
		}

		// After stop, there should be no active workers
		if wp.ActiveCount() != 0 {
			t.Fatalf("expected 0 active workers, got %d", wp.ActiveCount())
		}
		logger.Error("test")
		// Report operation counts
		enqueues, dequeues := q.Stats()
		totalEnqueues += enqueues
		totalDequeues += dequeues
		t.Logf("Fuzz iteration completed: enqueues=%d dequeues=%d\n", enqueues, dequeues)

		for u := range usersAdded {
			if _, ok := usersProcessed.Load(u); !ok {
				t.Errorf("user %v was enqueued but not processed", u)
			}
		}

		// Optionally try to dequeue anything left; shouldn't panic even if items remain
		// We don't assert full drain because fuzz may interrupt while items remain
		_, _ = q.Dequeue(ctx)
	})

	time.Sleep(15 * time.Second) // wait for log flush
	f.Logf("Fuzz run completed: enqueues=%d dequeues=%d", totalEnqueues, totalDequeues)

}
