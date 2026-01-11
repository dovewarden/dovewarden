# Worker Pool Implementation Summary

## Overview
Implemented a configurable worker pool for dequeuing and processing events from the Redis queue with graceful shutdown support.

## Components Implemented

### 1. Queue Interface Enhancement
- Added `Dequeue(ctx context.Context) (string, error)` method to Queue interface
- Returns the username with the lowest priority score (FIFO with priority support)

### 2. InMemoryQueue Dequeue Implementation
- Implemented `Dequeue()` using Redis ZPOPMIN command
- Returns empty string if queue is empty (non-blocking)
- Returns error on backend failures

### 3. Priority Factor Implementation
- Enhanced `Enqueue()` to apply priority factor to score calculation
- Formula: `score = timestamp / priorityFactor`
- **factor = 1.0**: Normal priority (timestamp as score)
- **factor > 1.0**: Higher priority (lower score = processed sooner)
- **factor < 1.0**: Lower priority (higher score = processed later)
- Safety check: prevents division by zero

### 4. EventHandler Interface
- Defined `EventHandler` interface for flexible event processing
- `Handle(ctx context.Context, username string) error`
- Allows custom implementations (currently using DefaultEventHandler)

### 5. WorkerPool Implementation
File: `internal/queue/worker.go`

#### Features:
- **Configurable number of workers**: Set via `LF_NUM_WORKERS` env var or `-num-workers` flag (default: 4)
- **Worker coordination**: Uses WaitGroup and context for clean shutdown
- **Error handling**: Failed events are automatically requeued
- **Graceful shutdown**: Stops accepting new tasks but waits for active tasks to complete
- **Timeout protection**: 1-second dequeue timeout prevents workers from getting stuck
- **Active task tracking**: Monitors number of workers currently processing

#### Worker Loop Logic:
1. Check for stop signal
2. Dequeue with 1-second timeout
3. If queue empty, wait 500ms before retry
4. Mark worker as active
5. Call handler (currently logs username)
6. On error: requeue event with factor 1.0
7. Mark worker as inactive
8. Repeat

### 6. Configuration
File: `internal/config/config.go`

New fields:
- `NumWorkers int` (default: 4)
- Can be set via:
  - Environment variable: `LF_NUM_WORKERS=8`
  - Command-line flag: `--num-workers=8`

### 7. Main Application Integration
File: `cmd/lightfeather/main.go`

Changes:
- Initialize WorkerPool after queue creation
- Start worker pool in background
- Graceful shutdown: stop worker pool before closing queue
- Worker pool shutdown has 5-second timeout

## Test Coverage

### Priority Tests (`internal/queue/redis_test.go`)
✅ `TestPriorityOrderByInsertion`: Earlier enqueued users processed first (same factor)
✅ `TestPriorityFactorGreaterThanOne`: Factor > 1 = higher priority (lower score)
✅ `TestPriorityFactorLessThanOne`: Factor < 1 = lower priority (higher score)

### Worker Pool Tests (`internal/queue/worker_test.go`)
✅ `TestWorkerPoolDequeue`: Workers successfully dequeue and process events
✅ `TestWorkerPoolRequeueOnError`: Failed events are requeued and retried
✅ `TestGracefulShutdown`: Shutdown waits for active tasks, completes within timeout

## Usage Example

```bash
# Start with 8 workers on custom Redis port
export LF_NUM_WORKERS=8
export LF_REDIS_ADDR=127.0.0.1:6380
./lightfeather

# Or with command-line flags
./lightfeather --num-workers=8 --redis-addr=127.0.0.1:6380
```

## Shutdown Behavior

1. Application receives SIGINT/SIGTERM
2. Worker pool is signaled to stop accepting new tasks
3. Workers finish processing active events
4. All workers exit gracefully (max 5-second timeout)
5. Queue is closed
6. HTTP servers shut down

## Logging

Worker pool events are logged with context:
- INFO: Worker pool started/stopped
- DEBUG: Worker processing events
- ERROR: Handler failures and requeue attempts

Example output:
```
time=2026-01-10T23:43:45.633+01:00 level=INFO msg="Worker pool started" num_workers=4
time=2026-01-10T23:43:45.634+01:00 level=INFO msg="Handling event" username=user-a
time=2026-01-10T23:43:46.634+01:00 level=INFO msg="Stopping worker pool"
```

## Future Enhancements

1. **Configurable dequeue timeout**: Make 1-second timeout configurable
2. **Dead letter queue**: Move repeatedly failing events to DLQ
3. **Metrics**: Add Prometheus metrics for worker utilization, processing times
4. **Backoff strategy**: Implement exponential backoff for failing events
5. **Handler configuration**: Make DefaultEventHandler pluggable at startup
6. **Context timeouts**: Make worker processing timeout configurable

