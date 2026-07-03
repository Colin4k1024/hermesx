# HermesX Deployment Assets

This directory contains deployment and observability assets for HermesX.

## Directory Structure

```
deploy/
├── grafana/
│   └── hermesx-dashboard.json    # Grafana dashboard for monitoring
├── scripts/
│   └── backup.sh                 # Backup automation script
└── README.md                     # This file
```

## Grafana Dashboard

### Features

The dashboard (`hermesx-dashboard.json`) provides 5 key monitoring panels:

1. **HTTP Request Latency P50/P95/P99** - Tracks API response times
2. **LLM Call Latency** - Monitors LLM API calls by model and tenant
3. **Circuit Breaker State** - Shows circuit breaker status over time
4. **Rate Limit Triggers** - Displays rate limit rejections by tenant
5. **Tool Execution Success Rate** - Monitors tool execution reliability

### Installation

1. **Import Dashboard**:
   - Open Grafana UI
   - Go to Dashboards → Import
   - Upload `hermesx-dashboard.json`
   - Select Prometheus datasource

2. **Configure Datasource**:
   - Ensure Prometheus is configured as `${DS_PROMETHEUS}`
   - HermesX exposes metrics at `/metrics` endpoint

### Required Metrics

The dashboard expects these metrics from HermesX:

- `http_request_duration_seconds_bucket` - HTTP latency histogram
- `llm_call_duration_seconds_bucket` - LLM call latency histogram
- `circuit_breaker_state` - Circuit breaker status
- `hermesx_rate_limit_rejected_total` - Rate limit counter
- `tool_execution_duration_seconds_count` - Tool execution counter

## Backup Automation

### Setup

1. **Make script executable**:
   ```bash
   chmod +x deploy/scripts/backup.sh
   ```

2. **Configure environment**:
   ```bash
   export REDIS_HOST=localhost
   export REDIS_PORT=6379
   export MINIO_ALIAS=hermesx
   export MINIO_BUCKET=skills
   export BACKUP_DIR=/var/backups/hermesx
   export RETENTION_DAYS=30
   ```

3. **Add to crontab**:
   ```bash
   # Edit crontab
   crontab -e
   
   # Add backup jobs
   */5 * * * * /path/to/deploy/scripts/backup.sh redis
   0 */6 * * * /path/to/deploy/scripts/backup.sh minio
   0 2 * * * /path/to/deploy/scripts/backup.sh full
   ```

### Backup Types

- **redis**: Redis RDB snapshot
- **minio**: MinIO bucket mirror
- **full**: Both Redis and MinIO + cleanup old backups
- **cleanup**: Remove backups older than RETENTION_DAYS

### Restore

To restore from backup:

1. **Redis**:
   ```bash
   # Stop Redis
   systemctl stop redis
   
   # Copy RDB file to Redis data directory
   cp /var/backups/hermesx/redis/dump_*.rdb /var/lib/redis/dump.rdb
   
   # Start Redis
   systemctl start redis
   ```

2. **MinIO**:
   ```bash
   # Mirror backup to MinIO
   mc mirror /var/backups/hermesx/minio/20260703_020000 hermesx/skills
   ```

## CI Security Integration

### govulncheck

Already configured in `.github/workflows/security.yml`:

```yaml
- name: Run govulncheck
  run: |
    go install golang.org/x/vuln/cmd/govulncheck@latest
    govulncheck ./...
```

### Trivy Container Scan

Already configured for container image scanning:

```yaml
- name: Run Trivy vulnerability scanner
  uses: aquasecurity/trivy-action@master
  with:
    scan-type: 'fs'
    scan-ref: '.'
    format: 'sarif'
    output: 'trivy-results.sarif'
```

## References

- Issue: #57
- Grafana: https://grafana.com/docs/
- Redis Backup: https://redis.io/docs/management/persistence/
- MinIO Mirror: https://min.io/docs/minio/linux/reference/minio-mc/mc-mirror.html
