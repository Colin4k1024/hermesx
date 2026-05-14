# Observability

> Monitoring, tracing, logging, and operational capabilities for the Hermes SaaS API.

## Prometheus Metrics

Hermes exposes Prometheus-format metrics via the `GET /metrics` endpoint, with no authentication required.

### HTTP Metrics

| Metric Name | Type | Labels | Description |
|-------------|------|--------|-------------|
| `hermes_http_requests_total` | Counter | `method`, `path`, `status`, `tenant_id` | Total HTTP requests |
| `hermes_http_request_duration_seconds` | Histogram | `method`, `path`, `tenant_id` | Request latency distribution |
| `hermes_http_requests_in_flight` | Gauge | none | Current number of concurrent requests being processed |

**Features**:
- All metrics are segmented by `tenant_id` dimension; unauthenticated requests are labeled as `anonymous`
- Paths are automatically normalized (truncated to >64 characters) to reduce metric cardinality
- Histogram uses Prometheus default buckets (`.005`, `.01`, `.025`, `.05`, `.1`, `.25`, `.5`, `1`, `2.5`, `5`, `10`)

### Scrape Configuration

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'hermes'
    scrape_interval: 15s
    static_configs:
      - targets: ['hermes-api:8080']
    metrics_path: /metrics
```

### Recommended Alert Rules

```yaml
groups:
  - name: hermes
    rules:
      - alert: HighErrorRate
        expr: rate(hermes_http_requests_total{status=~"5.."}[5m]) > 0.1
        for: 5m
        labels:
          severity: warning

      - alert: HighLatency
        expr: histogram_quantile(0.99, rate(hermes_http_request_duration_seconds_bucket[5m])) > 5
        for: 5m
        labels:
          severity: warning

      - alert: TenantRateLimited
        expr: rate(hermes_http_requests_total{status="429"}[5m]) > 0
        for: 1m
        labels:
          severity: info
```

## OpenTelemetry Tracing

Hermes supports OpenTelemetry distributed tracing, exporting spans via the OTLP gRPC protocol.

### Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | none (tracing disabled) | OTLP gRPC endpoint address |
| `OTEL_EXPORTER_OTLP_INSECURE` | `false` | Whether to use an insecure connection |
| `OTEL_SERVICE_NAME` | `hermesx` | Service name |

**When `OTEL_EXPORTER_OTLP_ENDPOINT` is not set, tracing is completely disabled with zero overhead.**

### Enabling Tracing

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT="localhost:4317"
export OTEL_EXPORTER_OTLP_INSECURE="true"
export OTEL_SERVICE_NAME="hermesx"
./hermesx saas-api
```

### Tracing Features

- **W3C Trace Context Propagation**: Supports `traceparent` / `tracestate` request headers
- **Baggage Propagation**: Supports `baggage` request headers for context passing
- **Batch Exporter**: Asynchronous batch export of spans without blocking request processing
- **pgx Tracer**: PostgreSQL queries automatically produce child spans

### Integration with Jaeger

```yaml
# Append to docker-compose.dev.yml
services:
  jaeger:
    image: jaegertracing/all-in-one:latest
    ports:
      - "4317:4317"   # OTLP gRPC
      - "16686:16686" # Jaeger UI
    environment:
      COLLECTOR_OTLP_ENABLED: "true"
```

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT="localhost:4317"
export OTEL_EXPORTER_OTLP_INSECURE="true"
```

Access `http://localhost:16686` to view trace data.

## Structured Logging

Hermes uses Go's standard `log/slog` library for structured logging.

### Context-Enriched Logger

Logs automatically extract the following fields from Context:

| Field | Source | Description |
|-------|--------|-------------|
| `request_id` | RequestID middleware | Unique request identifier |
| `tenant_id` | Tenant middleware | Tenant ID |
| `session_id` | Handler | Session ID (where applicable) |
| `trace_id` | OTel | Distributed trace ID |

**How it works**:
1. RequestID middleware generates `request_id` and writes it to Context
2. Auth + Tenant middleware extracts `tenant_id` and writes it to Context
3. Logging middleware creates an `slog.Logger` with these fields and injects it into Context
4. Subsequent handlers retrieve the enriched Logger via `observability.ContextLogger(ctx)`

