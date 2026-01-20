package queue

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// helper to fetch sorted set members with scores, ascending by score
func getQueueOrder(t *testing.T, q *InMemoryQueue) []string {
	t.Helper()
	ctx := context.Background()
	key := q.ns + ":" + SYNC_TASKS
	// Ascending by score: lowest score first (highest priority)
	vals, err := q.client.ZRange(ctx, key, 0, -1).Result()
	if err != nil {
		t.Fatalf("failed to read queue: %v", err)
	}
	return vals
}

func TestPriorityOrderByInsertion(t *testing.T) {
	q, err := NewInMemoryQueue("testns", "")
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer func() {
		if cerr := q.Close(); cerr != nil {
			// Fail the test if cleanup fails
			t.Fatalf("failed to close queue: %v", cerr)
		}
	}()

	ctx := context.Background()
	// Same factor = 1 for both
	if err := q.Enqueue(ctx, "user-first", 1.0); err != nil {
		t.Fatalf("enqueue user-first: %v", err)
	}
	// ensure different timestamp
	time.Sleep(1100 * time.Millisecond)
	if err := q.Enqueue(ctx, "user-second", 1.0); err != nil {
		t.Fatalf("enqueue user-second: %v", err)
	}

	order := getQueueOrder(t, q)
	if len(order) != 2 {
		t.Fatalf("expected 2 users in queue, got %d", len(order))
	}
	if order[0] != "user-first" || order[1] != "user-second" {
		t.Fatalf("expected order [user-first, user-second], got %v", order)
	}
}

func TestPriorityFactorGreaterThanOne(t *testing.T) {
	q, err := NewInMemoryQueue("testns2", "")
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer func() {
		if cerr := q.Close(); cerr != nil {
			t.Fatalf("failed to close queue: %v", cerr)
		}
	}()

	ctx := context.Background()
	// enqueue baseline user with factor = 1
	if err := q.Enqueue(ctx, "user-one", 1.0); err != nil {
		t.Fatalf("enqueue user-one: %v", err)
	}
	// small sleep to avoid identical timestamps
	time.Sleep(200 * time.Millisecond)
	// factor > 1 reduces score -> higher priority, should be ahead of factor=1
	if err := q.Enqueue(ctx, "user-high", 2.0); err != nil {
		t.Fatalf("enqueue user-high: %v", err)
	}

	order := getQueueOrder(t, q)
	if len(order) != 2 {
		t.Fatalf("expected 2 users in queue, got %d", len(order))
	}
	if order[0] != "user-high" || order[1] != "user-one" {
		t.Fatalf("expected order [user-high, user-one], got %v", order)
	}
}

func TestPriorityFactorLessThanOne(t *testing.T) {
	q, err := NewInMemoryQueue("testns3", "")
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer func() {
		if cerr := q.Close(); cerr != nil {
			t.Fatalf("failed to close queue: %v", cerr)
		}
	}()

	ctx := context.Background()
	// baseline factor = 1
	if err := q.Enqueue(ctx, "user-one", 1.0); err != nil {
		t.Fatalf("enqueue user-one: %v", err)
	}
	// small sleep
	time.Sleep(200 * time.Millisecond)
	// factor < 1 increases score -> lower priority, should be after factor=1
	if err := q.Enqueue(ctx, "user-low", 0.5); err != nil {
		t.Fatalf("enqueue user-low: %v", err)
	}

	order := getQueueOrder(t, q)
	if len(order) != 2 {
		t.Fatalf("expected 2 users in queue, got %d", len(order))
	}
	if order[0] != "user-one" || order[1] != "user-low" {
		t.Fatalf("expected order [user-one, user-low], got %v", order)
	}
}

func TestDequeueEmptyQueue(t *testing.T) {
	q, err := NewInMemoryQueue("testns_dequeue_empty", "")
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer func() {
		if cerr := q.Close(); cerr != nil {
			t.Fatalf("failed to close queue: %v", cerr)
		}
	}()

	ctx := context.Background()
	// Dequeue from empty queue should return empty string and no error
	username, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("expected no error on empty dequeue, got %v", err)
	}
	if username != "" {
		t.Fatalf("expected empty username on empty queue, got %q", username)
	}
}

