# SaaS 模式快速开始

> 5 分钟内启动 HermesX SaaS 多租户 API 服务器（v2.0.0）。

## 前置条件

| 依赖 | 版本 | 说明 |
|------|------|------|
| Go | 1.22+ | 编译 hermes 二进制 |
| PostgreSQL | 16+ | 多租户数据存储 |
| Docker + Docker Compose | 最新版 | 可选，一键启动基础设施 |

## 方式一：纯 Binary 快速开始（无需 Docker）

无需 Docker，直接编译运行，适合快速验证和本地开发。

### 1. 构建二进制

```bash
git clone https://github.com/Colin4k1024/hermesx.git
cd hermesx
go build -o hermesx ./cmd/hermes/
```

### 2. 配置向导（可选）

```bash
./hermesx setup
# 交互式配置 LLM API Key、provider 等
```

### 3. 启动服务

```bash
export DATABASE_URL="postgres://hermes:hermes@127.0.0.1:5432/hermes?sslmode=disable"
export HERMES_ACP_TOKEN="admin-test-token"
export SAAS_ALLOWED_ORIGINS="*"
export SAAS_STATIC_DIR="./internal/dashboard/static"

# 启动 SaaS API 服务器
./hermesx saas-api
```

> 注意：需要先有可用的 PostgreSQL 16+ 实例。可通过 `brew install postgresql@16 && brew services start postgresql@16 && createdb hermes` 快速安装。

### 4. 单命令验证

```bash
# 快速 Chat（使用 mock 模型，无需 LLM API Key）
./hermesx chat --model mock "Hello, who are you?"
```

## 方式二：Docker Compose 快速启动（推荐）

```bash
# 1. 克隆仓库
git clone https://github.com/Colin4k1024/hermesx.git
cd hermesx

# 2. 构建二进制
go build -o hermesx ./cmd/hermes/

# 3. 启动基础设施（PostgreSQL 16 + Redis 7 + MinIO）
docker compose -f docker-compose.dev.yml up -d postgres redis minio

# 4. 等待服务就绪
docker compose -f docker-compose.dev.yml ps  # 确认 healthy 状态

# 5. 导出环境变量
export DATABASE_URL="postgres://hermes:hermes@127.0.0.1:5432/hermes?sslmode=disable"
export HERMES_ACP_TOKEN="admin-test-token"
export SAAS_ALLOWED_ORIGINS="*"
export SAAS_STATIC_DIR="./internal/dashboard/static"

# 6. 启动 SaaS API 服务器
./hermesx saas-api
```

启动成功后输出：

```
SaaS API server running  port=8080
  openapi=http://localhost:8080/v1/openapi
  admin=http://localhost:8080/admin.html
  health_live=http://localhost:8080/health/live
  health_ready=http://localhost:8080/health/ready
```

## 方式二：手动配置

### 1. 安装并启动 PostgreSQL

```bash
# macOS
brew install postgresql@16
brew services start postgresql@16

# 创建数据库
createdb hermes
```

### 2. 构建并启动

```bash
go build -o hermesx ./cmd/hermes/

export DATABASE_URL="postgres://$(whoami)@127.0.0.1:5432/hermes?sslmode=disable"
export HERMES_ACP_TOKEN="your-secret-admin-token"

./hermesx saas-api
```

数据库表会在首次启动时自动创建（27 个 migration 自动执行）。

## Docker Compose 配置对比

| 配置 | 用途 | 包含服务 |
|------|------|----------|
| `docker-compose.quickstart.yml` | 单机快速体验 | hermesx + postgres + redis + minio + bootstrap |
| `docker-compose.dev.yml` | 本地开发（Gateway 模式） | hermesx-gateway + postgres + redis + minio |
| `docker-compose.prod.yml` | 生产部署 | hermesx-saas + postgres + redis + minio + OTel + Jaeger + Nginx LB |
| `docker-compose.saas.yml` | SaaS 全栈 | hermesx-saas + postgres + redis + minio + hermesx-webui + bootstrap |
| `docker-compose.test.yml` | 集成测试 | postgres-test + redis-test + minio-test（tmpfs 无持久化） |
| `docker-compose.webui.yml` | 独立 Web UI | hermesx-webui（需要外部 hermesx-saas） |

