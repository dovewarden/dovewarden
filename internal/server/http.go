package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/JensErat/lightfeather/internal/events"
	"github.com/JensErat/lightfeather/internal/metrics"
	"github.com/JensErat/lightfeather/internal/queue"
)

// Server handles HTTP requests for the Dovecot event API.
type Server struct {
	addr    string
	queue   queue.Queue
	metrics *metrics.Metrics
	mux     *http.ServeMux
}

// New creates a new HTTP server.
func New(addr string, q queue.Queue, m *metrics.Metrics) *Server {
	s := &Server{
		addr:    addr,
		queue:   q,
		metrics: m,
		mux:     http.NewServeMux(),
	}

	s.mux.HandleFunc("POST /events", s.handleEvents)

	return s
}

// handleEvents processes incoming Dovecot events.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	s.metrics.EventsReceived.Inc()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("failed to read request body", "error", err)
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Filter the event
	filtered, err := events.Filter(body)
	if err != nil {
		slog.Warn("event ignored", "reason", err.Error(), "body", string(body))
		w.WriteHeader(http.StatusNoContent)
		return
	}

	s.metrics.EventsFiltered.Inc()

	slog.Info("event accepted", "username", filtered.Username, "cmd", filtered.CmdName, "event_type", filtered.Event)

	// Enqueue the event with static priority
	eventJSON, _ := json.Marshal(filtered.Raw)
	staticPriority := 1.0 // Static priority for now; will be extended per event type later

	if err := s.queue.Enqueue(r.Context(), filtered.Username, string(eventJSON), staticPriority); err != nil {
		slog.Error("failed to enqueue event", "username", filtered.Username, "error", err)
		s.metrics.EnqueueErrors.Inc()
		http.Error(w, "failed to enqueue event", http.StatusInternalServerError)
		return
	}

	s.metrics.EventsEnqueued.Inc()

	w.WriteHeader(http.StatusAccepted)
}

// Start starts the HTTP server (blocking).
func (s *Server) Start() error {
	return http.ListenAndServe(s.addr, s.mux)
}

// Handler returns the HTTP handler for use with custom servers (e.g., for testing).
func (s *Server) Handler() http.Handler {
	return s.mux
}
