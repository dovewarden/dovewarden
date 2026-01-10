package queue

import (
	"context"
)

// Queue defines the interface for a priority queue implementation.
// Different backends (miniredis, external Redis) implement this interface.
type Queue interface {
	// Enqueue adds an event to the queue for a given username with a priority score.
	Enqueue(ctx context.Context, username string, eventData string, priorityFactor float64) error

	// HealthCheck verifies the backend is reachable and functioning.
	HealthCheck(ctx context.Context) error

	// Close closes the queue and releases resources.
	Close() error
}