默认凭证（开发/快速体验用，生产环境必须替换）：

| 服务 | 用户名 | 密码 | 数据库/Bucket |
|------|--------|------|---------------|
| PostgreSQL | `hermes` | `hermes` | `hermes` |
| Redis | — | 无密码 | — |
| MinIO | `hermes` | `hermesxpass` | `hermes-skills` |
| HermesX Admin Token | — | `dev-bootstrap-token` | — |

## 验证服务

```bash
# 健康检查
curl http://localhost:8080/health/live
# {"status":"ok"}

curl http://localhost:8080/health/ready
# {"status":"ready","database":"ok"}

# 查看当前身份
curl http://localhost:8080/v1/me \
  -H "Authorization: Bearer admin-test-token"

# 查看 OpenAPI 文档
curl http://localhost:8080/v1/openapi
```

## 创建第一个租户

```bash
# 1. 创建租户
curl -X POST http://localhost:8080/v1/tenants \
  -H "Authorization: Bearer admin-test-token" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My Company",
    "plan": "pro",
    "rate_limit_rpm": 120,
    "max_sessions": 50
  }'
# 返回: {"id":"<tenant-id>", "name":"My Company", ...}

# 2. 为该租户创建 API Key
curl -X POST http://localhost:8080/v1/api-keys \
  -H "Authorization: Bearer admin-test-token" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "dev-key",
    "tenant_id": "<tenant-id>",
    "roles": ["user"]
  }'
# 返回: {"id":"...", "key":"hk_xxxx...", "prefix":"hk_xxxx"}
# 注意：key 仅在创建时返回一次，请妥善保存
```

## 发送 Chat 请求

```bash
# 使用刚创建的 API Key
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

响应中会包含租户标识，确认请求已正确路由到对应租户。

## 访问管理面板

打开浏览器访问：

- **管理面板**: http://localhost:8080/admin.html
- **隔离测试页面**: http://localhost:8080/isolation-test.html

管理面板提供租户管理、API Key 管理、使用量查看等自助功能。

## 环境变量速查

| 变量 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `DATABASE_URL` | 是 | - | PostgreSQL 连接字符串 |
| `HERMES_ACP_TOKEN` | 是 | - | 静态管理员 Token |
| `SAAS_API_PORT` | 否 | `8080` | API 服务端口 |
| `SAAS_ALLOWED_ORIGINS` | 否 | - | CORS 允许的源，`*` 表示全部 |
| `SAAS_STATIC_DIR` | 否 | - | 静态文件目录 |

完整配置参考: [configuration.md](configuration.md)

## Enterprise SaaS Demo（11 步完整演示）

`examples/enterprise-saas-demo/demo.sh` 演示 HermesX v2.0.0 企业级能力的完整生命周期。

### 前置条件

```bash
# 确保 SaaS API 已启动（任意启动方式均可）
./hermesx saas-api &
# 或
docker compose -f docker-compose.prod.yml up -d hermesx-saas
```

### 运行完整演示

```bash
# 设置环境变量（可选，有默认值）
export HERMES_URL="${HERMES_URL:-http://localhost:8080}"
export HERMES_ADMIN_TOKEN="${HERMES_ADMIN_TOKEN:-admin-test-token}"

