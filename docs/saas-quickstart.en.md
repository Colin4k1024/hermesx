# SaaS Quickstart

> Start the HermesX SaaS API service. HermesX exposes only the SaaS API, embedded WebUI, and SaaS deployment paths.

## Prerequisites

| Dependency | Version | Notes |
|------------|---------|-------|
| Docker + Docker Compose | 24+ | Recommended SaaS stack path |
| PostgreSQL | 16+ | Required when running the binary directly |
| Redis | 7+ | Required for distributed rate limiting and scheduler locks |
| MinIO/S3 | Compatible | Required for tenant skills and soul storage |
| Go | 1.25+ | Only required when building the binary directly |

## Recommended: SaaS Compose Stack

```bash
git clone https://github.com/Colin4k1024/hermesx.git
cd hermesx

export POSTGRES_DB=hermes
export POSTGRES_USER=hermes
export POSTGRES_PASSWORD="$(openssl rand -hex 16)"
export MINIO_ACCESS_KEY=hermes
export MINIO_SECRET_KEY="$(openssl rand -hex 16)"
export HERMES_ACP_TOKEN="$(openssl rand -hex 32)"
export HERMES_API_KEY="$(openssl rand -hex 32)"
export SAAS_ALLOWED_ORIGINS="http://localhost:18080"
export HERMES_PROVIDER=openai
export HERMES_BASE_URL="https://api.openai.com/v1"
export HERMES_API_KEY_LLM="replace-me"

docker compose -f docker-compose.saas.yml up -d --build
curl http://localhost:18080/health/ready
```

The SaaS API serves the embedded WebUI from the same service. Open:

- User portal: `http://localhost:18080/`
- Admin console: `http://localhost:18080/admin.html`
- OpenAPI: `http://localhost:18080/v1/openapi`

## Binary SaaS Service

Use this only when the backing services are already available.

```bash
go build -o hermesx ./cmd/hermesx/

export DATABASE_URL="postgres://user:pass@127.0.0.1:5432/hermes?sslmode=disable"
export REDIS_URL="redis://127.0.0.1:6379"
export MINIO_ENDPOINT="127.0.0.1:9000"
export MINIO_ACCESS_KEY="replace-me"
export MINIO_SECRET_KEY="replace-me"
export MINIO_BUCKET="hermes-skills"
export HERMES_ACP_TOKEN="$(openssl rand -hex 32)"
export SAAS_ALLOWED_ORIGINS="http://localhost:8080"
export SAAS_STATIC_DIR="./webui/dist"

./hermesx saas-api
```

Root invocation without `saas-api` and the former chat, setup, and gateway subcommands are no longer supported public interfaces.

## Create a Tenant and API Key

```bash
curl -X POST http://localhost:18080/v1/tenants \
  -H "Authorization: Bearer $HERMES_ACP_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My Company",
    "plan": "pro",
    "rate_limit_rpm": 120,
    "max_sessions": 50
  }'

curl -X POST http://localhost:18080/v1/api-keys \
  -H "Authorization: Bearer $HERMES_ACP_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "app-key",
    "tenant_id": "<tenant-id>",
    "roles": ["user"]
  }'
```

Save the returned `hk_...` key. It is shown only once.

## Send a Chat Request

```bash
curl -X POST http://localhost:18080/v1/chat/completions \
  -H "Authorization: Bearer hk_xxxx..." \
  -H "Content-Type: application/json" \
  -d '{
    "model": "mock",
    "messages": [
      {"role": "user", "content": "Hello, who am I?"}
    ]
  }'
```

## Sandbox Default

`execute_code` has no implicit host execution default. Configure one of:

| Mode | Use |
|------|-----|
| `SANDBOX_MODE=k8s-job` | Production SaaS deployments |
| `SANDBOX_MODE=docker` | Isolated container execution |
| `SANDBOX_MODE=local` + `HERMESX_ALLOW_LOCAL_SANDBOX=true` | Explicit local SaaS development only; blocked in production envs |

## Related Docs

- [Deployment guide](deployment.en.md)
- [Configuration guide](configuration.en.md)
- [API reference](api-reference.en.md)
