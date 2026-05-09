# Changelog v1.3.0 — Enterprise Hardening Sprint

> Release candidate for production deployment.
> Builds on v1.0.0 (platform foundation) and v1.1.0 (production readiness).

## Security Fixes (Week 3)

### CRITICAL: API Key Tenant Boundary Enforcement
- **Before**: Non-admin callers could pass `tenant_id` in request body to create API keys for OTHER tenants.
- **After**: Tenant derivation is strictly from credential context. Body-supplied `tenant_id` only honored for admin role callers.
- File: `internal/api/apikeys.go`

### Auditor Role Added
- New RBAC role `auditor` for read-only access to audit logs and execution receipts.
- Routes `/v1/audit-logs` and `/v1/execution-receipts` gated behind `auditor` role.

### `generateRawKey()` Hardened
- Explicit panic on `crypto/rand.Read` failure (was silently returning partial key).

## ExecutionReceipt (Week 4)

Auditable tool execution records with idempotency support.

### Schema
- Migration 75-80: `execution_receipts` table with RLS policy, tenant index, session index, unique idempotency index.
- Fields: `id`, `tenant_id`, `session_id`, `user_id`, `tool_name`, `input`, `output`, `status`, `duration_ms`, `idempotency_id`, `trace_id`, `created_at`.

### Store Interface
- `ExecutionReceiptStore` with `Create`, `Get`, `List`, `GetByIdempotencyID`.
- `ReceiptListOptions` supports filtering by session_id, tool_name, status.

### API
- `GET /v1/execution-receipts` — list with pagination and filters (auditor role).
- `GET /v1/execution-receipts/{id}` — get by ID (auditor role).

## Tool Runtime Integration (Week 5)

### ReceiptRecorder
- `DispatchWithReceipt()` wraps tool execution with timing, input/output capture, and status determination.
- Idempotency dedup: if `idempotency_id` already exists, returns cached result without re-execution.
- Input/output truncated to 4096 bytes for storage efficiency.
- Trace ID propagation via `ToolContext.Extra["trace_id"]`.

## OpenAPI Specification (Week 6)

Expanded from 12 paths to full API surface:
- 22 documented endpoints with tags, parameters, and schemas.
- Component schemas: `ChatRequest`, `Message`, `CreateAPIKeyRequest`, `ExecutionReceipt`, `Tenant`, `SandboxPolicy`.
- Security scheme documentation (API Key / JWT / Static Token).
- Available at `GET /v1/openapi`.

## Deployment (Week 7)

### Production Compose (`docker-compose.prod.yml`)
- Full stack: PostgreSQL 16, Redis 7 (AOF + LRU), MinIO, OTel Collector, Jaeger.
- Resource limits on all containers.
- Health checks with proper dependency ordering.
- OTel tracing piped to Jaeger via gRPC.

### Observability
- `deploy/otel-collector.yaml`: traces → Jaeger, metrics → Prometheus exporter.
- Memory limiter processor (512MB cap).

### Backup/Restore
- `scripts/backup/backup.sh`: automated pg_dump with gzip compression, 7-day retention.
- `scripts/backup/restore.sh`: single-transaction restore with post-restore migration.

## Migration Summary

| Version | Description |
|---------|-------------|
| 75 | CREATE TABLE execution_receipts |
| 76 | Index: tenant_id |
| 77 | Index: tenant_id + session_id |
| 78 | Unique index: tenant_id + idempotency_id |
| 79 | ENABLE ROW LEVEL SECURITY |
| 80 | RLS policy: tenant_isolation_exec_receipts |

## Files Changed

| File | Action |
|------|--------|
| `internal/store/store.go` | Added ExecutionReceiptStore interface |
| `internal/store/types.go` | Added ExecutionReceipt struct (Week 4 prior) |
| `internal/store/pg/execution_receipts.go` | NEW — PG implementation |
| `internal/store/pg/pg.go` | Registered ExecutionReceipts sub-store |
| `internal/store/pg/migrate.go` | Migrations 75-80 |
| `internal/store/sqlite/noop.go` | Noop ExecutionReceipt implementation |
| `internal/api/execution_receipts.go` | NEW — HTTP handler |
| `internal/api/openapi.go` | Expanded to full spec |
| `internal/api/server.go` | Registered /v1/execution-receipts routes |
| `internal/api/apikeys.go` | Tenant boundary fix |
| `internal/tools/receipt_recorder.go` | NEW — Tool runtime integration |
| `cmd/hermes/saas.go` | Auditor role + route config |
| `docker-compose.prod.yml` | NEW — Production compose |
| `deploy/otel-collector.yaml` | NEW — OTel config |
| `scripts/backup/backup.sh` | NEW — Backup script |
| `scripts/backup/restore.sh` | NEW — Restore script |

## Observability Enhancements (Week 8)

### Prometheus Business Metrics
- `hermes_tool_executions_total` — counter by tool_name, status, tenant_id
- `hermes_tool_execution_duration_seconds` — histogram by tool_name, status, tenant_id
- `hermes_chat_completions_total` — counter by tenant_id, status
- `hermes_chat_completion_duration_seconds` — histogram by tenant_id
- `hermes_store_operation_duration_seconds` — histogram by operation, entity
- ReceiptRecorder now emits tool execution metrics on every dispatch.

### Production Deployment Guide
- Expanded `docs/DEPLOYMENT.md` with:
  - Complete environment variable reference
  - Prometheus metrics table and alerting rules
  - Backup/restore procedures
  - Horizontal scaling guidelines
  - Security hardening checklist
  - Rollback strategy and triggers

## Verification

```bash
# Build
go build ./...
go vet ./...

# Unit tests (no DB required — 21 packages, all passing)
go test ./...

# Integration tests (requires docker-compose.test.yml)
make test-integration

# Multi-replica HA
cd deploy/ && docker compose -f docker-compose.multi-replica.yml up -d --build
```
