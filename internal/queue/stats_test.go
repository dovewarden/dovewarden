package queue

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"
)

// TestQueueStats verifies that enqueue/dequeue operations are counted correctly
func TestQueueStats(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	q, err := NewInMemoryQueue("teststats", "", logger)
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer func() {
		if cerr := q.Close(); cerr != nil {
			t.Fatalf("failed to close queue: %v", cerr)
		}
	}()

	ctx := context.Background()

	// Enqueue several items
	for i := 0; i < 10; i++ {
		if err := q.Enqueue(ctx, fmt.Sprintf("user-%d", i), 1.0); err != nil {
			t.Fatalf("enqueue failed: %v", err)
		}
	}

	// Start worker pool
	wp := NewWorkerPool(q, 2, logger)
	wp.Start(ctx)

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	// Stop worker pool
	stopCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := wp.Stop(stopCtx); err != nil {
		t.Fatalf("failed to stop worker pool: %v", err)
	}

	// Get stats
	enqueues, dequeues := q.Stats()
	t.Logf("Queue stats: enqueues=%d dequeues=%d", enqueues, dequeues)

	if enqueues != 10 {
		t.Errorf("expected 10 enqueues, got %d", enqueues)
	}

	if dequeues != 10 {
		t.Errorf("expected 10 dequeues, got %d", dequeues)
	}
}
