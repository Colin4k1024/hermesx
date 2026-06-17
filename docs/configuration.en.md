# Configuration Guide

> Complete reference for all HermesX environment variables and configuration options.

## Configuration Priority

```
Environment variables > service defaults
```

SaaS deployments should inject configuration through environment variables or secrets. Legacy local config files are not part of the supported public runtime.

## SaaS Service Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DATABASE_URL` | Yes | - | Database connection string. PostgreSQL uses `postgres://...`; MySQL uses `user:pass@tcp(host:3306)/dbname?parseTime=true&charset=utf8mb4&loc=UTC` |
| `HERMES_ACP_TOKEN` | Yes | - | Static admin Bearer Token for authenticating admin endpoints |
| `HERMES_BOOTSTRAP_RATE_LIMIT_RPM` | No | `5` | Per-IP attempts per minute for `POST /admin/v1/bootstrap` |
| `SAAS_API_PORT` | No | `8080` | SaaS API server port |
| `SAAS_ALLOWED_ORIGINS` | No | - (CORS disabled) | CORS allowed origins; `*` for all, or comma-separated domain list |
| `SAAS_STATIC_DIR` | No | - (no static files) | Static files directory path. The release image uses `/static` for the embedded WebUI |
| `HERMES_API_PORT` | No | `8081` | OpenAI-compatible adapter port |
| `HERMES_API_KEY` | No | - | Bearer Token for the OpenAI-compatible adapter |
| `HERMES_ACP_PORT` | No | - | ACP server port (not started if not set) |

## LLM Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `LLM_API_URL` | No | - | LLM API endpoint URL |
| `LLM_API_KEY` | No | - | LLM API authentication key |
| `LLM_MODEL` | No | - | Default LLM model name |
| `HERMES_MODEL` | No | - | Default model for the SaaS agent runtime |
| `HERMES_PROVIDER` | No | - | LLM provider (openai / anthropic / auto) |
| `HERMES_BASE_URL` | No | - | LLM API Base URL for the SaaS agent runtime |
| `HERMES_API_KEY_LLM` | No | - | LLM API Key for the SaaS agent runtime |
| `HERMES_API_MODE` | No | - | API protocol mode (openai / anthropic) |
| `HERMES_MAX_ITERATIONS` | No | `20` | Maximum agent iterations |
| `HERMES_MAX_TOKENS` | No | `4096` | Maximum tokens per response |

## Storage Configuration

### Database

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DATABASE_URL` | Yes (SaaS mode) | - | PostgreSQL or MySQL connection string |
| `DATABASE_DRIVER` | No | `postgres` | Database driver type: `postgres` or `mysql` |

### Redis

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `REDIS_URL` | No | - | Redis connection string, used for distributed rate limiting and Cron Scheduler distributed locks |

### Cron Scheduler

Distributed scheduled task execution requires `REDIS_URL` to be configured. The scheduler uses Redis distributed locks for multi-pod mutual exclusion.

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SCHEDULER_POLL_INTERVAL` | No | `30s` | Polling interval for syncing cron jobs from PG |
| `SCHEDULER_EXEC_TIMEOUT` | No | `5m` | Single task execution timeout |
| `SCHEDULER_LOCK_TTL` | No | `12m` | Redis distributed lock TTL (should exceed longest task duration) |

The scheduler starts automatically when `REDIS_URL` is available. If Redis is unavailable, scheduler initialization fails gracefully without blocking the main service (non-fatal).

### MinIO / S3

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `MINIO_ENDPOINT` | No | - | MinIO service address (e.g. `localhost:9000`) |
| `MINIO_ACCESS_KEY` | No | - | MinIO access key |
| `MINIO_SECRET_KEY` | No | - | MinIO secret key |
| `MINIO_BUCKET` | No | `hermes-skills` | MinIO bucket name |
| `MINIO_USE_SSL` | No | `false` | Whether to use SSL for MinIO connections |
| `BUNDLED_SKILLS_DIR` | No | `skills` | Built-in skills directory path for tenant auto-provisioning |

