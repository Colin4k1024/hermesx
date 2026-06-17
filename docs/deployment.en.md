# Deployment Guide

> SaaS-only deployment methods for the HermesX v2.4.0-dev SaaS API: Docker Compose, Kubernetes, and Helm.

## HermesX v2.0.0 Deployment Notes

HermesX v2.0.0 introduces the following key changes compared to v1.x:

| Change | v1.x | v2.0.0 |
|--------|------|--------|
| Binary name | `hermes` | `hermesx` |
| Helm Chart path | `deploy/helm/hermes-agent/` | `deploy/helm/hermesx/` |
| Default replica count | 1 | 2 |
| PodDisruptionBudget | Not configured | Enabled by default (minAvailable: 1) |
| HPA | Disabled by default | Enabled by default (2-10 replicas) |
| OTel configuration | Manual mount | Built-in environment variable injection |
| LLM Model | Single model | Hot-reload model catalog supported |

### Pre-deployment Checklist

- [ ] Go 1.23+ for building from source
- [ ] PostgreSQL 16+ (RLS support required)
- [ ] Redis 7+ (for rate limiting Lua scripts)
- [ ] Kubernetes 1.28+ (if using Helm deployment)
- [ ] Docker 24+ (if using Docker Compose deployment)

## SaaS Image

`Dockerfile.saas` is the only supported release image. It builds the Go SaaS API binary, builds the React WebUI, copies the WebUI output to `/static`, bundles default skills for tenant provisioning, and defaults to `CMD ["saas-api"]`.

### Dockerfile.saas Features

```dockerfile
# Multi-stage build
# Stage 1: Build WebUI
# Stage 2: Compile Go binary
# Stage 3: Copy binary + WebUI static files to runtime image
# Includes /static for SAAS_STATIC_DIR
# Default CMD: ["saas-api"]
```

## Docker Compose Configuration Comparison

| Config | Use Case | Services | API Port | Health Check |
|--------|----------|----------|----------|--------------|
| `docker-compose.prod.yml` | Production deployment | hermesx-saas + postgres + redis + minio + OTel + Jaeger + Nginx LB | 8080/8081 | wget health/live |
| `docker-compose.saas.yml` | SaaS stack for staging or local SaaS validation | hermesx-saas + postgres + redis + minio | 18080/18081 | curl health/ready |
| `docker-compose.test.yml` | Integration testing | postgres-test + redis-test + minio-test (tmpfs, no persistence) | Isolated test ports | pg_isready |

### Production Configuration (docker-compose.prod.yml)

`docker-compose.prod.yml` is the recommended production Docker Compose configuration, featuring:

