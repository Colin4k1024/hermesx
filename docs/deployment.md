# 部署指南

> HermesX v2.0.0 SaaS API 的所有部署方式：Docker Compose、Kind 本地 K8s、Helm 生产部署。

## HermesX v2.0.0 部署说明

HermesX v2.0.0 相比 v1.x 版本有以下关键变化：

| 变化项 | v1.x | v2.0.0 |
|--------|------|--------|
| 二进制名称 | `hermes` | `hermesx` |
| Helm Chart 路径 | `deploy/helm/hermes-agent/` | `deploy/helm/hermesx/` |
| 默认副本数 | 1 | 2 |
| PodDisruptionBudget | 未配置 | 默认启用（minAvailable: 1） |
| HPA | 默认禁用 | 默认启用（2-10 副本） |
| OTel 配置 | 手动挂载 | 内置环境变量注入 |
| LLM Model | 单一模型 | 支持模型目录热重载 |

### 部署前置检查

- [ ] Go 1.23+ 用于从源码构建
- [ ] PostgreSQL 16+（需要 RLS 支持）
- [ ] Redis 7+（用于限流 Lua 脚本）
- [ ] Kubernetes 1.28+（若使用 Helm 部署）
- [ ] Docker 24+（若使用 Docker Compose 部署）

## Dockerfile 选择

| Dockerfile | 用途 | 产物大小 |
|------------|------|----------|
| `Dockerfile` | 通用构建，CLI + 全功能 | ~50MB |
| `Dockerfile.local` | 本地开发，Docker Compose 用 | ~50MB |
| `Dockerfile.k8s` | Kubernetes 部署，含 health probe | ~50MB |
| `Dockerfile.k8s-slim` | 精简 K8s 镜像，多阶段构建 | ~30MB |
| `Dockerfile.saas` | SaaS API 专用，含静态文件 | ~55MB |

### Dockerfile.saas 特性

```dockerfile
# 多阶段构建
# Stage 1: 编译 Go 二进制
# Stage 2: 复制二进制 + 静态文件到 distroless 基础镜像
# 包含 /static 目录供 SAAS_STATIC_DIR 使用
# 默认 CMD: ["saas-api"]
```

## Docker Compose 配置对比

| 配置 | 用途 | 包含服务 | API 端口 | 健康检查 |
|------|------|----------|----------|----------|
| `docker-compose.quickstart.yml` | 单机快速体验 | hermesx + postgres + redis + minio + bootstrap | 8080 | curl health/ready |
| `docker-compose.dev.yml` | 本地开发（Gateway 模式） | hermesx-gateway + postgres + redis + minio | 8080 | 无 |
| `docker-compose.prod.yml` | 生产部署 | hermesx-saas + postgres + redis + minio + OTel + Jaeger + Nginx LB | 8080/8081 | wget health/live |
| `docker-compose.saas.yml` | SaaS 全栈 | hermesx-saas + postgres + redis + minio + hermesx-webui + bootstrap | 8080/3000 | curl health/ready |
| `docker-compose.test.yml` | 集成测试 | postgres-test + redis-test + minio-test（tmpfs 无持久化） | 测试端口隔离 | pg_isready |
| `docker-compose.webui.yml` | 独立 Web UI | hermesx-webui（需要外部 hermesx-saas） | 3000 | 无 |

### 生产级配置（docker-compose.prod.yml）

`docker-compose.prod.yml` 是生产推荐的 Docker Compose 配置，包含以下特性：

- **OTel Collector**：接收 gRPC/HTTP OTLP，导出到 Jaeger + Prometheus
- **Jaeger**：分布式追踪后端（UI: http://localhost:16686）
- **Nginx**：3 副本负载均衡（ip_hash 会话亲和）
- **资源限制**：每个服务均配置 CPU/memory limits
- **健康检查**：所有关键服务均有 healthcheck
- **备份脚本**：postgres 容器挂载 `./scripts/backup` 目录

```bash
# 启动完整生产栈
docker compose -f docker-compose.prod.yml up -d

# 查看所有服务状态
docker compose -f docker-compose.prod.yml ps

# 查看 OTel collector 日志
docker compose -f docker-compose.prod.yml logs -f otel-collector

# 访问 Jaeger UI（追踪）
open http://localhost:16686

# 访问 Prometheus metrics
curl http://localhost:8889/metrics | grep hermesx
```

## 方式一：Docker Compose 本地开发

最快的本地开发环境搭建方式。

### 启动全部服务

```bash
# 启动 PostgreSQL 16 + Redis 7 + MinIO + Gateway
docker compose -f docker-compose.dev.yml up -d

# 查看日志
docker compose -f docker-compose.dev.yml logs -f hermes-gateway
```

