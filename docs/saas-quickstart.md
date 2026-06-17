# SaaS 快速开始

> 启动 HermesX SaaS API 服务。HermesX 对外只提供 SaaS API、内嵌 WebUI 和 SaaS 部署路径。

## 前置条件

| 依赖 | 版本 | 说明 |
|------|------|------|
| Docker + Docker Compose | 24+ | 推荐的 SaaS 栈启动方式 |
| PostgreSQL | 16+ | 直接运行二进制时必需 |
| Redis | 7+ | 分布式限流和调度器锁 |
| MinIO/S3 | 兼容 S3 | 租户 Skills 和 Soul 存储 |
| Go | 1.25+ | 仅直接构建二进制时需要 |

## 推荐方式：SaaS Compose 栈

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

SaaS API 会从同一个服务提供内嵌 WebUI：

- 用户入口：`http://localhost:18080/`
- 管理控制台：`http://localhost:18080/admin.html`
- OpenAPI：`http://localhost:18080/v1/openapi`

## 二进制 SaaS 服务

仅在 PostgreSQL、Redis、MinIO/S3 已经可用时使用。

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

不带 `saas-api` 的根命令，以及旧 chat、setup、gateway 子命令不再是受支持的公开接口。

## 创建租户和 API Key

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

保存返回的 `hk_...` key；它只会显示一次。

## 发送 Chat 请求

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

## 沙箱默认行为

`execute_code` 不再隐式使用宿主机执行。请显式配置：

| 模式 | 用途 |
|------|------|
| `SANDBOX_MODE=k8s-job` | 生产 SaaS 部署 |
| `SANDBOX_MODE=docker` | 容器隔离执行 |
| `SANDBOX_MODE=local` + `HERMESX_ALLOW_LOCAL_SANDBOX=true` | 仅本地 SaaS 开发；生产环境会被拒绝 |

## 相关文档

- [部署指南](deployment.md)
- [配置指南](configuration.md)
- [API 参考](api-reference.md)