```go
// Usage in handler
logger := observability.ContextLogger(r.Context())
logger.Info("Processing request", "action", "chat_completion")
// Output: level=INFO msg="Processing request" request_id=abc123 tenant_id=xxx action=chat_completion
```

### Log Levels

Controlled via standard `slog` mechanisms:

| Level | Use Case |
|-------|----------|
| `DEBUG` | Hub search failures, detailed query information |
| `INFO` | Service startup, migration completion, request processing |
| `WARN` | Static directory not found, degraded processing |
| `ERROR` | Database connection failure, handler errors |

## Audit Logs

All authenticated requests are automatically recorded to the `audit_logs` table.

### Recorded Fields

| Field | Description |
|-------|-------------|
| `tenant_id` | Tenant the request belongs to |
| `user_id` | Authenticated identity ID |
| `action` | `METHOD /path` format |
| `detail` | Request details (sanitized) |
| `request_id` | Unique request identifier |
| `status_code` | HTTP response status code |
| `latency_ms` | Request processing time (milliseconds) |
| `created_at` | Record timestamp |

### Querying Audit Logs

```bash
curl "http://localhost:8080/v1/audit-logs?limit=50" \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

### Audit Log Features

- **Automatic recording**: Audit middleware records after response write, no handler intervention needed
- **Query sanitization**: Sensitive fields are cleaned before writing
- **Per-tenant isolation**: Audit logs automatically associated with the requester's tenant_id
- **Index optimization**: `idx_audit_tenant` and `idx_audit_request` accelerate queries

## Health Probes

### GET /health/live — Liveness Probe

Returns 200 as soon as the service starts, indicating the process is alive.

```json
{"status": "ok"}
```

### GET /health/ready — Readiness Probe

Checks database connection status, confirming the service can handle requests.

```json
{"status": "ready", "database": "ok"}
```

Returns 503 when the database is unavailable:

```json
{"status": "not_ready", "database": "error: connection refused"}
```

### Kubernetes Probe Configuration

```yaml
livenessProbe:
  httpGet:
    path: /health/live
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /health/ready
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 15
```

## Request ID Propagation

Every request carries a unique `X-Request-ID`:

1. If the request header contains `X-Request-ID`, the client-provided value is used
2. Otherwise, the server automatically generates a UUID
3. This ID spans the entire request lifecycle: logs, audit, tracing, response headers

```
Client → X-Request-ID: abc123
                │
                ▼
Logs:     request_id=abc123
Audit:    request_id=abc123
Tracing:  span.attribute("request_id", "abc123")
Response: X-Request-ID: abc123
```

## Monitoring Dashboards

### Recommended Grafana Panels

1. **Request Overview**: `hermes_http_requests_total` grouped by status
2. **Latency Distribution**: `hermes_http_request_duration_seconds` P50/P95/P99
3. **Concurrency**: `hermes_http_requests_in_flight`
4. **Tenant Activity**: `hermes_http_requests_total` grouped by tenant_id
5. **Error Rate**: `rate(hermes_http_requests_total{status=~"5.."}[5m])`
6. **Rate Limiting**: `hermes_http_requests_total{status="429"}` grouped by tenant_id

### Common PromQL Queries

```promql
# Requests per second (by status code)
sum(rate(hermes_http_requests_total[5m])) by (status)

# P99 latency
histogram_quantile(0.99, sum(rate(hermes_http_request_duration_seconds_bucket[5m])) by (le))

# Request distribution by tenant
sum(rate(hermes_http_requests_total[5m])) by (tenant_id)

# Rate-limited requests
sum(rate(hermes_http_requests_total{status="429"}[5m])) by (tenant_id)
```

## Related Documentation

- [Configuration Guide](configuration.md) — Observability environment variables
- [Deployment Guide](deployment.md) — Production environment checklist
- [Architecture Overview](architecture.md) — Middleware stack details
- [API Reference](api-reference.md) — /metrics and /health endpoints
