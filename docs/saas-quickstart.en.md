# SaaS Mode Quickstart

> Launch HermesX SaaS multi-tenant API server (v2.0.0) in 5 minutes.

## Prerequisites

| Dependency | Version | Notes |
|------------|---------|-------|
| Go | 1.22+ | To compile the hermesx binary |
| PostgreSQL | 16+ | Multi-tenant data store |
| Docker + Docker Compose | Latest | Optional, one-command infrastructure |

## Option 1: Pure Binary Quickstart (No Docker)

No Docker required — compile and run directly. Best for quick validation and local development.

### 1. Build the binary

```bash
git clone https://github.com/Colin4k1024/hermesx.git
cd hermesx
go build -o hermesx ./cmd/hermes/
```

### 2. Configuration wizard (optional)

```bash
./hermesx setup
# Interactive configuration of LLM API Key, provider, etc.
```

### 3. Start the service

```bash
export DATABASE_URL="postgres://hermes:hermes@127.0.0.1:5432/hermes?sslmode=disable"
export HERMES_ACP_TOKEN="admin-test-token"
export SAAS_ALLOWED_ORIGINS="*"
export SAAS_STATIC_DIR="./internal/dashboard/static"

# Start SaaS API server
./hermesx saas-api
```

> Note: A running PostgreSQL 16+ instance is required. Quick install: `brew install postgresql@16 && brew services start postgresql@16 && createdb hermes`

### 4. Quick verification

```bash
# Quick chat (uses mock model, no LLM API Key needed)
./hermesx chat --model mock "Hello, who are you?"
```

## Option 2: Docker Compose Quickstart (Recommended)

```bash
# 1. Clone the repository
git clone https://github.com/Colin4k1024/hermesx.git
cd hermesx

# 2. Build the binary
go build -o hermesx ./cmd/hermes/

# 3. Start infrastructure (PostgreSQL 16 + Redis 7 + MinIO)
docker compose -f docker-compose.dev.yml up -d postgres redis minio

# 4. Wait for services to be ready
docker compose -f docker-compose.dev.yml ps  # Confirm "healthy" status

# 5. Export environment variables
export DATABASE_URL="postgres://hermes:hermes@127.0.0.1:5432/hermes?sslmode=disable"
export HERMES_ACP_TOKEN="admin-test-token"
export SAAS_ALLOWED_ORIGINS="*"
export SAAS_STATIC_DIR="./internal/dashboard/static"

# 6. Start SaaS API server
./hermesx saas-api
```

On successful start:

```
SaaS API server running  port=8080
  openapi=http://localhost:8080/v1/openapi
  admin=http://localhost:8080/admin.html
  health_live=http://localhost:8080/health/live
  health_ready=http://localhost:8080/health/ready
```

## Option 3: Manual Configuration

### 1. Install and start PostgreSQL

```bash
# macOS
brew install postgresql@16
brew services start postgresql@16

# Create database
createdb hermes
```

### 2. Build and start

```bash
go build -o hermesx ./cmd/hermes/

export DATABASE_URL="postgres://$(whoami)@127.0.0.1:5432/hermes?sslmode=disable"
export HERMES_ACP_TOKEN="your-secret-admin-token"

./hermesx saas-api
```

Database tables are created automatically on first startup (27 migrations execute automatically).

## Docker Compose Configuration Comparison

| Config | Purpose | Services Included |
|--------|---------|-------------------|
| `docker-compose.quickstart.yml` | Single-node quick demo | hermesx + postgres + redis + minio + bootstrap |
| `docker-compose.dev.yml` | Local development (Gateway mode) | hermesx-gateway + postgres + redis + minio |
| `docker-compose.prod.yml` | Production deployment | hermesx-saas + postgres + redis + minio + OTel + Jaeger + Nginx LB |
| `docker-compose.saas.yml` | Full SaaS stack | hermesx-saas + postgres + redis + minio + hermesx-webui + bootstrap |
| `docker-compose.test.yml` | Integration tests | postgres-test + redis-test + minio-test (tmpfs, no persistence) |
| `docker-compose.webui.yml` | Standalone Web UI | hermesx-webui (requires external hermesx-saas) |