When MinIO is configured, skills are automatically synced at:
- **Tenant creation**: All skills from `BUNDLED_SKILLS_DIR` are asynchronously copied to the tenant's MinIO prefix, and a default SOUL.md is generated
- **Service startup**: All tenants are iterated for incremental sync (new/updated built-in skills are synced, user-modified skills are skipped)

## Observability Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | No | - (tracing disabled) | OpenTelemetry OTLP gRPC endpoint |
| `OTEL_EXPORTER_OTLP_INSECURE` | No | `false` | Whether to use insecure connection |
| `OTEL_SERVICE_NAME` | No | `hermes-agent` | OTel service name |

## Agent Behavior Configuration (v1.4.0+)

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `HERMES_CONTEXT_COMPRESSION` | No | `true` | Enable automatic context compression |
| `HERMES_COMPRESSION_THRESHOLD` | No | `80000` | Token threshold to trigger compression |
| `HERMES_MEMORY_CURATOR` | No | `true` | Enable autonomous memory curation |
| `HERMES_MAX_MEMORIES` | No | `100` | Maximum memory entries per tenant |
| `HERMES_SELF_IMPROVE` | No | `true` | Enable self-improvement loop |
| `HERMES_REVIEW_INTERVAL` | No | `10` | Self-review trigger interval (conversation turns) |
| `HERMES_MAX_INSIGHTS` | No | `50` | Maximum improvement insights to store |

## Debug and Runtime

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `HERMES_DEBUG` | No | `false` | Enable debug logging (LLM request/response details) |
| `HERMES_DEFAULT_MODEL` | No | - | Global default model (config fallback) |
| `HERMES_FILE_STATE` | No | - | Enable file state tracking for SaaS agent execution |

## Memory System

Hermes supports multiple external memory providers, configured as needed:

### Honcho

| Variable | Description |
|----------|-------------|
| `HONCHO_API_KEY` | Honcho memory service API Key |
| `HONCHO_APP_ID` | Honcho application ID |
| `HONCHO_BASE_URL` | Honcho service URL |
| `HONCHO_USER_ID` | Honcho user identifier |

### Mem0

| Variable | Description |
|----------|-------------|
| `MEM0_API_KEY` | Mem0 memory service API Key |
| `MEM0_BASE_URL` | Mem0 service URL |

### Supermemory

| Variable | Description |
|----------|-------------|
| `SUPERMEMORY_API_KEY` | Supermemory service API Key |
| `SUPERMEMORY_BASE_URL` | Supermemory service URL |

## Channel and Messaging Integrations

Channel and messaging integrations are configured through the SaaS service. The former standalone gateway subcommand is no longer a supported public runtime.

| Variable | Description |
|----------|-------------|
| `TELEGRAM_BOT_TOKEN` | Telegram Bot Token |
| `DISCORD_BOT_TOKEN` | Discord Bot Token |
| `SLACK_APP_TOKEN` | Slack App-Level Token |
| `SLACK_BOT_TOKEN` | Slack Bot Token |
| `DMWORK_API_URL` | DmWork API endpoint |
| `DMWORK_BOT_TOKEN` | DmWork Bot Token |

## Auxiliary Vision Model

For image recognition and other multimodal scenarios:

| Variable | Default | Description |
|----------|---------|-------------|
| `AUXILIARY_VISION_API_KEY` | Falls back to `OPENROUTER_API_KEY` | Vision LLM API Key |
| `AUXILIARY_VISION_BASE_URL` | OpenRouter endpoint | Vision LLM Base URL |
| `AUXILIARY_VISION_MODEL` | - | Vision model name |

## Tool API Keys

API keys required for various tool integrations:

| Variable | Description |
|----------|-------------|
| `OPENROUTER_API_KEY` | OpenRouter API Key |
| `GEMINI_API_KEY` | Google Gemini API Key |
| `GOOGLE_API_KEY` | Google API Key (search, etc.) |
| `EXA_API_KEY` | Exa search API Key |
| `FIRECRAWL_API_KEY` | Firecrawl web scraping API Key |
| `FAL_KEY` | fal.ai image generation API Key |
| `ELEVENLABS_API_KEY` | ElevenLabs TTS API Key |
| `OSV_ENDPOINT` | Open Source Vulnerabilities API endpoint |

