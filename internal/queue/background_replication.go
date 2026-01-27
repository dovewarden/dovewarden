package queue

import (
	"context"
	"log/slog"
	"time"

	"github.com/dovewarden/dovewarden/internal/doveadm"
)

// BackgroundReplicationService manages periodic background replication
type BackgroundReplicationService struct {
	client    *doveadm.Client
	queue     Queue
	logger    *slog.Logger
	interval  time.Duration
	threshold time.Duration
	stopCh    chan struct{}
	doneCh    chan struct{}
}

// NewBackgroundReplicationService creates a new background replication service
func NewBackgroundReplicationService(
	client *doveadm.Client,
	queue Queue,
	logger *slog.Logger,
	interval time.Duration,
	threshold time.Duration,
) *BackgroundReplicationService {
	return &BackgroundReplicationService{
		client:    client,
		queue:     queue,
		logger:    logger,
		interval:  interval,
		threshold: threshold,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
}

// Start begins the background replication service
// It runs once immediately and then periodically based on the configured interval
func (s *BackgroundReplicationService) Start(ctx context.Context) {
	s.logger.Info("Starting background replication service",
		"interval", s.interval,
		"threshold", s.threshold,
	)

	go func() {
		defer close(s.doneCh)

		// Run once immediately on startup
		s.logger.Info("Running initial background replication")
		if err := s.runReplication(ctx); err != nil {
			s.logger.Error("Initial background replication failed", "error", err)
		}

		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-s.stopCh:
				s.logger.Info("Background replication service stopping")
				return
			case <-ticker.C:
				s.logger.Info("Running periodic background replication")
				if err := s.runReplication(ctx); err != nil {
					s.logger.Error("Background replication failed", "error", err)
				}
			}
		}
	}()
}

// Stop gracefully stops the background replication service
func (s *BackgroundReplicationService) Stop(ctx context.Context) error {
	s.logger.Info("Stopping background replication service")
	close(s.stopCh)

	// Wait for the service to stop or context to expire
	select {
	case <-s.doneCh:
		s.logger.Info("Background replication service stopped")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// runReplication lists all users and enqueues those that need replication
func (s *BackgroundReplicationService) runReplication(ctx context.Context) error {
	startTime := time.Now()
	s.logger.Debug("Listing users from doveadm API")

	// List all users from doveadm
	users, err := s.client.ListUsers(ctx)
	if err != nil {
		return err
	}

	s.logger.Info("Retrieved user list from doveadm", "count", len(users))

	// Track statistics
	var enqueuedCount, skippedCount, errorCount int

	// Process each user
	for _, user := range users {
		// Check if this user was replicated recently
		lastReplication, err := s.queue.GetLastReplicationTime(ctx, user.Username)
		if err != nil {
			s.logger.Warn("Failed to get last replication time, will enqueue user",
				"username", user.Username,
				"error", err,
			)
			errorCount++
			// Continue to enqueue in case of error
		}

		// Skip if user was replicated within the threshold
		if !lastReplication.IsZero() && time.Since(lastReplication) < s.threshold {
			s.logger.Debug("Skipping user - recently replicated",
				"username", user.Username,
				"last_replication", lastReplication,
				"age", time.Since(lastReplication),
			)
			skippedCount++
			continue
		}

		// Enqueue user for replication with normal priority
		if err := s.queue.Enqueue(ctx, user.Username, 1.0); err != nil {
			s.logger.Error("Failed to enqueue user for background replication",
				"username", user.Username,
				"error", err,
			)
			errorCount++
			continue
		}

		s.logger.Debug("Enqueued user for background replication",
			"username", user.Username,
			"last_replication", lastReplication,
		)
		enqueuedCount++
	}

	duration := time.Since(startTime)
	s.logger.Info("Background replication completed",
		"duration", duration,
		"total_users", len(users),
		"enqueued", enqueuedCount,
		"skipped", skippedCount,
		"errors", errorCount,
	)

	return nil
}