Default credentials (for development/demo — must be replaced in production):

| Service | Username | Password | Database/Bucket |
|---------|----------|----------|-----------------|
| PostgreSQL | `hermes` | `hermes` | `hermes` |
| Redis | — | none | — |
| MinIO | `hermes` | `hermesxpass` | `hermes-skills` |
| HermesX Admin Token | — | `dev-bootstrap-token` | — |

## Verify the Service

```bash
# Health checks
curl http://localhost:8080/health/live
# {"status":"ok"}

curl http://localhost:8080/health/ready
# {"status":"ready","database":"ok"}

# View current identity
curl http://localhost:8080/v1/me \
  -H "Authorization: Bearer admin-test-token"

# View OpenAPI spec
curl http://localhost:8080/v1/openapi
```

## Create Your First Tenant

```bash
# 1. Create a tenant
curl -X POST http://localhost:8080/v1/tenants \
  -H "Authorization: Bearer admin-test-token" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My Company",
    "plan": "pro",
    "rate_limit_rpm": 120,
    "max_sessions": 50
  }'
# Returns: {"id":"<tenant-id>", "name":"My Company", ...}

# 2. Create an API Key for this tenant
curl -X POST http://localhost:8080/v1/api-keys \
  -H "Authorization: Bearer admin-test-token" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "dev-key",
    "tenant_id": "<tenant-id>",
    "roles": ["user"]
  }'
# Returns: {"id":"...", "key":"hk_xxxx...", "prefix":"hk_xxxx"}
# Note: the key is returned only once — save it securely
```

## Send a Chat Request

```bash
# Use the API Key just created
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer hk_xxxx..." \
  -H "Content-Type: application/json" \
  -d '{
    "model": "mock",
    "messages": [
      {"role": "user", "content": "Hello, who am I?"}
    ]
  }'
```

The response will include the tenant identifier, confirming the request was routed to the correct tenant.

## Access the Admin Panel

Open a browser and navigate to:

- **Admin Panel**: http://localhost:8080/admin.html
- **Isolation Test Page**: http://localhost:8080/isolation-test.html

The admin panel provides self-service tenant management, API Key management, and usage viewing.

## Environment Variable Quick Reference

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DATABASE_URL` | Yes | - | PostgreSQL connection string |
| `HERMES_ACP_TOKEN` | Yes | - | Static admin Bearer Token |
| `SAAS_API_PORT` | No | `8080` | API server port |
| `SAAS_ALLOWED_ORIGINS` | No | - | CORS allowed origins; `*` means all |
| `SAAS_STATIC_DIR` | No | - | Static files directory |

Full configuration reference: [configuration.md](configuration.md)

## Enterprise SaaS Demo (11-Step Walkthrough)

`examples/enterprise-saas-demo/demo.sh` demonstrates the full lifecycle of HermesX v2.0.0 enterprise capabilities.

### Prerequisites

```bash
# Ensure SaaS API is running (any startup method)
./hermesx saas-api &
# or
docker compose -f docker-compose.prod.yml up -d hermesx-saas
```

### Run the Full Demo

```bash
# Set environment variables (optional, defaults provided)
export HERMES_URL="${HERMES_URL:-http://localhost:8080}"
export HERMES_ADMIN_TOKEN="${HERMES_ADMIN_TOKEN:-admin-test-token}"

