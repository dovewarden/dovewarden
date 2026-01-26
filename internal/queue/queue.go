package queue

import (
	"context"
)

// Queue defines the interface for a priority queue implementation.
// Different backends (miniredis, external Redis) implement this interface.
type Queue interface {
	// Enqueue adds an event to the queue for a given username with a priority score.
	Enqueue(ctx context.Context, username string, priorityFactor float64) error

	// Dequeue removes and returns the username with the lowest priority score (highest priority).
	// Returns empty string and error if queue is empty or backend error occurs.
	Dequeue(ctx context.Context) (string, error)

	// HealthCheck verifies the backend is reachable and functioning.
	HealthCheck(ctx context.Context) error

	// Close closes the queue and releases resources.
	Close() error

	// GetReplicationState retrieves the stored replication state for a user.
	GetReplicationState(ctx context.Context, username string) (string, error)

	// SetReplicationState stores the replication state for a user.
	SetReplicationState(ctx context.Context, username string, state string) error
}
