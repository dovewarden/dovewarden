package queue

import (
	"context"
	"log/slog"

	"github.com/dovewarden/dovewarden/internal/doveadm"
)

// DoveadmEventHandler handles events by sending dsync requests to Doveadm
type DoveadmEventHandler struct {
	client      *doveadm.Client
	destination string
	logger      *slog.Logger
}

// NewDoveadmEventHandler creates a new handler for Doveadm sync operations
func NewDoveadmEventHandler(baseURL, username, password, destination string, logger *slog.Logger) *DoveadmEventHandler {
	return &DoveadmEventHandler{
		client:      doveadm.NewClient(baseURL, username, password),
		destination: destination,
		logger:      logger,
	}
}

// Handle sends a dsync request to Doveadm for the given username
func (h *DoveadmEventHandler) Handle(ctx context.Context, username string) error {
	h.logger.Info("Syncing user via dsync", "username", username, "destination", h.destination)

	err := h.client.Sync(ctx, username, h.destination)
	if err != nil {
		h.logger.Error("dsync failed", "username", username, "error", err)
		return err
	}

	h.logger.Info("dsync completed", "username", username)
	return nil
}