# 运行全部 11 步
./examples/enterprise-saas-demo/demo.sh
```

### 分步运行

```bash
./examples/enterprise-saas-demo/demo.sh step1   # 创建租户
./examples/enterprise-saas-demo/demo.sh step2  # 创建 API Key
./examples/enterprise-saas-demo/demo.sh step3  # 验证身份
./examples/enterprise-saas-demo/demo.sh step4  # 创建会话
./examples/enterprise-saas-demo/demo.sh step5  # Chat Completion
./examples/enterprise-saas-demo/demo.sh step6  # 执行回执审计
./examples/enterprise-saas-demo/demo.sh step7  # 用量计量
./examples/enterprise-saas-demo/demo.sh step8  # 审计日志
./examples/enterprise-saas-demo/demo.sh step9  # GDPR 数据导出
./examples/enterprise-saas-demo/demo.sh step10  # 健康检查
./examples/enterprise-saas-demo/demo.sh step11  # GDPR 数据删除（dry-run）
```

### 演示覆盖的企业能力

| 步骤 | 能力 | 说明 |
|------|------|------|
| step1 | 多租户隔离 | 创建企业租户，配置套餐和资源限制 |
| step2 | 凭证管理 | 创建带作用域的 API Key |
| step3 | 身份验证 | 通过 API Key 验证身份上下文 |
| step4 | 会话管理 | 创建带元数据的 Chat 会话 |
| step5 | Agent 执行 | Chat Completion 调用 |
| step6 | 执行回执 | 可审计的工具调用记录 |
| step7 | 用量计量 | Token 使用量和成本归因 |
| step8 | 审计日志 | 合规级操作审计追踪 |
| step9 | GDPR 导出 | 租户全量数据导出 |
| step10 | 健康检查 | 运行时就绪/存活探测 |
| step11 | GDPR 删除 | 租户数据完全删除（dry-run） |

## 快速验证清单

安装完成后，按以下清单逐项验证：

### 基础验证

- [ ] `./hermesx --version` 输出 v2.0.0
- [ ] `curl http://localhost:8080/health/live` 返回 `{"status":"ok"}`
- [ ] `curl http://localhost:8080/health/ready` 返回 `{"status":"ready","database":"ok"}`
- [ ] `curl http://localhost:8080/v1/me -H "Authorization: Bearer admin-test-token"` 返回身份信息

### 多租户验证

- [ ] 成功创建租户并获得 tenant_id
- [ ] 成功创建 API Key 并获得 key
- [ ] 使用 API Key 的 Chat 请求正确路由到对应租户

### 可观测性验证（生产部署）

- [ ] `curl http://localhost:8080/v1/metrics` 返回 Prometheus 指标
- [ ] Jaeger UI (http://localhost:16686) 可访问并看到 trace 数据
- [ ] OTel Collector 接收来自 hermesx 的遥测数据

### 进阶验证

- [ ] `docker compose -f docker-compose.prod.yml ps` 所有服务状态为 healthy
- [ ] Nginx 负载均衡正常（多副本场景）
- [ ] MinIO 控制台可访问，skills bucket 已创建

## v2.0.0 新增能力

v2.0.0 吸收上游 hermes-agent v2026.4.30 后，SaaS 模式自动获得以下 Agent 增强：

| 能力 | 说明 | 配置 |
|------|------|------|
| 上下文压缩 | 接近 token 限制时自动摘要历史，保持长对话连贯性 | `context_compression: true` |
| 多模态路由 | 图片/音频/视频请求按提供商能力自动分发 | 配置 `AUXILIARY_VISION_*` |
| 自主记忆整理 | 去重、LLM 合并、过期清理 | 自动启用 |
| 自我改进循环 | 定期对话质量自评，持久化改进洞察 | 自动启用 |
| CJK 模糊搜索 | pg_trgm 中日韩文字模糊匹配 | PostgreSQL pg_trgm 扩展 |
| 模型目录热重载 | 运行时更新可用模型列表，无需重启 | 自动启用 |

这些能力对已有 API 完全兼容，无需修改客户端调用方式。

## 下一步

- [API 参考](api-reference.md) — 完整的端点文档
- [认证系统](authentication.md) — Auth Chain、API Key、RBAC
- [配置指南](configuration.md) — 所有环境变量
- [部署指南](deployment.md) — Docker Compose / Helm / Kind（含 v2.0.0 生产检查清单）
- [架构概览](architecture.md) — 系统设计与数据流
- [企业加固](enterprise-hardening.md) — Phase 1-5 加固全记录
