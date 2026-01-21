# dovewarden Implementation Summary

This document provides an overview of the dovewarden boilerplate application for processing Dovecot events.

## ✅ Completed Components

### 1. **HTTP Event Receiver** (`internal/server/http.go`)
- **POST /events**: Accepts JSON events from Dovecot
  - Returns `202 Accepted` if event passes filter and is enqueued
  - Returns `204 No Content` if event is filtered out
  - Returns `400 Bad Request` for malformed JSON
  - Returns `500 Internal Server Error` for enqueue failures
- **GET /metrics**: Prometheus metrics endpoint

### 2. **Event Filter** (`internal/events/`)
- `types.go`: Event data structures
  - `Event`: Raw event from Dovecot
  - `FilteredEvent`: Event that passed validation
- `filter.go`: Filtering logic
  - Only accepts `imap_command_finished` events
  - Validates `event` and `username` fields
  - Returns appropriate error types for failed validation

### 3. **Priority Queue** (`internal/queue/`)
- `queue.go`: Queue interface (Redis-compatible)
  - `Enqueue(ctx, username, eventData, priority)`: Add event to queue
  - `Close()`: Clean shutdown
- `redis.go`: In-memory implementation using miniredis
  - Uses Redis sorted sets (one per username key)
  - Current score: Unix timestamp (for FIFO ordering)
  - Priority parameter reserved for future use
  - `GetQueueSize()`: Metric helper for queue size per username

### 4. **Prometheus Instrumentation** (`internal/metrics/metrics.go`)
- **Counters**:
  - `dovewarden_events_received_total`: All received events
  - `dovewarden_events_filtered_total`: Events passing filter
  - `dovewarden_events_enqueued_total`: Successfully enqueued events
  - `dovewarden_enqueue_errors_total`: Enqueue failures
  - `dovewarden_redis_errors_total`: Redis operation errors
- **Gauges**:
  - `dovewarden_queue_size{username="..."}`: Queue size per user

### 5. **Configuration** (`internal/config/config.go`)
- Environment variables with CLI flag overrides:
  - `DOVEWARDEN_HTTP_ADDR`: HTTP listen address (default: `:8080`)
  - `DOVEWARDEN_REDIS_MODE`: Backend mode (default: `inmemory`)
  - `DOVEWARDEN_REDIS_ADDR`: External Redis address (for future use)
  - `DOVEWARDEN_NAMESPACE`: Queue key prefix (default: `dovewarden`)

### 6. **Application Main** (`cmd/dovewarden/main.go`)
- Config initialization
- Queue setup (currently inmemory only)
- HTTP server startup
- Graceful shutdown handling

## 📦 Dependencies

```
github.com/alicebob/miniredis/v2 v2.35.0  # In-memory Redis
github.com/redis/go-redis/v9 v9.17.2      # Redis client
github.com/prometheus/client_golang v1.20.5 # Prometheus instrumentation
```

## 🧪 Testing

### Manual Testing
```bash
# Start server
./dovewarden

# In another terminal:

# Test 1: Valid event
curl -X POST http://localhost:8080/events \
  -H "Content-Type: application/json" \
  -d '{"event":"imap_command_finished","username":"user@example.com"}'

# Test 2: Invalid event (wrong event type)
curl -X POST http://localhost:8080/events \
  -H "Content-Type: application/json" \
  -d '{"event":"imap_command_started","username":"user@example.com"}'

# Test 3: View metrics
curl http://localhost:8080/metrics | grep dovewarden
```

### Expected HTTP Status Codes
- `202 Accepted`: Event successfully enqueued
- `204 No Content`: Event filtered out (not matching criteria)
- `400 Bad Request`: Malformed JSON
- `500 Internal Server Error`: Server error

## 🔧 Architecture Decisions

### Event Filtering
- Only `imap_command_finished` events pass the filter
- Easily extendable: add event types to `AcceptedEvents` map
- Validation errors return `204 No Content` (not an error state)

### Queue Implementation
- **In-Memory (Default)**: miniredis for zero-dependency development
- **API-Compatible**: Can be replaced with external Redis without code changes
- **Per-User Keys**: Separate sorted set per username for independent queuing
- **Scoring**: Currently timestamp-based; can be extended per event reason

### Metrics
- Comprehensive counters for debugging and monitoring
- Per-username queue size gauge for capacity planning
- Error counters for queue operation issues

## 🚀 Future Extensions

### Phase 2: Queue Dequeuing
- Implement worker goroutines to consume events
- Add configurable concurrency levels
- Track processing status and errors

### Phase 3: Priority Enhancements
- Calculate priority based on event reason/command type
- Support dynamic priority adjustment
- Implement backpressure mechanisms

### Phase 4: External Redis
- Support external Redis deployments
- Add connection pooling and retry logic
- Implement Redis cluster support

### Phase 5: Advanced Features
- Event enrichment (add request IDs, durations, etc.)
- Rate limiting per username
- Dead-letter queue for failed events
- Event persistence and recovery

## 📝 Code Quality

The codebase follows Go best practices:
- Clear package organization
- Comprehensive error handling
- Metric-driven observability
- Configuration via environment variables
- Graceful shutdown

## 🏗️ Building and Running

```bash
# Build binary
go build -o dovewarden ./cmd/dovewarden

# Run with defaults (inmemory, :8080)
./dovewarden

# Run with custom address
./dovewarden --http-addr :9090

# Run with environment variables
DOVEWARDEN_HTTP_ADDR=:3000 ./dovewarden
```

## 📊 Monitoring Example

```bash
# Check received events
curl -s http://localhost:8080/metrics | grep dovewarden_events_received_total

# Check queue sizes
curl -s http://localhost:8080/metrics | grep dovewarden_queue_size

# Check error rates
curl -s http://localhost:8080/metrics | grep _errors_total
```

