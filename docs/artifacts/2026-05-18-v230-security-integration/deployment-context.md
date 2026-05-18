# Deployment Context: v2.3.0 Security Integration Sprint

> **角色**: devops-engineer  
> **状态**: released  
> **日期**: 2026-05-18  
> **关联任务**: 2026-05-18-v230-security-integration

---

## 环境清单

| 环境 | 用途 | 访问入口 | 部署目标 |
|------|------|----------|----------|
| local | 开发自测 | `localhost:8080` | `go run ./cmd/hermesx` |
| staging | 集成测试 + QA 验证 | staging.hermesx.internal | Docker Compose / K8s namespace |
| production | 生产 | hermesx.prod / ghcr.io 镜像 | K8s cluster (Helm v3) |

---

## 部署入口

| 入口类型 | 操作 | 前置条件 |
|----------|------|----------|
| 主入口 | `go build ./cmd/hermesx` → 替换二进制，或 `docker build` → push ghcr.io | DB migrations 已执行 |
| CI 自动入口 | GitHub Actions: main 分支 push 触发 Docker build + push to ghcr.io | PR 合并 + CI 绿 |
| 手工回退入口 | 回滚到上一镜像 tag（v2.2.0）或还原二进制 | 见回滚方案 |
| K8s 入口 | `helm upgrade hermesx ./charts/hermesx --set image.tag=v2.3.0` | Helm v3 + kubeconfig |

### 前置条件

1. **DB migrations 已执行**（v2.3.0 新增）：
   - `000001_create_safety_policies.sql`（safety_policies 表）
   - `000002_create_secret_patterns.sql`（secret_patterns 表）
   - 若使用 PostgreSQL：`migrate -database $PG_DSN -path ./migrations up`
   - 若使用 MySQL：`migrate -database $MYSQL_DSN -path ./migrations up`

2. **Admin API DI 已对齐**：`AdminHandler` 构造时须传入 `policyStore`、`canaryDetector`、`leakScanner`（见 `internal/api/admin/handler.go`）。

---

## 配置与密钥

### 新增环境变量（v2.3.0）

| 变量 | 说明 | 是否必须 | 来源 |
|------|------|----------|------|
| `SAFETY_MODE` | `audit`（默认） 或 `enforce` | 否 | `.env` / K8s Secret |
| `SAFETY_TIMEOUT_MS` | CheckInput/CheckOutput 超时，默认 500ms | 否 | `.env` |
| `EGRESS_ALLOW_ALL` | 过渡期：`true` 使用 AllowAllPolicy（待 v2.4.0 per-tenant 替换） | 否 | `.env` |
| `CANARY_TTL_SECONDS` | Canary token 有效期，默认 3600s | 否 | `.env` |

### 已有关键变量（保持不变）

| 变量 | 说明 |
|------|------|
| `PG_DSN` / `MYSQL_DSN` | 数据库连接 |
| `REDIS_ADDR` | Redis 集群地址 |
| `ADMIN_JWT_SECRET` | Admin API JWT 签名密钥 |
| `OIDC_ISSUER_URL` | OIDC 提供商（可选激活） |

### 密钥来源

- 生产环境：K8s Secret（`kubectl create secret generic hermesx-secrets`）
- 开发/staging：`~/.hermes/.env` 文件（不入库）
- SecretResolver 工具层：支持从 env 动态解析，fallback 为 `os.Getenv`（已有 warn 日志）

---

## 运行保障

### 功能开关

| 开关 | 状态 | 说明 |
|------|------|------|
| SafetyInterceptor | audit 模式（默认） | 上线初期不阻塞，仅记录 audit 日志 |
| SecureTransport | 全量启用 | IsBlockedIP SSRF 防护 + egress policy |
| SecretResolver | 全量启用 | 工具层 fallback 保留向后兼容 |
| Canary TTL cleanup | 全量启用 | goroutine 随 server shutdown context cancel |
| CachedEgressPolicy | TTL 60s | AllowAllPolicy 过渡（v2.4.0 替换） |
| Admin API | 全量启用 | 11 端点，RequireScope("admin") 全部覆盖 |

### 监控与告警

| 指标 / 日志 | 观察路径 | 告警阈值 |
|-------------|----------|----------|
| SafetyInterceptor audit 日志 | `slog` JSON → 日志平台搜索 `safety.audit` | 首次触发时验证消费 |
| CheckInput/CheckOutput 超时降级 | `slog.Warn "safety: CheckInput error, degrading to allow"` | >5 次/分钟 告警 |
| SecretResolver fallback | `slog.Warn "secrets: resolve failed, falling back to env"` | 任意出现 → 排查 |
| egress IsBlockedIP 命中 | `slog.Warn "egress: blocked IP"` / Prometheus `egress_blocked_total` | 异常激增 |
| Canary token 活跃数 | Admin API `GET /admin/v1/secrets/canary-tokens` | 超过预期值 |

### 值守安排

- 上线后观察窗口：24h
- 值守角色：backend-engineer（主）+ devops-engineer（备）
- 联系方式：内部 on-call 渠道

---

## 恢复能力

### 回滚触发条件

- SafetyInterceptor ModeEnforce 误 fail-close（干扰正常对话）
- Admin API 新端点 5xx 率 > 1%
- SecureTransport 大量阻断合法出站（egress_blocked_total 异常）
- SecretResolver fallback 告警持续 > 2 分钟且工具不可用
- `go test ./... -race` 在 staging 出现新增失败

### 回滚路径

```bash
# K8s Helm 回滚
helm rollback hermesx 0   # 回滚到上一个 release

# Docker Compose 手工回滚
docker pull ghcr.io/colin4k1024/hermesx:v2.2.0
docker-compose up -d

# 二进制替换（如直接部署）
cp hermesx-v2.2.0 /usr/local/bin/hermesx
systemctl restart hermesx
```

### 回滚验证

回滚后确认：
1. `GET /health` → 200
2. Admin API `GET /admin/v1/safety/rules` → 200（若 v2.2.0 无此端点则接受 404）
3. `go test ./... -race` staging → 全绿

---

## v2.3.0 变更摘要（部署相关）

| 变更类型 | 描述 |
|----------|------|
| 新增 DB tables | safety_policies, secret_patterns（须 migrate up） |
| 新增 Admin API | 11 端点：safety(5) + egress(3) + secrets(3) |
| 新增 goroutine | Canary TTL cleanup（随 ctx cancel 自动退出） |
| 共享 Transport | 单 `*http.Transport` per Agent 实例（取代 per-call http.Client） |
| body size 限制 | Admin POST 端点 64KB MaxBytesReader |
| 鉴权变更 | 无（沿用 RequireScope("admin")） |
| 向后兼容 | SecretResolver/SecureTransport 均保留 os.Getenv fallback |
