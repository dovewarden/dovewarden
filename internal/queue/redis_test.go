package queue

import (
	"context"
	"testing"
	"time"
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
