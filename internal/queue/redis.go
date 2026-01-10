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
// A user's priority is only updated if the new score is lower than the existing score (must be synced sooner).
// The queue priority is determined by the timestamp the event was enqueued, divided by the priorityFactor (higher
// factor will result in user being dequeued sooner).
func (q *InMemoryQueue) Enqueue(ctx context.Context, username string, priorityFactor float64) error {
	key := fmt.Sprintf("%s:%s", q.ns, SYNC_TASKS)

	// Use current timestamp as initial score; priority parameter reserved for future use
	score := float64(time.Now().UnixNano()) / priorityFactor

	if err := q.client.ZAddLT(ctx, key, redis.Z{
		Score:  score,
		Member: username,
	}).Err(); err != nil {
		return fmt.Errorf("failed to enqueue event: %w", err)
	}

	return nil
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
