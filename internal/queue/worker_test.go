package queue

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

// TestWorkerPoolDequeue verifies that workers dequeue events from the queue.
func TestWorkerPoolDequeue(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	q, err := NewInMemoryQueue("test", "")
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer func() {
		if cerr := q.Close(); cerr != nil {
			t.Fatalf("failed to close queue: %v", cerr)
		}
	}()

	ctx := context.Background()
	// Enqueue a few events
	if err := q.Enqueue(ctx, "user-a", 1.0); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}
	if err := q.Enqueue(ctx, "user-b", 1.0); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	// Create worker pool and start
	wp := NewWorkerPool(q, 2, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wp.Start(ctx)

	// Wait a bit for workers to process
	time.Sleep(1 * time.Second)

	// Queue should be empty now (events were processed)
	dequeued, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue failed: %v", err)
	}
	if dequeued != "" {
		t.Fatalf("expected empty queue, got user: %s", dequeued)
	}

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := wp.Stop(shutdownCtx); err != nil {
		t.Fatalf("failed to stop worker pool: %v", err)
	}
}

// TestWorkerPoolRequeueOnError verifies that failed events are requeued.
func TestWorkerPoolRequeueOnError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	q, err := NewInMemoryQueue("test", "")
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer func() {
		if cerr := q.Close(); cerr != nil {
			t.Fatalf("failed to close queue: %v", cerr)
		}
	}()

	ctx := context.Background()
	// Enqueue one event
	if err := q.Enqueue(ctx, "user-a", 1.0); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	// Create worker pool
	wp := NewWorkerPool(q, 1, logger)

	// Create a handler that fails once then succeeds
	failCount := int32(0)
	handler := &TestHandler{
		failOnce: true,
		onHandle: func(username string) error {
			current := atomic.AddInt32(&failCount, 1)
			if current == 1 {
				return errors.New("simulated handler failure")
			}
			return nil
		},
	}
	wp.SetHandler(handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wp.Start(ctx)
	time.Sleep(2 * time.Second)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := wp.Stop(shutdownCtx); err != nil {
		t.Fatalf("failed to stop worker pool: %v", err)
	}

	// Should have been called at least twice (fail then succeed)
	if atomic.LoadInt32(&failCount) < 2 {
		t.Fatalf("expected handler to be called at least twice, got %d", atomic.LoadInt32(&failCount))
	}
}

// TestGracefulShutdown verifies that shutdown waits for active tasks.
func TestGracefulShutdown(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	q, err := NewInMemoryQueue("test", "")
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer func() {
		if cerr := q.Close(); cerr != nil {
			t.Fatalf("failed to close queue: %v", cerr)
		}
	}()

	ctx := context.Background()
	if err := q.Enqueue(ctx, "user-a", 1.0); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	// Create worker pool with handler that takes a bit to process
	wp := NewWorkerPool(q, 1, logger)
	handler := &TestHandler{
		delay: 100 * time.Millisecond,
	}
	wp.SetHandler(handler)
	wpCtx, wpCancel := context.WithCancel(context.Background())
	defer wpCancel()
	wp.Start(wpCtx)

	time.Sleep(500 * time.Millisecond)

	// Graceful shutdown should complete
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	start := time.Now()
	if err := wp.Stop(shutdownCtx); err != nil {
		t.Fatalf("failed to stop worker pool: %v", err)
	}
	elapsed := time.Since(start)

	if elapsed > 5*time.Second {
		t.Fatalf("shutdown took too long: %v", elapsed)
	}

	// Active count should be zero
	if wp.ActiveCount() != 0 {
		t.Fatalf("expected 0 active workers, got %d", wp.ActiveCount())
	}
}

// TestHandler is a mock event handler for testing.
type TestHandler struct {
	delay    time.Duration
	failOnce bool
	onHandle func(username string) error
}

func (h *TestHandler) Handle(ctx context.Context, username string) error {
	if h.delay > 0 {
		select {
		case <-time.After(h.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if h.onHandle != nil {
		return h.onHandle(username)
	}
	return nil
}
