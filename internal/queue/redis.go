package queue

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync/atomic"
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
	logger *slog.Logger

	// operation counters
	enqueueCount uint64
	dequeueCount uint64
}

// NewInMemoryQueue creates a new in-memory Redis queue.
// addr parameter allows specifying the address for miniredis (for testing).
func NewInMemoryQueue(namespace string, addr string, logger *slog.Logger) (*InMemoryQueue, error) {
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
		logger: logger,
	}, nil
}

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
	atomic.AddUint64(&q.enqueueCount, 1)
	return nil
}

// Dequeue removes and returns the username with the lowest priority score (highest priority).
// Returns empty string if queue is empty.
func (q *InMemoryQueue) Dequeue(ctx context.Context) (string, error) {
	key := fmt.Sprintf("%s:%s", q.ns, SYNC_TASKS)
	// Using BZPopMin would be preferable to avoid busy-waiting, but miniredis does not support it
	// https://github.com/alicebob/miniredis/issues/428
	result, err := q.client.ZPopMin(ctx, key).Result()
	if err != nil {
		return "", fmt.Errorf("failed to dequeue: %w", err)
	}
	if len(result) == 0 {
		return "", nil
	}
	atomic.AddUint64(&q.dequeueCount, 1)
	return result[0].Member.(string), nil
}

// Stats returns the total number of enqueue and dequeue operations.
func (q *InMemoryQueue) Stats() (enqueues uint64, dequeues uint64) {
	return atomic.LoadUint64(&q.enqueueCount), atomic.LoadUint64(&q.dequeueCount)
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

// GetReplicationState retrieves the stored replication state for a user.
// Returns empty string if no state exists.
func (q *InMemoryQueue) GetReplicationState(ctx context.Context, username string) (string, error) {
	key := fmt.Sprintf("%s:state:%s", q.ns, username)
	state, err := q.client.Get(ctx, key).Result()
	if err == redis.Nil {
		// No state stored yet
		q.logger.Debug("replication state not found", "username", username, "key", key)
		return "", nil
	}
	if err != nil {
		q.logger.Debug("failed to get replication state", "username", username, "key", key, "error", err)
		return "", fmt.Errorf("failed to get replication state: %w", err)
	}
	q.logger.Debug("retrieved replication state", "username", username, "key", key, "state", state)
	return state, nil
}

// SetReplicationState stores the replication state for a user.
// The state is used for incremental sync in the next replication.
// State expires after 30 days to prevent unbounded Redis memory growth.
func (q *InMemoryQueue) SetReplicationState(ctx context.Context, username string, state string) error {
	key := fmt.Sprintf("%s:state:%s", q.ns, username)
	// Set TTL to 30 days - states older than this are considered stale
	ttl := 30 * 24 * time.Hour
	if err := q.client.Set(ctx, key, state, ttl).Err(); err != nil {
		q.logger.Debug("failed to set replication state", "username", username, "key", key, "state", state, "error", err)
		return fmt.Errorf("failed to set replication state: %w", err)
	}
	q.logger.Debug("stored replication state", "username", username, "key", key, "state", state, "ttl", ttl)
	return nil
}

// GetLastReplicationTime retrieves the timestamp of the last replication for a user.
// Returns zero time if no replication has been performed.
func (q *InMemoryQueue) GetLastReplicationTime(ctx context.Context, username string) (time.Time, error) {
	key := fmt.Sprintf("%s:last_replication:%s", q.ns, username)
	timestampStr, err := q.client.Get(ctx, key).Result()
	if err == redis.Nil {
		// No timestamp stored yet
		q.logger.Debug("last replication time not found", "username", username, "key", key)
		return time.Time{}, nil
	}
	if err != nil {
		q.logger.Debug("failed to get last replication time", "username", username, "key", key, "error", err)
		return time.Time{}, fmt.Errorf("failed to get last replication time: %w", err)
	}

	// Parse Unix timestamp
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	t := time.Unix(timestamp, 0)
	q.logger.Debug("retrieved last replication time", "username", username, "key", key, "time", t)
	return t, nil
}

// SetLastReplicationTime stores the timestamp of the last replication for a user.
// The timestamp expires after 30 days to prevent unbounded Redis memory growth.
func (q *InMemoryQueue) SetLastReplicationTime(ctx context.Context, username string, t time.Time) error {
	key := fmt.Sprintf("%s:last_replication:%s", q.ns, username)
	// Store as Unix timestamp
	timestampStr := strconv.FormatInt(t.Unix(), 10)
	// Set TTL to 30 days - timestamps older than this are considered stale
	ttl := 30 * 24 * time.Hour
	if err := q.client.Set(ctx, key, timestampStr, ttl).Err(); err != nil {
		q.logger.Debug("failed to set last replication time", "username", username, "key", key, "time", t, "error", err)
		return fmt.Errorf("failed to set last replication time: %w", err)
	}
	q.logger.Debug("stored last replication time", "username", username, "key", key, "time", t, "ttl", ttl)
	return nil
}