### 仅启动基础设施

```bash
# 启动数据层，手动运行 hermes
docker compose -f docker-compose.dev.yml up -d postgres redis minio

# 手动启动 SaaS API
export DATABASE_URL="postgres://hermes:hermes@127.0.0.1:5432/hermes?sslmode=disable"
export HERMES_ACP_TOKEN="admin-test-token"
export SAAS_ALLOWED_ORIGINS="*"
export SAAS_STATIC_DIR="./internal/dashboard/static"
./hermesx saas-api
```

### 服务地址

| 服务 | 地址 | 说明 |
|------|------|------|
| PostgreSQL | `localhost:5432` | 用户 `hermes`，密码 `hermes`，数据库 `hermes` |
| Redis | `localhost:6379` | 无密码 |
| MinIO API | `localhost:9000` | 用户 `hermes`，密码 `hermespass` |
| MinIO Console | `localhost:9001` | Web 管理界面 |
| Hermes API | `localhost:8080` | SaaS API 端点 |

## 方式二：Kind 本地 K8s

使用 [Kind](https://kind.sigs.k8s.io/) 在本地运行 Kubernetes 集群。

### 1. 创建集群

```bash
kind create cluster --name hermes
```

### 2. 部署 PostgreSQL

```bash
kubectl apply -f deploy/kind/postgres.yaml
```

`postgres.yaml` 包含：
- PersistentVolumeClaim（1Gi）
- Deployment（PostgreSQL 16 单实例）
- Service（ClusterIP, port 5432）
- ConfigMap（初始化用户和数据库）

### 3. 构建并加载镜像

```bash
# 构建 SaaS 镜像
docker build -t hermes-agent-saas:local -f Dockerfile.saas .

# 加载到 Kind 集群
kind load docker-image hermes-agent-saas:local --name hermes
```

### 4. 安装 Helm Chart

```bash
# v2.0.0: Chart 路径变更为 hermesx/
helm install hermesx deploy/helm/hermesx/ \
  -f deploy/kind/values.local.yaml
```

`values.local.yaml` 覆盖：
- `image.pullPolicy: Never`（使用本地镜像）
- `DATABASE_URL` 指向 Kind 内 PostgreSQL Service

### 5. 验证

```bash
kubectl get pods
kubectl port-forward svc/hermesx 8080:8080

curl http://localhost:8080/health/ready
```

### OTel Collector 接入说明

`deploy/otel-collector.yaml` 是 HermesX v2.0.0 内置的可观测性收集器配置。

| 协议 | 端口 | 说明 |
|------|------|------|
| OTLP gRPC | 4317 | 接收 HermesX OTel 导出（推荐） |
| OTLP HTTP | 4318 | HTTP 方式接收 OTel 数据 |

处理器：`memory_limiter`（512MiB limit）+ `batch`（1024 batch, 5s timeout）
导出器：Jaeger（traces）+ Prometheus:8889（metrics）+ Logging（warn 级别）

```bash
# 在 Helm 中启用 OTel
helm install hermesx deploy/helm/hermesx/ \
  --set env.OTEL_EXPORTER_OTLP_ENDPOINT="http://otel-collector:4317" \
  --set env.OTEL_SERVICE_NAME="hermesx"

# 独立部署 OTel Collector
kubectl apply -f deploy/otel-collector.yaml
```

## 方式三：Helm Chart 生产部署

### Chart 结构

```
deploy/helm/hermesx/
├── Chart.yaml          # Chart 元数据
├── values.yaml         # 默认值
└── templates/          # K8s 资源模板
```

### 安装

```bash
helm install hermesx deploy/helm/hermesx/ \
  --namespace hermesx \
  --create-namespace \
  --set env.DATABASE_URL="postgres://user:pass@pg-host:5432/hermes?sslmode=require" \
  --set env.HERMES_ACP_TOKEN="production-strong-token"
```

### values.yaml 关键配置（v2.0.0）

```yaml
replicaCount: 2          # v2.0.0 默认 2（v1.x 为 1）

image:
  repository: hermesx/hermesx-saas  # v2.0.0 镜像名变更
  tag: latest
  pullPolicy: IfNotPresent

service:
  type: ClusterIP
  port: 8080

args:
  - saas-api

env:
  DATABASE_URL: ""              # 必填
  HERMES_ACP_TOKEN: ""          # 必填
  SAAS_API_PORT: "8080"
  SAAS_ALLOWED_ORIGINS: "https://your-domain.example.com"  # 生产必须设置具体域名
  SAAS_STATIC_DIR: "/static"
  REDIS_URL: "redis://redis:6379"  # v2.0.0 新增
  HERMES_API_PORT: "8081"
  LLM_API_URL: ""
  LLM_API_KEY: ""
  LLM_MODEL: ""

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

# PodDisruptionBudget — v2.0.0 默认启用
pdb:
  enabled: true
  minAvailable: 1

# 自动扩缩容 — v2.0.0 默认启用
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

# PostgreSQL 子 Chart（开发用）
postgresql:
  enabled: true
  auth:
    database: hermesx
    username: hermes
    password: hermes-dev-password
```

### 使用外部 PostgreSQL

生产环境应使用外部管理的 PostgreSQL：

```bash
helm install hermesx deploy/helm/hermesx/ \
  --set postgresql.enabled=false \
  --set env.DATABASE_URL="postgres://hermes:pass@rds-endpoint:5432/hermes?sslmode=require"
```

## 生产环境检查清单（Pre-flight Checklist）

### 安全性检查

- [ ] `HERMES_ACP_TOKEN` 使用高强度随机字符串（32+ 字符）
- [ ] `SAAS_ALLOWED_ORIGINS` 设置为具体域名，禁止 `*`
- [ ] `DATABASE_URL` 通过 Kubernetes Secret 注入（不使用明文 values）
- [ ] 启用 TLS（通过 Ingress 或 `tls.enabled`）
- [ ] API Key 定期轮换机制已建立
- [ ] Helm values.yaml 中所有 `changeme` 占位符已替换

### 高可用性检查

- [ ] `replicaCount >= 2`（v2.0.0 默认 2）
- [ ] `autoscaling.enabled: true`（v2.0.0 默认启用）
- [ ] `pdb.enabled: true`（v2.0.0 默认启用，minAvailable: 1）
- [ ] 健康探针 `liveness` 和 `readiness` 已配置
- [ ] PostgreSQL 配置主从复制或使用云托管服务（RDS/Cloud SQL）
- [ ] Redis 启用 AOF 持久化用于分布式速率限制

### 可观测性检查

- [ ] `/metrics` 端点已接入 Prometheus（指标见下表）
- [ ] `OTEL_EXPORTER_OTLP_ENDPOINT` 配置 OpenTelemetry Collector
- [ ] `OTEL_SERVICE_NAME` 设置为 `hermesx`
- [ ] 日志采集到集中式日志平台（EFK/Loki）
- [ ] 配置审计日志保留策略（建议 365 天）
- [ ] OTel Collector 已部署并验证可接收数据

### 资源规划检查

- [ ] CPU/Memory requests 和 limits 已设置（参考下方扩缩容表格）
- [ ] PostgreSQL 配置连接池（推荐 PgBouncer，>5 实例时必需）
- [ ] MinIO 使用持久化存储卷（不使用 hostPath）
- [ ] `audit_logs` / `execution_receipts` > 10M 行时考虑分区

### v2.0.0 特有检查

- [ ] Helm Chart 路径已更新为 `deploy/helm/hermesx/`（不再是 `hermes-agent/`）
- [ ] 镜像仓库地址已更新为 `hermesx/hermesx-saas`（不再是 `hermes-agent-saas`）
- [ ] `HERMES_API_PORT: "8081"` 已加入 values（v2.0.0 双端口架构）
- [ ] `REDIS_URL` 环境变量已配置（v2.0.0 限流依赖）
- [ ] `MINIO_BUCKET` 设置为 `hermesx-skills`（区分多环境）

## 方式四：多副本 HA (Docker Compose)

3 实例 + Nginx LB，适用于小规模生产或验证水平扩展能力。

```bash
cd deploy/
docker compose -f docker-compose.multi-replica.yml up -d --build
```

架构：Nginx (ip_hash) → 3× hermes instances → 共享 PG + Redis + MinIO

---

## 生产环境变量完整参考

### 必须

| Variable | Description | Example |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL 连接串 | `postgres://user:pass@host:5432/hermes?sslmode=require` |
| `HERMES_API_KEY` | API 认证 Bearer token | `sk-prod-xxxxx` |
| `HERMES_API_KEY_LLM` | LLM Provider API key | `sk-...` |
| `HERMES_PROVIDER` | LLM Provider | `openai`, `anthropic`, `gemini` |
| `HERMES_MODEL` | 默认模型 | `gpt-4o`, `claude-sonnet-4-20250514` |

### 基础设施（推荐）

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_URL` | — | Redis 连接，启用分布式限流（v2.0.0 新增） |
| `MINIO_ENDPOINT` | — | MinIO/S3 endpoint |
| `MINIO_ACCESS_KEY` | — | MinIO access key |
| `MINIO_SECRET_KEY` | — | MinIO secret key |
| `MINIO_BUCKET` | `hermes-skills` | Skills bucket |

### SaaS API（v2.0.0）

| Variable | Default | Description |
|----------|---------|-------------|
| `HERMES_ACP_TOKEN` | — | 静态管理员 Token（必填） |
| `SAAS_API_PORT` | `8080` | SaaS API 端口 |
| `SAAS_ALLOWED_ORIGINS` | `*` | CORS 允许的源（生产必须设置具体域名） |
| `SAAS_STATIC_DIR` | — | 静态文件目录 |
| `HERMES_API_PORT` | `8081` | HTTP API 端口（v2.0.0 新增） |
| `HERMES_API_KEY` | — | API 认证 Token（v2.0.0 新增） |

### Agent 运行时

| Variable | Default | Description |
|----------|---------|-------------|
| `HERMES_INSTANCE_ID` | hostname | HA 实例标识 |
| `HERMES_MAX_ITERATIONS` | `20` | Agent 最大迭代次数 |
| `HERMES_MAX_TOKENS` | `4096` | 最大响应 token |
| `HERMES_BASE_URL` | provider default | 自定义 LLM endpoint |
| `HERMES_DEBUG` | `false` | Debug 日志 |

### 可观测性（v2.0.0 增强）

| Variable | Default | Description |
|----------|---------|-------------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | — | OTel Collector gRPC/HTTP |
| `OTEL_EXPORTER_OTLP_INSECURE` | `false` | 禁用 OTel TLS |
| `OTEL_SERVICE_NAME` | `hermesx` | 服务名（v2.0.0 默认值变更） |

---

## Prometheus 指标

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

### 告警建议

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

## 备份恢复

### 自动备份

```bash
./scripts/backup/backup.sh /backup
# 输出: /backup/hermes_YYYYMMDD_HHMMSS.sql.gz
# BACKUP_RETENTION_DAYS=7 (默认保留 7 天)
```

### 恢复

```bash
./scripts/backup/restore.sh /backup/hermes_20260507_120000.sql.gz
# 单事务恢复 + 自动运行 pending migrations
```

### PITR

生产环境建议启用 WAL archiving 实现 < 5 min RPO。配置模板见 `deploy/pitr/`。

---

## 水平扩展

HermesX 实例无状态，所有持久化状态在 PG + Redis 中。

| 负载 | CPU/实例 | 内存/实例 | 实例数 |
|------|----------|-----------|--------|
| < 100 req/s | 1 core | 512MB | 1-2 |
| 100-500 req/s | 2 cores | 1GB | 3-5 |
| 500+ req/s | 4 cores | 2GB | 5+ |

数据库扩展建议:
- > 5 实例时使用 PgBouncer 连接池
- `audit_logs` / `execution_receipts` > 10M 行时考虑分区

---

## 安全加固

### 认证体系

1. **API Key**: SHA-256 hash 存储，支持 scopes + expiry + rotation
2. **JWT**: 签名验证 + claims 提取 tenant_id
3. **Static Token**: 单租户部署的简单 Bearer token

### 行级安全 (RLS)

所有租户数据表启用 PostgreSQL RLS。每个事务通过 `SET LOCAL app.current_tenant` 设置上下文——即使应用层有 bug，数据库层面也阻止跨租户访问。

### 网络安全

- API 仅绑定内部网络，通过 reverse proxy + TLS 暴露
- PG/Redis/MinIO 禁止公网暴露
- 生产环境 MinIO 启用 TLS

---

## 回滚策略

### 应用回滚

```bash
docker compose -f docker-compose.prod.yml up -d --no-build  # 使用上一个镜像
# 或
docker service update --image ghcr.io/org/hermes:previous-tag hermes
```

### 数据库回滚

Migrations 仅前向。回滚步骤:
1. 从最近备份恢复
2. 部署上一个应用版本
3. 验证数据完整性

### 回滚触发条件

- Error rate > 5% 持续 5 分钟
- P95 latency > 30s 持续 5 分钟
- 数据完整性告警（跨租户数据泄漏）
- 发布后发现 Critical 安全漏洞

---

## 相关文档

- [快速开始](saas-quickstart.md) — 本地开发环境（含 binary/Docker 对比）
- [配置指南](configuration.md) — 所有环境变量
- [可观测性](observability.md) — 监控和追踪
- [架构概览](architecture.md) — 系统设计
- [企业加固](enterprise-hardening.md) — 安全与合规
- [Changelog v2.0.0](CHANGELOG.md) — v2.0.0 变更记录（含上游吸收）
