package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/dovewarden/dovewarden/internal/config"
	"github.com/dovewarden/dovewarden/internal/doveadm"
	"github.com/dovewarden/dovewarden/internal/metrics"
	"github.com/dovewarden/dovewarden/internal/queue"
	"github.com/dovewarden/dovewarden/internal/server"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	version     = "0.0.0-dev" // Set by ldflags during build
	showVersion bool
)

func init() {
	flag.BoolVar(&showVersion, "version", false, "Show version and exit")
	flag.Parse()
}

func main() {
	if showVersion {
		fmt.Printf("dovewarden version %s\n", version)
		os.Exit(0)
	}

	// Load configuration early so we can configure logging
	cfg := config.Load()

	// Initialize structured logging
	// LOG_FORMAT environment variable controls output: "json" or "text" (default)
	logFormat := strings.ToLower(os.Getenv("LOG_FORMAT"))
	var logger *slog.Logger

	lvl := parseLogLevel(cfg.LogLevel)
	opts := &slog.HandlerOptions{
		AddSource: true,
		Level:     lvl,
	}

	if logFormat == "json" {
		handler := slog.NewJSONHandler(os.Stdout, opts)
		logger = slog.New(handler)
	} else {
		handler := slog.NewTextHandler(os.Stdout, opts)
		logger = slog.New(handler)
	}

	slog.SetDefault(logger)

	// Log version information
	slog.Info("dovewarden starting", "version", version, "log_level", lvl.String())

	slog.Info("Starting dovewarden",
		"http_addr", cfg.HTTPAddr,
		"metrics_addr", cfg.MetricsAddr,
		"redis_mode", cfg.RedisMode,
		"namespace", cfg.Namespace,
		"num_workers", cfg.NumWorkers,
		"doveadm_url", cfg.DoveadmURL,
	)

	// Initialize metrics with default prometheus registry
	m := metrics.New(prometheus.DefaultRegisterer)

	// Initialize queue
	var q queue.Queue
	var err error

	if cfg.RedisMode == "inmemory" {
		slog.Info("Initializing in-memory Redis queue")
		q, err = queue.NewInMemoryQueue(cfg.Namespace, cfg.RedisAddr, logger)
		if err != nil {
			slog.Error("failed to create in-memory queue", "error", err)
			os.Exit(1)
		}
	} else {
		slog.Error("Redis mode not yet implemented", "mode", cfg.RedisMode)
		os.Exit(1)
	}

	defer func() {
		if err := q.Close(); err != nil {
			slog.Error("error closing queue", "error", err)
		}
	}()

	// Initialize worker pool for dequeuing
	slog.Info("Initializing worker pool", "num_workers", cfg.NumWorkers)
	workerPool := queue.NewWorkerPool(q, cfg.NumWorkers, logger)

	// Set up Doveadm event handler if credentials are provided
	if cfg.DoveadmPassword == "" {
		slog.Error("Doveadm password not provided; exiting")
		os.Exit(1)
	}
	slog.Info("Setting up Doveadm sync handler")
	handler := queue.NewDoveadmEventHandler(cfg.DoveadmURL, cfg.DoveadmPassword, cfg.DoveadmDest, logger, q)
	workerPool.SetHandler(handler)

	workerPool.Start(context.Background())

	// Initialize background replication service if enabled
	var backgroundReplicationService *queue.BackgroundReplicationService
	if cfg.BackgroundReplicationEnabled {
		slog.Info("Initializing background replication service",
			"enabled", cfg.BackgroundReplicationEnabled,
			"interval", cfg.BackgroundReplicationInterval,
			"threshold", cfg.BackgroundReplicationThreshold,
		)
		doveadmClient := doveadm.NewClient(cfg.DoveadmURL, cfg.DoveadmPassword)
		backgroundReplicationService = queue.NewBackgroundReplicationService(
			doveadmClient,
			q,
			logger,
			cfg.BackgroundReplicationInterval,
			cfg.BackgroundReplicationThreshold,
		)
		backgroundReplicationService.Start(context.Background())
	} else {
		slog.Info("Background replication disabled")
	}

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
		slog.Error("failed to bind events listener", "addr", cfg.HTTPAddr, "error", err)
		os.Exit(1)
	}

	// Start servers in goroutines
	done := make(chan struct{}, 2)
	go func() {
		slog.Info("Events HTTP server listening", "addr", cfg.HTTPAddr)
		atomic.StoreUint32(&readyFlag, 1)
		if err := eventsHTTP.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("events server error", "error", err)
		}
		done <- struct{}{}
	}()

	go func() {
		slog.Info("Metrics HTTP server listening", "addr", cfg.MetricsAddr)
		if err := metricsHTTP.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("metrics server error", "error", err)
		}
		done <- struct{}{}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	slog.Info("Shutdown signal received", "signal", sig.String())

	// Graceful shutdown
	atomic.StoreUint32(&readyFlag, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Stop background replication service first if enabled
	if cfg.BackgroundReplicationEnabled && backgroundReplicationService != nil {
		if err := backgroundReplicationService.Stop(ctx); err != nil {
			slog.Error("error stopping background replication service", "error", err)
		}
	}

	// Stop worker pool (gracefully)
	if err := workerPool.Stop(ctx); err != nil {
		slog.Error("error stopping worker pool", "error", err)
	}

	if err := eventsHTTP.Shutdown(ctx); err != nil {
		slog.Error("error shutting down events server", "error", err)
	}
	if err := metricsHTTP.Shutdown(ctx); err != nil {
		slog.Error("error shutting down metrics server", "error", err)
	}

	// Wait for goroutines to exit or timeout
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

// parseLogLevel converts a string log level to slog.Level, defaulting to info on unknown values.
func parseLogLevel(lvl string) slog.Level {
	switch strings.ToLower(lvl) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error", "err":
		return slog.LevelError
	default:
		fmt.Fprintf(os.Stderr, "unknown log level %q, defaulting to info\n", lvl)
		return slog.LevelInfo
	}
}
