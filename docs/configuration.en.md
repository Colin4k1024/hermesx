# Configuration Guide

> Complete reference for all HermesX environment variables and configuration options.

## Configuration Priority

```
Environment variables > config.yaml > defaults
```

## SaaS Service Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DATABASE_URL` | Yes | - | PostgreSQL connection string, format: `postgres://user:pass@host:5432/dbname?sslmode=disable` |
| `HERMES_ACP_TOKEN` | Yes | - | Static admin Bearer Token for authenticating admin endpoints |
| `HERMES_BOOTSTRAP_RATE_LIMIT_RPM` | No | `5` | Per-IP attempts per minute for `POST /admin/v1/bootstrap` |
| `SAAS_API_PORT` | No | `8080` | SaaS API server port |
| `SAAS_ALLOWED_ORIGINS` | No | - (CORS disabled) | CORS allowed origins; `*` for all, or comma-separated domain list |
| `SAAS_STATIC_DIR` | No | - (no static files) | Static files directory path, e.g. `./internal/dashboard/static` |
| `HERMES_API_PORT` | No | `8081` | OpenAI-compatible adapter port |
| `HERMES_API_KEY` | No | - | Bearer Token for the OpenAI-compatible adapter |
| `HERMES_ACP_PORT` | No | - | ACP server port (not started if not set) |

## LLM Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `LLM_API_URL` | No | - | LLM API endpoint URL |
| `LLM_API_KEY` | No | - | LLM API authentication key |
| `LLM_MODEL` | No | - | Default LLM model name |
| `HERMES_MODEL` | No | - | Default model for CLI mode |
| `HERMES_PROVIDER` | No | - | LLM provider (openai / anthropic / auto) |
| `HERMES_BASE_URL` | No | - | LLM API Base URL for CLI mode |
| `HERMES_API_KEY_LLM` | No | - | LLM API Key for CLI mode |
| `HERMES_API_MODE` | No | - | API protocol mode (openai / anthropic) |
| `HERMES_MAX_ITERATIONS` | No | `20` | Maximum agent iterations |
| `HERMES_MAX_TOKENS` | No | `4096` | Maximum tokens per response |

## Storage Configuration

### PostgreSQL

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DATABASE_URL` | Yes (SaaS mode) | - | PostgreSQL connection string |
| `DATABASE_DRIVER` | No | `postgres` | Database driver type |

### Redis

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `REDIS_URL` | No | - | Redis connection string, used for distributed rate limiting |

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
| `HERMES_FILE_STATE` | No | - | Enable file state tracking |
| `HERMES_GATEWAY_URL` | No | - | Gateway URL for messaging platform integration |

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

## Gateway Platforms

The message gateway (`hermes gateway`) supports multiple platform adapters:

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

## Terminal and SSH

| Variable | Description |
|----------|-------------|
| `TERMINAL_CWD` | Terminal tool working directory |
| `SSH_PASSWORD` | SSH authentication password |

## CLI Mode Configuration

The following variables only apply in CLI interactive mode:

| Variable | Description |
|----------|-------------|
| `HERMES_HOME` | Hermes home directory (default `~/.hermes`) |
| `HERMES_PROFILE` | Current profile name |
| `HERMES_DISPLAY_THEME` | CLI theme |
| `HERMES_TERMINAL_BACKEND` | Terminal backend type (local/docker/ssh/modal/daytona/singularity/persistent) |

## config.yaml Reference

The main configuration file for CLI mode, located at `~/.hermes/config.yaml`:

```yaml
# LLM configuration
model: "gpt-4o"
provider: "openai"
base_url: "https://api.openai.com/v1"
api_mode: "openai"

# Agent behavior
max_iterations: 20
max_tokens: 4096
context_compression: true
compression_threshold: 80000
memory_curator: true
max_memories: 100
self_improve: true
review_interval: 10
max_insights: 50

# Terminal
terminal:
  backend: "local"
  timeout: 30

# Display
display:
  theme: "default"
  show_tool_calls: true
  show_reasoning: false

# Model routing
smart_routing:
  enabled: false
  cheap_model: "gpt-4o-mini"
  threshold: 0.3

# Fallback chain
fallback:
  enabled: true
  models:
    - "gpt-4o"
    - "claude-sonnet-4-20250514"
```

## Docker Compose Configuration Example

`docker-compose.dev.yml` provides a complete local development environment:

```yaml
services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_DB: hermes
      POSTGRES_USER: hermes
      POSTGRES_PASSWORD: hermes
    ports:
      - "5432:5432"

  redis:
    image: redis:7
    ports:
      - "6379:6379"

  minio:
    image: minio/minio:latest
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: hermes
      MINIO_ROOT_PASSWORD: hermespass
    ports:
      - "9000:9000"   # API
      - "9001:9001"   # Console
```

## Security Notes

- `HERMES_ACP_TOKEN` is used for admin authentication; use a strong password in production
- `POST /admin/v1/bootstrap` is rate-limited to 5 RPM per source IP by default; additional Nginx/Ingress-level rate limiting is still recommended for public-facing deployments
- Passwords in `DATABASE_URL` should be injected via Kubernetes Secrets or Vault
- API Keys are stored as SHA-256 hashes and cannot be reversed to retrieve the original value
- Set `SAAS_ALLOWED_ORIGINS` to specific domain names; avoid using `*` in production
- MinIO credentials should be managed independently from PostgreSQL credentials

## Related Documentation

- [Getting Started](saas-quickstart.md) — 5-minute quickstart
- [Authentication](authentication.md) — Auth Chain and RBAC details
- [Deployment Guide](deployment.md) — Production deployment
