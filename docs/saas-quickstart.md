# SaaS 模式快速开始

> 5 分钟内启动 Hermes SaaS 多租户 API 服务器（v1.4.0）。

## 前置条件

| 依赖 | 版本 | 说明 |
|------|------|------|
| Go | 1.22+ | 编译 hermes 二进制 |
| PostgreSQL | 16+ | 多租户数据存储 |
| Docker + Docker Compose | 最新版 | 可选，一键启动基础设施 |

## 方式一：Docker Compose 快速启动（推荐）

```bash
# 1. 克隆仓库
git clone https://github.com/MLT-OSS/hermesx.git
cd hermesx

# 2. 构建二进制
go build -o hermes ./cmd/hermes/

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
./hermes saas-api
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
go build -o hermes ./cmd/hermes/

export DATABASE_URL="postgres://$(whoami)@127.0.0.1:5432/hermes?sslmode=disable"
export HERMES_ACP_TOKEN="your-secret-admin-token"

./hermes saas-api
```

数据库表会在首次启动时自动创建（27 个 migration 自动执行）。

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

## v1.4.0 新增能力

v1.4.0 吸收上游 hermes-agent v2026.4.30 后，SaaS 模式自动获得以下 Agent 增强：

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
- [部署指南](deployment.md) — Docker / Helm / Kind
- [架构概览](architecture.md) — 系统设计与数据流
- [企业加固](enterprise-hardening.md) — Phase 1-5 加固全记录
