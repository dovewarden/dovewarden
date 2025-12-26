# Structured Logging

The application uses Go's standard library `slog` (structured logging) for all log output. This provides:

- **Flexible output formats**: Both readable text and JSON output
- **Structured data**: Logs include contextual key-value pairs
- **Built-in source location**: File and line number information automatically included

## Configuration

### Log Format

Control the log output format using the `LOG_FORMAT` environment variable:

```bash
# Text output (default, human-readable)
./lightfeather
# or explicitly:
LOG_FORMAT=text ./lightfeather

# JSON output (machine-readable, suitable for log aggregation)
LOG_FORMAT=json ./lightfeather
```

### Log Level

The default log level is `Info`. To change it, modify the `opts.Level` in `cmd/lightfeather/main.go`.

## Examples

### Text Output
```
time=2024-12-26T20:40:02.123Z level=INFO source=main.go:48 msg="Starting lightfeather" http_addr=:8080 metrics_addr=:8081 redis_mode=inmemory namespace=lightfeather
time=2024-12-26T20:40:02.124Z level=INFO source=main.go:61 msg="Initializing in-memory Redis queue"
time=2024-12-26T20:40:02.125Z level=INFO source=http.go:58 msg="event accepted" username=user-a cmd=APPEND event_type=imap_command_finished
time=2024-12-26T20:40:02.126Z level=WARN source=http.go:51 msg="event ignored" reason="cmd_name not accepted by filter"
```

### JSON Output
```json
{"time":"2024-12-26T20:40:02.123Z","level":"INFO","source":"main.go:48","msg":"Starting lightfeather","http_addr":":8080","metrics_addr":":8081","redis_mode":"inmemory","namespace":"lightfeather"}
{"time":"2024-12-26T20:40:02.124Z","level":"INFO","source":"main.go:61","msg":"Initializing in-memory Redis queue"}
{"time":"2024-12-26T20:40:02.125Z","level":"INFO","source":"http.go:58","msg":"event accepted","username":"user-a","cmd":"APPEND","event_type":"imap_command_finished"}
{"time":"2024-12-26T20:40:02.126Z","level":"WARN","source":"http.go:51","msg":"event ignored","reason":"cmd_name not accepted by filter"}
```

## HTTP Event Handler Logs

The HTTP event handler logs the following:

### Accepted Events (INFO level)
When an event passes the filter and is successfully enqueued:
```
level=INFO msg="event accepted" username=<username> cmd=<command> event_type=<event_type>
```

### Ignored Events (WARN level)
When an event fails the filter (e.g., wrong command type):
```
level=WARN msg="event ignored" reason=<filter_error>
```

These should be filtered in Dovecot's event configuration to reduce unnecessary log entries.

## Best Practices

- Use structured logging with key-value pairs for better searchability
- Use appropriate log levels: `Error` for errors, `Warn` for warnings, `Info` for important information
- Include contextual information like usernames, command names, and error details
- For JSON output, pipe logs to a log aggregation service (ELK, Datadog, etc.)

