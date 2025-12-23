package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/JensErat/lightfeather/internal/config"
	"github.com/JensErat/lightfeather/internal/metrics"
	"github.com/JensErat/lightfeather/internal/queue"
	"github.com/JensErat/lightfeather/internal/server"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// Load configuration
	cfg := config.Load()
	log.Printf("Starting lightfeather with config: HTTPAddr=%s, MetricsAddr=%s, RedisMode=%s, Namespace=%s\n",
		cfg.HTTPAddr, cfg.MetricsAddr, cfg.RedisMode, cfg.Namespace)

	// Initialize metrics with default prometheus registry
	m := metrics.New(prometheus.DefaultRegisterer)

	// Initialize queue
	var q queue.Queue
	var err error

	if cfg.RedisMode == "inmemory" {
		log.Println("Initializing in-memory Redis queue")
		q, err = queue.NewInMemoryQueue(cfg.Namespace)
		if err != nil {
			log.Fatalf("failed to create in-memory queue: %v\n", err)
		}
	} else {
		log.Fatalf("Redis mode %q not yet implemented\n", cfg.RedisMode)
	}

	defer func() {
		if err := q.Close(); err != nil {
			log.Printf("error closing queue: %v\n", err)
		}
	}()

	// Create HTTP server for events
	eventSrv := server.New(cfg.HTTPAddr, q, m)
	eventsHTTP := &http.Server{Addr: cfg.HTTPAddr, Handler: eventSrv.Handler()}

	// Create HTTP server for metrics with health and readiness probes
	var readyFlag uint32 // 0 = not ready, 1 = ready
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	metricsMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		// Liveness check: process is up
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	metricsMux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 1*time.Second)
		defer cancel()

		if atomic.LoadUint32(&readyFlag) == 0 {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		if err := q.HealthCheck(ctx); err != nil {
			http.Error(w, "queue not healthy", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})
	metricsHTTP := &http.Server{Addr: cfg.MetricsAddr, Handler: metricsMux}

	// Bind event listener before serving; mark ready only after bind success
	ln, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		log.Fatalf("failed to bind events listener on %s: %v\n", cfg.HTTPAddr, err)
	}

	// Start servers in goroutines
	done := make(chan struct{}, 2)
	go func() {
		log.Printf("Events HTTP server listening on %s\n", cfg.HTTPAddr)
		atomic.StoreUint32(&readyFlag, 1)
		if err := eventsHTTP.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("events server error: %v\n", err)
		}
		done <- struct{}{}
	}()

	go func() {
		log.Printf("Metrics HTTP server listening on %s\n", cfg.MetricsAddr)
		if err := metricsHTTP.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("metrics server error: %v\n", err)
		}
		done <- struct{}{}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	log.Printf("Shutdown signal received (%v), closing...\n", sig)

	// Graceful shutdown
	atomic.StoreUint32(&readyFlag, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := eventsHTTP.Shutdown(ctx); err != nil {
		log.Printf("error shutting down events server: %v\n", err)
	}
	if err := metricsHTTP.Shutdown(ctx); err != nil {
		log.Printf("error shutting down metrics server: %v\n", err)
	}

	// Wait for goroutines to exit or timeout
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}
