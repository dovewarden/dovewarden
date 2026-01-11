package config

import (
	"flag"
	"os"
	"strconv"
)

// Config holds application configuration.
type Config struct {
	HTTPAddr    string
	MetricsAddr string
	RedisMode   string // "inmemory" or "external"
	RedisAddr   string
	Namespace   string
	NumWorkers  int
}

// Load reads configuration from environment and command-line flags.
func Load() *Config {
	cfg := &Config{
		HTTPAddr:    ":8080",
		MetricsAddr: ":9090",
		RedisMode:   "inmemory",
		RedisAddr:   "localhost:6379",
		Namespace:   "lf",
		NumWorkers:  4,
	}

	flag.StringVar(&cfg.HTTPAddr, "http-addr", envOrDefault("LF_HTTP_ADDR", cfg.HTTPAddr), "HTTP server listen address for events")
	flag.StringVar(&cfg.MetricsAddr, "metrics-addr", envOrDefault("LF_METRICS_ADDR", cfg.MetricsAddr), "HTTP server listen address for Prometheus metrics")
	flag.StringVar(&cfg.RedisMode, "redis-mode", envOrDefault("LF_REDIS_MODE", cfg.RedisMode), "Redis mode: inmemory or external")
	flag.StringVar(&cfg.RedisAddr, "redis-addr", envOrDefault("LF_REDIS_ADDR", cfg.RedisAddr), "Redis address for external mode")
	flag.StringVar(&cfg.Namespace, "namespace", envOrDefault("LF_NAMESPACE", cfg.Namespace), "Key namespace prefix")

	// Parse NumWorkers from environment or flag
	numWorkersStr := envOrDefault("LF_NUM_WORKERS", "4")
	if nw, err := strconv.Atoi(numWorkersStr); err == nil && nw > 0 {
		cfg.NumWorkers = nw
	}
	flag.IntVar(&cfg.NumWorkers, "num-workers", cfg.NumWorkers, "Number of worker goroutines for dequeuing")

	flag.Parse()

	return cfg
}

func envOrDefault(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}