## Email Configuration

| Variable | Description |
|----------|-------------|
| `EMAIL_FROM` | Sender address |
| `EMAIL_USERNAME` | Email account |
| `EMAIL_PASSWORD` | Email password |
| `EMAIL_SMTP_HOST` | SMTP server address |
| `EMAIL_SMTP_PORT` | SMTP port |
| `EMAIL_IMAP_HOST` | IMAP server address |

## Browser Tools

| Variable | Default | Description |
|----------|---------|-------------|
| `BROWSER_BACKEND` | `local` | Browser backend (`local` or `browserbase`) |
| `BROWSER_CDP_URL` | - | Chrome DevTools Protocol endpoint |
| `HASS_URL` | - | Home Assistant URL |
| `HASS_TOKEN` | - | Home Assistant long-lived access token |

## Sandbox Execution Mode

`SANDBOX_MODE` controls the execution backend for the `execute_code` tool:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SANDBOX_MODE` | Yes when `execute_code` is enabled | - | Code execution backend mode |

Available modes:

| Mode | Description | Use Case |
|------|-------------|----------|
| `local` | Execute directly on host only when `HERMESX_ALLOW_LOCAL_SANDBOX=true` and the environment is not production | Explicit local SaaS development only |
| `docker` | Execute via Docker container | Container isolation, requires Docker daemon |
| `k8s-job` | Execute via Kubernetes Job | Production environments, no DinD or privileged containers needed |

If `SANDBOX_MODE` is unset, `execute_code` returns an error instead of falling back to host execution. `local` mode is blocked when any of `HERMES_ENV`, `HERMESX_ENV`, `APP_ENV`, or `GO_ENV` is `production`.

### K8s Job Mode Configuration

When `SANDBOX_MODE=k8s-job`, the following optional variables are available:

| Variable | Default | Description |
|----------|---------|-------------|
| `K8S_JOB_NAMESPACE` | `default` | Namespace for Job execution |
| `K8S_JOB_IMAGE` | `ubuntu:latest` | Container image for the Job |
| `K8S_JOB_CPU_LIMIT` | `500m` | CPU resource limit |
| `K8S_JOB_MEMORY_LIMIT` | `256Mi` | Memory resource limit |
| `K8S_JOB_SERVICE_ACCOUNT` | - (uses default SA) | ServiceAccount for the Job Pod |

> Note: K8s Job mode requires `kubectl` to be configured and able to reach the target cluster. When deployed inside a cluster, this is typically handled automatically via in-cluster config.

## Local SaaS Development

Local development should still run the SaaS service shape:

```bash
docker compose -f docker-compose.saas.yml up -d --build
# or, with external backing services:
SAAS_STATIC_DIR=./webui/dist ./hermesx saas-api
```

Do not document or rely on former local assistant, gateway runtime, or separate frontend container configuration.

## Security Notes

- `HERMES_ACP_TOKEN` is used for admin authentication; use a strong password in production
- `POST /admin/v1/bootstrap` is rate-limited to 5 RPM per source IP by default; additional Nginx/Ingress-level rate limiting is still recommended for public-facing deployments
- Passwords in `DATABASE_URL` should be injected via Kubernetes Secrets or Vault
- API Keys are stored as SHA-256 hashes and cannot be reversed to retrieve the original value
- Set `SAAS_ALLOWED_ORIGINS` to specific domain names; avoid using `*` in production
- Pin production infrastructure images by release tag or digest; do not deploy `:latest`
- MinIO credentials should be managed independently from PostgreSQL credentials

## Related Documentation

- [Getting Started](saas-quickstart.md) — 5-minute quickstart
- [Authentication](authentication.md) — Auth Chain and RBAC details
- [Deployment Guide](deployment.md) — Production deployment
