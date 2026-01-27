package queue

import (
	"context"
	"log/slog"
	"time"

	"github.com/dovewarden/dovewarden/internal/doveadm"
)

// DoveadmEventHandler handles events by sending dsync requests to Doveadm
type DoveadmEventHandler struct {
	client      *doveadm.Client
	destination string
	logger      *slog.Logger
	queue       Queue
}

// NewDoveadmEventHandler creates a new handler for Doveadm sync operations
func NewDoveadmEventHandler(baseURL, password, destination string, logger *slog.Logger, queue Queue) *DoveadmEventHandler {
	return &DoveadmEventHandler{
		client:      doveadm.NewClient(baseURL, password),
		destination: destination,
		logger:      logger,
		queue:       queue,
	}
}

// Handle sends a dsync request to Doveadm for the given username
func (h *DoveadmEventHandler) Handle(ctx context.Context, username string) error {
	// Retrieve the last known replication state for this user
	state, err := h.queue.GetReplicationState(ctx, username)
	if err != nil {
		h.logger.Warn("Failed to get replication state, proceeding without state", "username", username, "error", err)
		state = ""
	}

	h.logger.Info("Syncing user via dsync", "username", username, "destination", h.destination, "has_state", state != "")

	resp, err := h.client.Sync(ctx, username, h.destination, state)
	if err != nil {
		h.logger.Error("dsync failed", "username", username, "error", err)
		return err
	}

	// Store the new replication state for next sync
	if resp.State != "" {
		if err := h.queue.SetReplicationState(ctx, username, resp.State); err != nil {
			h.logger.Warn("Failed to store replication state", "username", username, "error", err)
			// Don't fail the sync operation if state storage fails
		} else {
			h.logger.Debug("Stored replication state", "username", username)
		}
	}

	// Record the timestamp of this successful replication
	if err := h.queue.SetLastReplicationTime(ctx, username, time.Now()); err != nil {
		h.logger.Warn("Failed to store last replication time", "username", username, "error", err)
		// Don't fail the sync operation if timestamp storage fails
	}

	h.logger.Info("dsync completed", "username", username)
	return nil
}
