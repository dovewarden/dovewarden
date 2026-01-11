package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

const SYNC_TASKS = "sync_tasks"

// InMemoryQueue is a Redis-compatible queue using miniredis for development and testing.
type InMemoryQueue struct {
	server *miniredis.Miniredis
	client *redis.Client
	ns     string
}

// NewInMemoryQueue creates a new in-memory Redis queue.
// addr parameter allows specifying the address for miniredis (for testing).
func NewInMemoryQueue(namespace string, addr string) (*InMemoryQueue, error) {
	s := miniredis.NewMiniRedis()
	if addr != "" {
		if err := s.StartAddr(addr); err != nil {
			return nil, fmt.Errorf("failed to start miniredis at %s: %w", addr, err)
		}
	} else {
		if err := s.Start(); err != nil {
			return nil, fmt.Errorf("failed to start miniredis: %w", err)
		}
	}

	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		s.Close()
		return nil, fmt.Errorf("failed to ping miniredis: %w", err)
	}

	return &InMemoryQueue{
		server: s,
		client: client,
		ns:     namespace,
	}, nil
}

// Enqueue adds or updates a user to the priority queue.
// Uses a sorted set with the current timestamp as the score (lower score = higher priority initially).
// Enqueue adds or updates a user to the priority queue.
// Uses a sorted set with the timestamp divided by the priority factor as the score.
// Lower score = higher priority.
// factor=1.0 = normal priority (scores are timestamps)
// factor>1.0 = higher priority (scores are reduced by factor)
// factor<1.0 = lower priority (scores are increased by factor)
func (q *InMemoryQueue) Enqueue(ctx context.Context, username string, priorityFactor float64) error {
	key := fmt.Sprintf("%s:%s", q.ns, SYNC_TASKS)

	// Use current timestamp as base score
	timestamp := float64(time.Now().UnixNano()) / 1e9

	// Apply priority factor: divide by factor to adjust priority
	// factor > 1.0 produces lower score (higher priority)
	// factor < 1.0 produces higher score (lower priority)
	if priorityFactor <= 0 {
		priorityFactor = 1.0 // Safety: avoid division by zero
	}
	score := timestamp / priorityFactor

	if err := q.client.ZAddLT(ctx, key, redis.Z{
		Score:  score,
		Member: username,
	}).Err(); err != nil {
		return fmt.Errorf("failed to enqueue event: %w", err)
	}

	return nil
}

// Dequeue removes and returns the username with the lowest priority score (highest priority).
// Returns empty string if queue is empty.
func (q *InMemoryQueue) Dequeue(ctx context.Context) (string, error) {
	key := fmt.Sprintf("%s:%s", q.ns, SYNC_TASKS)

	// ZPOPMIN returns the member with the lowest score
	result, err := q.client.ZPopMin(ctx, key).Result()
	if err != nil {
		return "", fmt.Errorf("failed to dequeue: %w", err)
	}

	if len(result) == 0 {
		// Queue is empty
		return "", nil
	}

	return result[0].Member.(string), nil
}

// HealthCheck checks connectivity to the in-memory Redis client.
func (q *InMemoryQueue) HealthCheck(ctx context.Context) error {
	return q.client.Ping(ctx).Err()
}

// Close closes the queue and releases resources.
func (q *InMemoryQueue) Close() error {
	if err := q.client.Close(); err != nil {
		return fmt.Errorf("failed to close client: %w", err)
	}
	q.server.Close()
	return nil
}

// GetQueueSize returns the current size of the queue for a given username (for metrics).
func (q *InMemoryQueue) GetQueueSize(ctx context.Context, username string) (int64, error) {
	key := fmt.Sprintf("%s:%s", q.ns, username)
	size, err := q.client.ZCard(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get queue size: %w", err)
	}
	return size, nil
}