# Run all 11 steps
./examples/enterprise-saas-demo/demo.sh
```

### Run Individual Steps

```bash
./examples/enterprise-saas-demo/demo.sh step1   # Create tenant
./examples/enterprise-saas-demo/demo.sh step2   # Create API Key
./examples/enterprise-saas-demo/demo.sh step3   # Verify identity
./examples/enterprise-saas-demo/demo.sh step4   # Create session
./examples/enterprise-saas-demo/demo.sh step5   # Chat Completion
./examples/enterprise-saas-demo/demo.sh step6   # Execution receipt audit
./examples/enterprise-saas-demo/demo.sh step7   # Usage metering
./examples/enterprise-saas-demo/demo.sh step8   # Audit logs
./examples/enterprise-saas-demo/demo.sh step9   # GDPR data export
./examples/enterprise-saas-demo/demo.sh step10  # Health checks
./examples/enterprise-saas-demo/demo.sh step11  # GDPR data deletion (dry-run)
```

### Enterprise Capabilities Demonstrated

| Step | Capability | Description |
|------|------------|-------------|
| step1 | Multi-tenant isolation | Create enterprise tenant with plan and resource limits |
| step2 | Credential management | Create scoped API Key |
| step3 | Identity verification | Verify identity context via API Key |
| step4 | Session management | Create chat session with metadata |
| step5 | Agent execution | Chat Completion invocation |
| step6 | Execution receipts | Auditable tool invocation records |
| step7 | Usage metering | Token usage and cost attribution |
| step8 | Audit logs | Compliance-grade operation audit trail |
| step9 | GDPR export | Full tenant data export |
| step10 | Health checks | Runtime readiness/liveness probes |
| step11 | GDPR deletion | Complete tenant data deletion (dry-run) |

## Verification Checklist

After installation, verify each item in the following checklist:

### Basic Verification

- [ ] `./hermesx --version` outputs v2.0.0
- [ ] `curl http://localhost:8080/health/live` returns `{"status":"ok"}`
- [ ] `curl http://localhost:8080/health/ready` returns `{"status":"ready","database":"ok"}`
- [ ] `curl http://localhost:8080/v1/me -H "Authorization: Bearer admin-test-token"` returns identity info

### Multi-Tenant Verification

- [ ] Successfully created a tenant and received tenant_id
- [ ] Successfully created an API Key and received the key
- [ ] Chat requests using the API Key correctly route to the corresponding tenant

### Observability Verification (Production Deployment)

- [ ] `curl http://localhost:8080/v1/metrics` returns Prometheus metrics
- [ ] Jaeger UI (http://localhost:16686) is accessible and shows trace data
- [ ] OTel Collector is receiving telemetry from hermesx

### Advanced Verification

- [ ] `docker compose -f docker-compose.prod.yml ps` shows all services as healthy
- [ ] Nginx load balancing is working (multi-replica scenario)
- [ ] MinIO console is accessible and the skills bucket is created

## v2.0.0 New Capabilities

After absorbing upstream hermes-agent v2026.4.30, SaaS mode automatically gains the following agent enhancements:

| Capability | Description | Configuration |
|------------|-------------|---------------|
| Context compression | Auto-summarize history near token limit to maintain long conversation coherence | `context_compression: true` |
| Multimodal routing | Auto-dispatch image/audio/video requests based on provider capabilities | Configure `AUXILIARY_VISION_*` |
| Autonomous memory curation | Deduplication, LLM merging, expiry cleanup | Enabled automatically |
| Self-improvement loop | Periodic conversation quality self-evaluation, persist improvement insights | Enabled automatically |
| CJK fuzzy search | pg_trgm fuzzy matching for Chinese/Japanese/Korean text | PostgreSQL pg_trgm extension |
| Model catalog hot-reload | Update available model list at runtime without restart | Enabled automatically |

These capabilities are fully backward-compatible with existing APIs — no client changes needed.

## Next Steps

- [API Reference](api-reference.md) — Complete endpoint documentation
- [Authentication](authentication.md) — Auth Chain, API Keys, RBAC
- [Configuration Guide](configuration.md) — All environment variables
- [Deployment Guide](deployment.md) — Docker Compose / Helm / Kind (with v2.0.0 production checklist)
- [Architecture Overview](architecture.md) — System design and data flow