- **OTel Collector**: Receives gRPC/HTTP OTLP, exports to Jaeger + Prometheus
- **Jaeger**: Distributed tracing backend (UI: http://localhost:16686)
- **Nginx**: 3-replica load balancing (ip_hash session affinity)
- **Resource limits**: CPU/memory limits configured for each service
- **Health checks**: All critical services have healthcheck configured
- **Backup scripts**: postgres container mounts `./scripts/backup` directory

```bash
# Start the full production stack
docker compose -f docker-compose.prod.yml up -d

# View all service status
docker compose -f docker-compose.prod.yml ps

# View OTel collector logs
docker compose -f docker-compose.prod.yml logs -f otel-collector

# Access Jaeger UI (tracing)
open http://localhost:16686

# Access Prometheus metrics
curl http://localhost:8889/metrics | grep hermesx
```

## Method 1: Docker Compose SaaS

Use `docker-compose.prod.yml` for production-like deployments and `docker-compose.saas.yml` for staging or local SaaS validation. Both run `hermesx saas-api`; the API service serves the embedded WebUI from `/static`.

```bash
docker compose -f docker-compose.saas.yml up -d --build
curl http://localhost:18080/health/ready
open http://localhost:18080/admin.html
```

## Method 2: Kubernetes / Kind Validation

Use [Kind](https://kind.sigs.k8s.io/) to run a Kubernetes cluster locally.

### 1. Create Cluster

```bash
kind create cluster --name hermes
```

### 2. Deploy PostgreSQL

```bash
kubectl apply -f deploy/kind/postgres.yaml
```

`postgres.yaml` includes:
- PersistentVolumeClaim (1Gi)
- Deployment (PostgreSQL 16 single instance)
- Service (ClusterIP, port 5432)
- ConfigMap (initialize user and database)

### 3. Build and Load Image

```bash
# Build SaaS image
docker build -t hermes-agent-saas:local -f Dockerfile.saas .

# Load into Kind cluster
kind load docker-image hermes-agent-saas:local --name hermes
```

### 4. Install Helm Chart

```bash
# v2.0.0: Chart path changed to hermesx/
helm install hermesx deploy/helm/hermesx/ \
  -f deploy/kind/values.local.yaml
```

`values.local.yaml` overrides:
- `image.pullPolicy: Never` (use local image)
- `DATABASE_URL` pointing to PostgreSQL Service inside Kind

### 5. Verify

```bash
kubectl get pods
kubectl port-forward svc/hermesx 8080:8080

curl http://localhost:8080/health/ready
```

### OTel Collector Integration

`deploy/otel-collector.yaml` is the observability collector configuration built into HermesX v2.0.0.

| Protocol | Port | Description |
|----------|------|-------------|
| OTLP gRPC | 4317 | Receives HermesX OTel exports (recommended) |
| OTLP HTTP | 4318 | HTTP method to receive OTel data |

Processors: `memory_limiter` (512MiB limit) + `batch` (1024 batch, 5s timeout)  
Exporters: Jaeger (traces) + Prometheus:8889 (metrics) + Logging (warn level)

```bash
# Enable OTel in Helm
helm install hermesx deploy/helm/hermesx/ \
  --set env.OTEL_EXPORTER_OTLP_ENDPOINT="http://otel-collector:4317" \
  --set env.OTEL_SERVICE_NAME="hermesx"

# Deploy OTel Collector independently
kubectl apply -f deploy/otel-collector.yaml
```

## Method 3: Helm Chart Production Deployment

### Chart Structure

```
deploy/helm/hermesx/
├── Chart.yaml          # Chart metadata
├── values.yaml         # Default values
└── templates/          # K8s resource templates
```

### Install

```bash
kubectl create secret generic hermesx-runtime \
  --namespace hermesx \
  --from-literal=DATABASE_URL="postgres://user:pass@pg-host:5432/hermes?sslmode=require" \
  --from-literal=HERMES_ACP_TOKEN="$(openssl rand -hex 32)" \
  --from-literal=HERMES_API_KEY="$(openssl rand -hex 32)" \
  --from-literal=REDIS_URL="redis://redis:6379" \
  --from-literal=LLM_API_KEY="replace-me" \
  --from-literal=MINIO_ACCESS_KEY="replace-me" \
  --from-literal=MINIO_SECRET_KEY="replace-me"

helm install hermesx deploy/helm/hermesx/ \
  --namespace hermesx \
  --create-namespace \
  --set image.tag="v2.4.0-dev" \
  --set secretEnv.existingSecret="hermesx-runtime" \
  --set env.SAAS_ALLOWED_ORIGINS="https://your-domain.example.com"
```

### Key values.yaml Configuration

```yaml
replicaCount: 2

image:
  repository: hermesx/hermesx-saas
  # Empty defaults to Chart.appVersion. Pin a release tag or digest in production.
  tag: ""
  pullPolicy: IfNotPresent

secretEnv:
  # Prefer a pre-created Kubernetes Secret for sensitive values.
  existingSecret: "hermesx-runtime"
  keys:
    DATABASE_URL: DATABASE_URL
    HERMES_ACP_TOKEN: HERMES_ACP_TOKEN
    HERMES_API_KEY: HERMES_API_KEY
    REDIS_URL: REDIS_URL
    LLM_API_KEY: LLM_API_KEY
    MINIO_ACCESS_KEY: MINIO_ACCESS_KEY
    MINIO_SECRET_KEY: MINIO_SECRET_KEY

serviceAccount:
  create: true
  automountServiceAccountToken: false

service:
  type: ClusterIP
  port: 8080

args:
  - saas-api

env:
  SAAS_API_PORT: "8080"
  SAAS_ALLOWED_ORIGINS: "https://your-domain.example.com"  # Must set specific domain in production
  SAAS_STATIC_DIR: "/static"
  HERMES_API_PORT: "8081"
  LLM_API_URL: ""
  LLM_MODEL: ""

podSecurityContext:
  seccompProfile:
    type: RuntimeDefault

containerSecurityContext:
  runAsNonRoot: true
  runAsUser: 1000
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop:
      - ALL

networkPolicy:
  enabled: false  # Enable and restrict ingressFrom/egressTo for production namespaces.

resources:
  limits:
    cpu: "1000m"
    memory: "512Mi"
  requests:
    cpu: "200m"
    memory: "128Mi"

probes:
  liveness:
    path: /health/live
    initialDelaySeconds: 5
    periodSeconds: 10
  readiness:
    path: /health/ready
    initialDelaySeconds: 5
    periodSeconds: 10

# PodDisruptionBudget — enabled by default in v2.0.0
pdb:
  enabled: true
  minAvailable: 1

# Auto-scaling — enabled by default in v2.0.0
autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70
  targetMemoryUtilizationPercentage: 80
  scaleDownStabilizationSeconds: 300

# TLS
tls:
  enabled: false
  certFile: ""
  keyFile: ""

# PostgreSQL sub-chart (for development)
postgresql:
  enabled: true
  auth:
    database: hermesx
    username: hermes
    password: hermes-dev-password
```

### Using External PostgreSQL

Production environments should use externally managed PostgreSQL:

```bash
helm install hermesx deploy/helm/hermesx/ \
  --set postgresql.enabled=false \
  --set env.DATABASE_URL="postgres://hermes:pass@rds-endpoint:5432/hermes?sslmode=require"
```

## Production Pre-flight Checklist

### Security Checks

- [ ] `HERMES_ACP_TOKEN` uses a high-entropy random string (32+ characters)
- [ ] `SAAS_ALLOWED_ORIGINS` set to specific domains — do not use `*`
- [ ] `DATABASE_URL` injected via Kubernetes Secret (no plaintext values)
- [ ] TLS enabled (via Ingress or `tls.enabled`)
- [ ] API Key rotation mechanism established
- [ ] All `changeme` placeholders in Helm values.yaml replaced

### High Availability Checks

- [ ] `replicaCount >= 2` (v2.0.0 default 2)
- [ ] `autoscaling.enabled: true` (enabled by default in v2.0.0)
- [ ] `pdb.enabled: true` (enabled by default in v2.0.0, minAvailable: 1)
- [ ] Health probes `liveness` and `readiness` configured
- [ ] PostgreSQL configured with read replicas or using managed service (RDS/Cloud SQL)
- [ ] Redis AOF persistence enabled for distributed rate limiting

### Observability Checks

- [ ] `/metrics` endpoint connected to Prometheus (see metrics table below)
- [ ] `OTEL_EXPORTER_OTLP_ENDPOINT` configured for OpenTelemetry Collector
- [ ] `OTEL_SERVICE_NAME` set to `hermesx`
- [ ] Logs collected to centralized logging platform (EFK/Loki)
- [ ] Audit log retention policy configured (recommended 365 days)
- [ ] OTel Collector deployed and verified to receive data

### Resource Planning Checks

- [ ] CPU/Memory requests and limits configured (see scaling table below)
- [ ] PostgreSQL connection pool configured (PgBouncer recommended, required for >5 instances)
- [ ] MinIO using persistent storage volumes (not hostPath)
- [ ] Consider partitioning for `audit_logs` / `execution_receipts` > 10M rows

### v2.0.0-Specific Checks

- [ ] Helm Chart path updated to `deploy/helm/hermesx/` (no longer `hermes-agent/`)
- [ ] Image repository updated to `hermesx/hermesx-saas` (no longer `hermes-agent-saas`)
- [ ] `HERMES_API_PORT: "8081"` added to values (v2.0.0 dual-port architecture)
- [ ] `REDIS_URL` environment variable configured (v2.0.0 rate limiting dependency)
- [ ] `MINIO_BUCKET` set to `hermesx-skills` (differentiate multiple environments)

## Method 4: Multi-Replica HA (Docker Compose)

3 instances + Nginx LB, suitable for small-scale production or validating horizontal scaling.

```bash
cd deploy/
docker compose -f docker-compose.multi-replica.yml up -d --build
```

Architecture: Nginx (ip_hash) → 3× hermes instances → Shared PG + Redis + MinIO

---

## Production Environment Variable Reference

### Required

| Variable | Description | Example |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection string | `postgres://user:pass@host:5432/hermes?sslmode=require` |
| `HERMES_API_KEY` | API authentication Bearer token | `sk-prod-xxxxx` |
| `HERMES_API_KEY_LLM` | LLM Provider API key | `sk-...` |
| `HERMES_PROVIDER` | LLM Provider | `openai`, `anthropic`, `gemini` |
| `HERMES_MODEL` | Default model | `gpt-4o`, `claude-sonnet-4-20250514` |

### Infrastructure (Recommended)

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_URL` | — | Redis connection, enables distributed rate limiting (new in v2.0.0) |
| `MINIO_ENDPOINT` | — | MinIO/S3 endpoint |
| `MINIO_ACCESS_KEY` | — | MinIO access key |
| `MINIO_SECRET_KEY` | — | MinIO secret key |
| `MINIO_BUCKET` | `hermes-skills` | Skills bucket |

Enterprise multi-replica deployments must configure `REDIS_URL`. When Redis is unavailable, the runtime falls back to the in-process `LocalDualLimiter`; this is an availability-first failure policy. Each replica counts independently, so the effective limit during the outage window is approximately `limit × replica_count`. Regulated environments should record Redis HA/failover evidence or enforce fail-closed/load-shedding at the ingress layer.

### SaaS API (v2.0.0)

| Variable | Default | Description |
|----------|---------|-------------|
| `HERMES_ACP_TOKEN` | — | Static admin token (required) |
| `SAAS_API_PORT` | `8080` | SaaS API port |
| `SAAS_ALLOWED_ORIGINS` | — (CORS disabled) | CORS allowed origins (must set specific domains in production) |
| `SAAS_STATIC_DIR` | — | Static files directory |
| `HERMES_API_PORT` | `8081` | HTTP API port (new in v2.0.0) |
| `HERMES_API_KEY` | — | API authentication token (new in v2.0.0) |

### Agent Runtime

| Variable | Default | Description |
|----------|---------|-------------|
| `HERMES_INSTANCE_ID` | hostname | HA instance identifier |
| `HERMES_MAX_ITERATIONS` | `20` | Agent max iterations |
| `HERMES_MAX_TOKENS` | `4096` | Max response tokens |
| `HERMES_BASE_URL` | provider default | Custom LLM endpoint |
| `HERMES_DEBUG` | `false` | Debug logging |
| `HERMES_ENV` | `development` | Set to `production` to make egress default to `deny-all` |
| `HERMES_EGRESS_DEFAULT` | derived from environment | Explicit egress default override: `allow-all`, `deny-all`, or `log-only` |

### Observability (v2.0.0 Enhanced)

| Variable | Default | Description |
|----------|---------|-------------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | — | OTel Collector gRPC/HTTP |
| `OTEL_EXPORTER_OTLP_INSECURE` | `false` | Disable OTel TLS |
| `OTEL_SERVICE_NAME` | `hermesx` | Service name (default value changed in v2.0.0) |

---

## Prometheus Metrics

Scrape endpoint: `GET /v1/metrics`

| Metric | Type | Labels |
|--------|------|--------|
| `hermes_http_requests_total` | Counter | method, path, status, tenant_id |
| `hermes_http_request_duration_seconds` | Histogram | method, path, tenant_id |
| `hermes_http_requests_in_flight` | Gauge | — |
| `hermes_llm_request_duration_seconds` | Histogram | provider, model, status, tenant_id |
| `hermes_llm_tokens_total` | Counter | provider, model, direction, tenant_id |
| `hermes_rate_limit_rejected_total` | Counter | tenant_id |
| `hermes_tool_executions_total` | Counter | tool_name, status, tenant_id |
| `hermes_tool_execution_duration_seconds` | Histogram | tool_name, status, tenant_id |
| `hermes_active_sessions` | Gauge | tenant_id |
| `hermes_chat_completions_total` | Counter | tenant_id, status |
| `hermes_store_operation_duration_seconds` | Histogram | operation, entity |

### Recommended Alert Rules

```yaml
- alert: HermesHighErrorRate
  expr: rate(hermes_http_requests_total{status=~"5.."}[5m]) / rate(hermes_http_requests_total[5m]) > 0.05
  for: 2m

- alert: HermesLLMSlow
  expr: histogram_quantile(0.95, rate(hermes_llm_request_duration_seconds_bucket[5m])) > 30
  for: 5m

- alert: HermesRateLimitSurge
  expr: rate(hermes_rate_limit_rejected_total[5m]) > 100
  for: 1m
```

---

## Backup & Recovery

### RPO/RTO Targets

| Component | RPO (Max Data Loss) | RTO (Max Recovery Time) | Strategy |
|-----------|---------------------|--------------------------|----------|
| PostgreSQL | < 5 min | < 1 h | WAL archiving + pg_dump |
| Redis | < 15 min | < 5 min | BGSAVE + RDB copy |
| MinIO | < 1 h | < 30 min | mc mirror full sync |

### PostgreSQL Backup

```bash
# Automated backup (run inside postgres container or host with pg_dump)
./scripts/backup/backup.sh /backup
# Output: /backup/hermes_YYYYMMDD_HHMMSS.sql.gz
# BACKUP_RETENTION_DAYS=7 (default 7-day retention)
```

### PostgreSQL Restore

```bash
./scripts/backup/restore.sh /backup/hermes_20260507_120000.sql.gz
# Single-transaction restore + automatic pending migration execution
```

### PITR

Production environments should enable WAL archiving for < 5 min RPO. Configuration templates are in `deploy/pitr/`.

### Redis Backup

```bash
# Triggers BGSAVE and copies RDB to backup location
./scripts/redis-backup.sh

# Environment variables:
#   REDIS_HOST=localhost      Redis hostname
#   REDIS_PORT=6379           Redis port
#   REDIS_PASSWORD=           Redis auth password
#   BACKUP_DIR=/backup/redis  Local backup directory
#   S3_BUCKET=                Optional S3 bucket (auto-uploads when set)
#   REDIS_DATA_DIR=/data      Redis data directory
#   RETENTION_DAYS=7          Local backup retention in days
```

### Redis Restore

```bash
# 1. Stop Redis service
docker compose stop redis

# 2. Copy backup RDB to Redis data directory
cp /backup/redis/redis-20260515_120000.rdb /data/dump.rdb

# 3. Start Redis (automatically loads dump.rdb)
docker compose start redis

# 4. Verify
redis-cli DBSIZE
```

### MinIO Backup

```bash
# Mirror bucket to local directory or remote bucket
./scripts/minio-backup.sh

# Environment variables:
#   MINIO_ENDPOINT=http://localhost:9000   MinIO endpoint
#   MINIO_ACCESS_KEY=                      Access key (required)
#   MINIO_SECRET_KEY=                      Secret key (required)
#   SOURCE_BUCKET=hermes-skills            Source bucket name
#   TARGET=/backup/minio                   Target: local path or s3://bucket-name
#   RETENTION_DAYS=7                       Local backup retention in days
```

### MinIO Restore

```bash
# Restore from local backup to MinIO
mc alias set hermesx http://localhost:9000 $MINIO_ACCESS_KEY $MINIO_SECRET_KEY
mc mirror /backup/minio/hermes-skills-20260515_120000 hermesx/hermes-skills

# Verify
mc ls --recursive hermesx/hermes-skills | wc -l
```

### Disaster Recovery Verification

```bash
# Run DR test script to verify backups are restorable
./scripts/dr-test.sh

# This script will:
#   1. Check Redis backup file existence and integrity
#   2. Check MinIO backup directory and file counts
#   3. Verify data consistency against live Redis and MinIO
#   4. Output PASS/FAIL report
```

### Cron Schedule Recommendations

```cron
# PostgreSQL — backup every 4 hours
0 */4 * * * /opt/hermesx/scripts/backup/backup.sh /backup/postgres >> /var/log/hermesx-backup-pg.log 2>&1

# Redis — backup every 15 minutes
*/15 * * * * /opt/hermesx/scripts/redis-backup.sh >> /var/log/hermesx-backup-redis.log 2>&1

# MinIO — daily full mirror at 2 AM
0 2 * * * /opt/hermesx/scripts/minio-backup.sh >> /var/log/hermesx-backup-minio.log 2>&1

# DR verification — weekly on Sunday at 4 AM
0 4 * * 0 /opt/hermesx/scripts/dr-test.sh >> /var/log/hermesx-dr-test.log 2>&1
```

---

## Horizontal Scaling

HermesX instances are stateless — all persistent state lives in PG + Redis.

| Load | CPU/Instance | Memory/Instance | Instances |
|------|-------------|----------------|-----------|
| < 100 req/s | 1 core | 512MB | 1-2 |
| 100-500 req/s | 2 cores | 1GB | 3-5 |
| 500+ req/s | 4 cores | 2GB | 5+ |

Database scaling recommendations:
- Use PgBouncer connection pooling for > 5 instances
- Consider partitioning for `audit_logs` / `execution_receipts` > 10M rows

---

## Security Hardening

### Authentication System

1. **API Key**: SHA-256 hash storage, supports scopes + expiry + rotation
2. **JWT**: Signature verification + claims-based tenant_id extraction
3. **Static Token**: Simple Bearer token for single-tenant deployments

### Row-Level Security (RLS)

All tenant data tables have PostgreSQL RLS enabled. Each transaction sets the context via `SET LOCAL app.current_tenant` — even if there is a bug at the application layer, the database layer prevents cross-tenant access.

### Network Security

- API bound to internal network only, exposed via reverse proxy + TLS
- PG/Redis/MinIO must not be publicly exposed
- Enable TLS for MinIO in production

---

## Rollback Strategy

### Application Rollback

```bash
docker compose -f docker-compose.prod.yml up -d --no-build  # Use previous image
# or
docker service update --image ghcr.io/org/hermes:previous-tag hermes
```

### Database Rollback

Migrations are forward-only. Rollback steps:
1. Restore from most recent backup
2. Deploy the previous application version
3. Verify data integrity

### Rollback Triggers

- Error rate > 5% sustained for 5 minutes
- P95 latency > 30s sustained for 5 minutes
- Data integrity alert (cross-tenant data leak)
- Critical security vulnerability discovered post-release

---

## Related Documentation

- [Getting Started](saas-quickstart.md) — Local dev environment (binary/Docker comparison)
- [Configuration Guide](configuration.md) — All environment variables
- [Observability](observability.md) — Monitoring and tracing
- [Architecture Overview](architecture.md) — System design
- [Changelog v2.0.0](CHANGELOG.md) — v2.0.0 change log