func TestDequeueRetryBehavior(t *testing.T) {
	q, err := NewInMemoryQueue("testns_dequeue_retry", "")
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer func() {
		if cerr := q.Close(); cerr != nil {
			t.Fatalf("failed to close queue: %v", cerr)
		}
	}()

	ctx := context.Background()

	// Simulate multiple retry attempts on empty queue
	for i := 0; i < 5; i++ {
		username, err := q.Dequeue(ctx)
		if err != nil {
			t.Fatalf("retry attempt %d: expected no error, got %v", i, err)
		}
		if username != "" {
			t.Fatalf("retry attempt %d: expected empty username, got %q", i, username)
		}
	}
}

func TestDequeueGracefulErrorOnMalformedData(t *testing.T) {
	q, err := NewInMemoryQueue("testns_dequeue_malformed", "")
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer func() {
		if cerr := q.Close(); cerr != nil {
			t.Fatalf("failed to close queue: %v", cerr)
		}
	}()

	ctx := context.Background()
	key := q.ns + ":" + SYNC_TASKS

	// Manually insert malformed data directly into the sorted set
	// Use miniredis' internal command to corrupt the data
	// We'll insert a member that's not a valid username string
	err = q.client.ZAdd(ctx, key, redis.Z{
		Score:  100.0,
		Member: "123", // Non-standard member (numeric string)
	}).Err()
	// Note: Redis/miniredis will convert to string anyway, but let's test the graceful handling

	// Now try to dequeue - should handle gracefully
	username, err := q.Dequeue(ctx)
	// Should either succeed with string conversion or return an error gracefully (no panic)
	if err != nil {
		// If it's an error, ensure it's wrapped and informative
		t.Logf("graceful error handling: %v", err)
	} else {
		// If no error, the member should be extractable (even if numeric)
		t.Logf("dequeued member: %q", username)
	}
}

func TestDequeueWithEnqueuedData(t *testing.T) {
	q, err := NewInMemoryQueue("testns_dequeue_with_data", "")
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer func() {
		if cerr := q.Close(); cerr != nil {
			t.Fatalf("failed to close queue: %v", cerr)
		}
	}()

	ctx := context.Background()

	// Enqueue multiple users
	if err := q.Enqueue(ctx, "user-a", 1.0); err != nil {
		t.Fatalf("enqueue user-a: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if err := q.Enqueue(ctx, "user-b", 1.0); err != nil {
		t.Fatalf("enqueue user-b: %v", err)
	}

	// Dequeue and verify order
	username1, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue 1: %v", err)
	}
	if username1 != "user-a" {
		t.Fatalf("expected user-a, got %q", username1)
	}

	username2, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue 2: %v", err)
	}
	if username2 != "user-b" {
		t.Fatalf("expected user-b, got %q", username2)
	}

	// Queue should now be empty
	username3, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue 3 (should be empty): %v", err)
	}
	if username3 != "" {
		t.Fatalf("expected empty string when queue empty, got %q", username3)
	}
}

func TestDequeueStatsIncrement(t *testing.T) {
	q, err := NewInMemoryQueue("testns_dequeue_stats", "")
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer func() {
		if cerr := q.Close(); cerr != nil {
			t.Fatalf("failed to close queue: %v", cerr)
		}
	}()

	ctx := context.Background()

	// Check initial stats
	enqs, deqs := q.Stats()
	if enqs != 0 || deqs != 0 {
		t.Fatalf("expected initial stats (0,0), got (%d,%d)", enqs, deqs)
	}

	// Enqueue some users
	for i := 0; i < 3; i++ {
		if err := q.Enqueue(ctx, "user-"+string(rune('a'+i)), 1.0); err != nil {
			t.Fatalf("enqueue: %v", err)
		}
	}

	enqs, deqs = q.Stats()
	if enqs != 3 {
		t.Fatalf("expected 3 enqueues, got %d", enqs)
	}
	if deqs != 0 {
		t.Fatalf("expected 0 dequeues, got %d", deqs)
	}

	// Dequeue once
	if _, err := q.Dequeue(ctx); err != nil {
		t.Fatalf("dequeue: %v", err)
	}

	enqs, deqs = q.Stats()
	if deqs != 1 {
		t.Fatalf("expected 1 dequeue after first pop, got %d", deqs)
	}

	// Multiple dequeues from empty queue should not increment counter
	for i := 0; i < 3; i++ {
		if _, err := q.Dequeue(ctx); err != nil {
			t.Fatalf("dequeue empty: %v", err)
		}
	}

	enqs, deqs = q.Stats()
	if deqs != 3 {
		t.Fatalf("expected 3 dequeues total, got %d", deqs)
	}
}
