# HermesX v3.0.0 Migration Guide

## Overview

HermesX v3.0.0 introduces several new features and improvements. This guide helps you upgrade from v2.x to v3.0.0.

## Breaking Changes

### 1. API Changes

#### New Endpoints

- `GET /admin/v1/usage` - Usage aggregation API
- `GET /admin/v1/usage/tenants` - Tenant usage aggregation
- `POST /admin/v1/tenants/{id}/api-keys/{kid}/rotate` - API key rotation

#### Deprecated Endpoints

None in v3.0.0.

### 2. Configuration Changes

#### SANDBOX_MODE

The `SANDBOX_MODE` environment variable is now required for SaaS deployments. Options:

- `local` - Local execution (development only)
- `docker` - Docker container isolation
- `k8s-job` - Kubernetes Job isolation (recommended for production)

```bash
# Production recommendation
SANDBOX_MODE=k8s-job
K8S_JOB_NAMESPACE=default
K8S_JOB_IMAGE=ubuntu:latest
K8S_JOB_CPU_LIMIT=500m
K8S_JOB_MEMORY_LIMIT=256Mi
K8S_JOB_RUNTIME_CLASS=gvisor
```

### 3. Database Changes

#### New Tables

- `execution_receipts` - Stores tool execution records for audit

#### New Columns

None in v3.0.0.

### 4. Authentication Changes

#### API Key Rotation

API keys can now be rotated without revoking the old key first. Use the new endpoint:

```bash
curl -X POST http://localhost:8080/admin/v1/tenants/{id}/api-keys/{kid}/rotate \
  -H "Authorization: Bearer {admin-key}"
```

## Upgrade Steps

### Step 1: Backup Database

```bash
pg_dump -U hermesx hermesx > hermesx_backup_$(date +%Y%m%d).sql
```

### Step 2: Update Configuration

Add the following to your environment configuration:

```bash
SANDBOX_MODE=k8s-job
K8S_JOB_NAMESPACE=default
K8S_JOB_IMAGE=ubuntu:latest
K8S_JOB_CPU_LIMIT=500m
K8S_JOB_MEMORY_LIMIT=256Mi
K8S_JOB_RUNTIME_CLASS=gvisor
```

### Step 3: Update Docker Compose

If using Docker Compose, update to the latest version:

```bash
git pull origin main
docker compose -f docker-compose.prod.yml up -d
```

### Step 4: Run Database Migrations

Migrations will run automatically on startup. Verify:

```bash
curl http://localhost:8080/health/ready
```

### Step 5: Verify Installation

```bash
# Check health
curl http://localhost:8080/health/ready

# Run enterprise demo
./examples/enterprise-saas-demo/demo.sh
```

## New Features

### 1. Execution Receipts

All tool calls now generate execution receipts for audit purposes. Query receipts:

```bash
curl http://localhost:8080/v1/sessions/{session-id}/receipts \
  -H "Authorization: Bearer {api-key}"
```

### 2. Usage Aggregation API

Query tenant usage:

```bash
curl "http://localhost:8080/admin/v1/usage?tenant_id={tenant-id}&granularity=daily&from=2026-07-01&to=2026-07-31" \
  -H "Authorization: Bearer {admin-key}"
```

### 3. API Key Rotation

Rotate API keys without downtime:

```bash
curl -X POST http://localhost:8080/admin/v1/tenants/{id}/api-keys/{kid}/rotate \
  -H "Authorization: Bearer {admin-key}"
```

### 4. K8s Job Sandbox

Run code execution in isolated Kubernetes Jobs:

```bash
export SANDBOX_MODE=k8s-job
export K8S_JOB_NAMESPACE=default
export K8S_JOB_IMAGE=ubuntu:latest
```

## SDK Updates

### Go SDK

```bash
go get github.com/Colin4k1024/hermesx/sdk/go@v3.0.0
```

### TypeScript SDK

```bash
npm install @hermesx/sdk@3.0.0
```

## Troubleshooting

### Issue: SANDBOX_MODE error

```
error: SANDBOX_MODE is required for execute_code in SaaS-only mode
```

**Solution**: Set the `SANDBOX_MODE` environment variable:

```bash
export SANDBOX_MODE=k8s-job
```

### Issue: K8s Job fails

```
error: k8s-job: failed to create job
```

**Solution**: Ensure kubectl is configured and can reach the cluster:

```bash
kubectl cluster-info
```

### Issue: Execution receipts not showing

**Solution**: Ensure the `execution_receipts` table exists:

```bash
psql -U hermesx -d hermesx -c "\dt execution_receipts"
```

## Rollback

If you need to rollback to v2.x:

1. Stop the v3.0.0 service
2. Restore the database backup
3. Deploy v2.x version
4. Remove v3.0.0 specific configuration

```bash
# Restore database
psql -U hermesx -d hermesx < hermesx_backup_20260625.sql

# Deploy v2.x
git checkout v2.3.0
docker compose -f docker-compose.prod.yml up -d
```

## Support

For issues or questions:

- GitHub Issues: https://github.com/Colin4k1024/hermesx/issues
- Documentation: https://hermesx.dev/docs
- Security: See [SECURITY.md](SECURITY.md)
