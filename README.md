# dovewarden

A lightweight event processor for Dovecot's event API. Receives IMAP command events, filters them, and enqueues them in a priority queue for asynchronous processing.

## Features

- **HTTP Event Receiver**: Accepts JSON events from Dovecot's event API
- **Event Filtering**: Passes only `imap_command_finished` events
- **Priority Queue**: Events stored in a Redis-compatible sorted set per username
- **Background Replication**: Periodic replication of all users to cover missed events
- **In-Memory Development Mode**: Uses miniredis for zero-dependency local development
- **Prometheus Metrics**: Full instrumentation for monitoring
- **Separate ports for events and metrics**
- **Health/Readiness Probes**: `/healthz` and `/readyz` on the metrics port
- **Graceful Shutdown**: Drains HTTP servers with timeouts

## Architecture

### Event Flow

```
Dovecot Event API
       ↓
   POST /events  (default :8080)
       ↓
   Filter (imap_command_finished only)
       ↓
   Priority Queue (Redis sorted set per username)
       ↓
   Metrics update (scraped from :9090/metrics)
```

### Components

- **Config** (`internal/config`): Configuration from environment variables or CLI flags
- **Events** (`internal/events`): Event parsing and filtering logic
- **Queue** (`internal/queue`): Redis-compatible queue abstraction with miniredis backend
- **Metrics** (`internal/metrics`): Prometheus instrumentation
- **Server** (`internal/server`): HTTP endpoint for events

## Configuration

Environment variables (with CLI flag overrides):

- `DOVEWARDEN_HTTP_ADDR` (`--http-addr`): HTTP server listen address for events (default: `:8080`)
- `DOVEWARDEN_METRICS_ADDR` (`--metrics-addr`): HTTP server listen address for Prometheus metrics (default: `:9090`)
- `DOVEWARDEN_REDIS_MODE` (`--redis-mode`): Redis mode: `inmemory` or `external` (default: `inmemory`)
- `DOVEWARDEN_REDIS_ADDR` (`--redis-addr`): Redis server address for external mode (default: `localhost:6379`)
- `DOVEWARDEN_NAMESPACE` (`--namespace`): Key namespace prefix for queue keys (default: `dovewarden`)
- `DOVEWARDEN_NUM_WORKERS` (`--num-workers`): Number of worker goroutines for dequeuing (default: `4`)
- `DOVEWARDEN_DOVEADM_URL` (`--doveadm-url`): Doveadm API base URL (default: `http://localhost:8080`)
- `DOVEWARDEN_DOVEADM_PASSWORD` (`--doveadm-password`): Doveadm API password (required)
- `DOVEWARDEN_DOVEADM_DEST` (`--doveadm-dest`): Doveadm dsync destination (default: `imap`)
- `DOVEWARDEN_LOG_LEVEL` (`--log-level`): Log level: debug, info, warn, error (default: `info`)
- `DOVEWARDEN_BACKGROUND_REPLICATION_ENABLED` (`--background-replication-enabled`): Enable background replication (default: `true`)
- `DOVEWARDEN_BACKGROUND_REPLICATION_INTERVAL` (`--background-replication-interval`): Background replication interval (default: `1h`)
- `DOVEWARDEN_BACKGROUND_REPLICATION_THRESHOLD` (`--background-replication-threshold`): Skip users replicated within this time (default: `24h`)

### Background Replication

Background replication periodically lists all users from the Doveadm API and enqueues them for replication if they haven't been replicated within the configured threshold. This ensures that users who haven't triggered any IMAP events are still regularly replicated.

The background replication service:
- Runs once immediately on startup
- Then runs periodically based on the configured interval (default: 1 hour)
- Tracks the last replication time for each user in Redis
- Skips users who were replicated within the threshold period (default: 24 hours)
- Can be disabled by setting `DOVEWARDEN_BACKGROUND_REPLICATION_ENABLED=false`

## API Endpoints

- Events server (default `:8080`)
  - POST `/events`
    - `202 Accepted`: Event successfully enqueued
    - `204 No Content`: Event filtered out (not matching criteria)
    - `400 Bad Request`: Malformed JSON or missing required fields
    - `500 Internal Server Error`: Enqueue or queue operation failed

- Metrics server (default `:9090`)
  - GET `/metrics` (Prometheus text format)
  - GET `/healthz` (liveness)
    - Always returns `200 OK` when the process is running
  - GET `/readyz` (readiness)
    - Returns `503 Service Unavailable` until the events listener is bound
    - Returns `503` if the queue backend health check fails
    - Returns `200 OK` when ready and healthy

## Dovecot Event Payload

Events must be sent as JSON POST requests to `/events`:

```json
{
  "event": "imap_command_finished",
  "username": "user@example.com",
  "timestamp": "2024-12-20T10:30:45Z"
}
```

Only events with `event: "imap_command_finished"` are accepted. The `username` field is used as the queue key.

## Prometheus Metrics

- `dovewarden_events_received_total`: Counter of all received events
- `dovewarden_events_filtered_total`: Counter of events passing the filter
- `dovewarden_events_enqueued_total`: Counter of events successfully enqueued
- `dovewarden_enqueue_errors_total`: Counter of enqueue failures
- `dovewarden_queue_size{username="..."}`: Current queue size per username
- `dovewarden_redis_errors_total`: Counter of Redis operation errors

## Development & Local Testing

### Run Locally (In-Memory Mode)

```bash
go build -o dovewarden ./cmd/dovewarden
./dovewarden --http-addr :8080 --metrics-addr :9090
```

### Test Event Submission (events port)

```bash
curl -X POST http://localhost:8080/events \
  -H "Content-Type: application/json" \
  -d '{"event":"imap_command_finished","username":"testuser","timestamp":"2024-12-20T10:30:45Z"}'
```

### View Metrics and Probes (metrics port)

```bash
curl http://localhost:9090/metrics
curl -i http://localhost:9090/healthz
curl -i http://localhost:9090/readyz
```

## Graceful Shutdown

The application listens for SIGINT/SIGTERM and gracefully shuts down both the events and metrics HTTP servers with a 5-second timeout.

## Future Extensions

- **Priority by Event Reason**: Extend priority calculation based on event reason/command type
- **External Redis Support**: Implement external Redis backend for production deployments
- **Dequeue Operations**: Implement queue draining/worker logic in next phase
- **Event Enrichment**: Add additional context to queued events (request IDs, durations, etc.)
- **Rate Limiting**: Add rate limiting per username to prevent queue saturation
