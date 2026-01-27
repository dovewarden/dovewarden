package config

import (
	"flag"
	"os"
	"strconv"
	"time"
)

// Config holds application configuration.
type Config struct {
	HTTPAddr                       string
	MetricsAddr                    string
	RedisMode                      string // "inmemory" or "external"
	RedisAddr                      string
	Namespace                      string
	NumWorkers                     int
	DoveadmURL                     string
	DoveadmPassword                string
	DoveadmDest                    string // destination for dsync (e.g., "imap")
	LogLevel                       string
	BackgroundReplicationEnabled   bool
	BackgroundReplicationInterval  time.Duration
	BackgroundReplicationThreshold time.Duration
}

// Load reads configuration from environment and command-line flags.
func Load() *Config {
	cfg := &Config{
		HTTPAddr:                       ":8080",
		MetricsAddr:                    ":9090",
		RedisMode:                      "inmemory",
		RedisAddr:                      "localhost:6379",
		Namespace:                      "dovewarden",
		NumWorkers:                     4,
		DoveadmURL:                     "http://localhost:8080",
		DoveadmPassword:                "",
		DoveadmDest:                    "imap",
		LogLevel:                       "info",
		BackgroundReplicationEnabled:   true,
		BackgroundReplicationInterval:  time.Hour,
		BackgroundReplicationThreshold: 24 * time.Hour,
	}

	flag.StringVar(&cfg.HTTPAddr, "http-addr", envOrDefault("DOVEWARDEN_HTTP_ADDR", cfg.HTTPAddr), "HTTP server listen address for events")
	flag.StringVar(&cfg.MetricsAddr, "metrics-addr", envOrDefault("DOVEWARDEN_METRICS_ADDR", cfg.MetricsAddr), "HTTP server listen address for Prometheus metrics")
	flag.StringVar(&cfg.RedisMode, "redis-mode", envOrDefault("DOVEWARDEN_REDIS_MODE", cfg.RedisMode), "Redis mode: inmemory or external")
	flag.StringVar(&cfg.RedisAddr, "redis-addr", envOrDefault("DOVEWARDEN_REDIS_ADDR", cfg.RedisAddr), "Redis address for external mode")
	flag.StringVar(&cfg.Namespace, "namespace", envOrDefault("DOVEWARDEN_NAMESPACE", cfg.Namespace), "Key namespace prefix")
	flag.StringVar(&cfg.DoveadmURL, "doveadm-url", envOrDefault("DOVEWARDEN_DOVEADM_URL", cfg.DoveadmURL), "Doveadm API base URL")
	flag.StringVar(&cfg.DoveadmPassword, "doveadm-password", envOrDefault("DOVEWARDEN_DOVEADM_PASSWORD", cfg.DoveadmPassword), "Doveadm API password")
	flag.StringVar(&cfg.DoveadmDest, "doveadm-dest", envOrDefault("DOVEWARDEN_DOVEADM_DEST", cfg.DoveadmDest), "Doveadm dsync destination")
	flag.StringVar(&cfg.LogLevel, "log-level", envOrDefault("DOVEWARDEN_LOG_LEVEL", cfg.LogLevel), "Log level: debug, info, warn, error")

	// Parse NumWorkers from environment or flag
	numWorkersStr := envOrDefault("DOVEWARDEN_NUM_WORKERS", "4")
	if nw, err := strconv.Atoi(numWorkersStr); err == nil && nw > 0 {
		cfg.NumWorkers = nw
	}
	flag.IntVar(&cfg.NumWorkers, "num-workers", cfg.NumWorkers, "Number of worker goroutines for dequeuing")

	// Parse background replication settings
	backgroundReplicationEnabledStr := envOrDefault("DOVEWARDEN_BACKGROUND_REPLICATION_ENABLED", "true")
	cfg.BackgroundReplicationEnabled = backgroundReplicationEnabledStr == "true" || backgroundReplicationEnabledStr == "1"
	flag.BoolVar(&cfg.BackgroundReplicationEnabled, "background-replication-enabled", cfg.BackgroundReplicationEnabled, "Enable background replication")

	backgroundReplicationIntervalStr := envOrDefault("DOVEWARDEN_BACKGROUND_REPLICATION_INTERVAL", "1h")
	if interval, err := time.ParseDuration(backgroundReplicationIntervalStr); err == nil && interval > 0 {
		cfg.BackgroundReplicationInterval = interval
	}
	flag.DurationVar(&cfg.BackgroundReplicationInterval, "background-replication-interval", cfg.BackgroundReplicationInterval, "Background replication interval")

	backgroundReplicationThresholdStr := envOrDefault("DOVEWARDEN_BACKGROUND_REPLICATION_THRESHOLD", "24h")
	if threshold, err := time.ParseDuration(backgroundReplicationThresholdStr); err == nil && threshold > 0 {
		cfg.BackgroundReplicationThreshold = threshold
	}
	flag.DurationVar(&cfg.BackgroundReplicationThreshold, "background-replication-threshold", cfg.BackgroundReplicationThreshold, "Background replication threshold - users replicated within this time are skipped")

	flag.Parse()

	return cfg
}

func envOrDefault(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}
