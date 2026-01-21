package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds all Prometheus metrics for the application.
type Metrics struct {
	EventsReceived prometheus.Counter
	EventsFiltered prometheus.Counter
	EventsEnqueued prometheus.Counter
	EnqueueErrors  prometheus.Counter
	RedisErrors    prometheus.Counter
}

// New creates and registers all metrics.
func New(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		EventsReceived: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "dovewarden_events_received_total",
				Help: "Total number of events received from Dovecot",
			},
		),
		EventsFiltered: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "dovewarden_events_filtered_total",
				Help: "Total number of events that passed the filter",
			},
		),
		EventsEnqueued: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "dovewarden_events_enqueued_total",
				Help: "Total number of events successfully enqueued",
			},
		),
		EnqueueErrors: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "dovewarden_enqueue_errors_total",
				Help: "Total number of enqueue errors",
			},
		),
		RedisErrors: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "dovewarden_redis_errors_total",
				Help: "Total number of Redis operation errors",
			},
		),
	}

	reg.MustRegister(
		m.EventsReceived,
		m.EventsFiltered,
		m.EventsEnqueued,
		m.EnqueueErrors,
		m.RedisErrors,
	)

	return m
}
