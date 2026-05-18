# Release Plan: v2.3.0 Security Integration Sprint

> **角色**: devops-engineer  
> **状态**: released  
> **日期**: 2026-05-18  
> **关联任务**: 2026-05-18-v230-security-integration

---

## 发布信息

| 项目 | 内容 |
|------|------|
| 版本 | hermesx v2.3.0 |
| 发布类型 | Security Integration Sprint — minor version |
| 发布窗口 | 2026-05-18（当日） |
| 放行依据 | launch-acceptance.md — READY（5 阻塞项全部修复，26/26 测试通过） |
| 发布负责人 | devops-engineer |
| 值守负责人 | backend-engineer（24h 观察窗口） |

---

## 变更与风险

### 变更范围

| Story | 模块 | 变更摘要 |
|-------|------|----------|
| A | `internal/agent` | SafetyInterceptor 接入 RunConversation（audit 模式，超时 500ms fail-open） |
| B | `internal/agent`, `internal/egress` | SecureTransport 注入工具层；共享 Transport；ErrNotAllowed 语义修复 |
| C | `internal/tools` (4 files) | SecretResolver 替换 os.Getenv，fallback + warn 保留 |
| D | `internal/api/admin` | 11 Admin 端点统一注册；DI 迁移（globalXxx → struct fields）；64KB body limit |
| E | `internal/safety` | Canary token TTL cleanup goroutine；opaque handle（sha256[:4]） |
| F | `internal/secrets`, `internal/safety`, `.golangci.yml` | WithAllowedKeys、NFKC normalization、forbidigo linter、CachedEgressPolicy |

### 不在本次发布范围

- WASM sandbox（ADR-006 推迟）
- WebUI 安全配置界面
- v2.4.0 per-tenant EgressPolicy（AllowAllPolicy 过渡中）
- Admin singleton 完整 DI 重构（next sprint）

### 风险与缓解

| 风险 | 影响 | 缓解 | Owner |
|------|------|------|-------|
| AllowAllPolicy 过渡（无 per-tenant 主机限制） | egress 仅 RFC-1918 SSRF 防护 | IsBlockedIP 阻断私有网段；v2.4.0 替换 allowlist | tech-lead |
| Canary goroutine 双实例（main.go vs InterceptorChain） | 独立实例，不冲突 | v2.4.0 统一 Runner 接入 | backend-engineer |
| Admin singleton 测试竞态 | 生产单线程安全，测试 -race 已通过 | next sprint DI 完整迁移 | backend-engineer |
| per-call Transport 改为共享 | staging 负载下可接受连接池 | v2.4.0 前修复 | backend-engineer |
| SafetyInterceptor audit-only 上线 | 无拦截效果，仅记录 | 上线后审查日志，enforce 策略另行评估 | tech-lead |

---

## 执行步骤

### Pre-release 检查（Go/No-Go）

- [ ] `go test ./... -race` → 26/26 通过，0 新增失败 ✅
- [ ] `go build ./cmd/hermesx` → 干净编译 ✅
- [ ] launch-acceptance.md 状态：READY ✅
- [ ] DB migration SQL 已准备：000001, 000002 ✅
- [ ] Admin API 11 端点 RequireScope("admin") 覆盖确认 ✅

### 发布步骤

```bash
# 1. 运行 DB migrations（staging 先验证）
migrate -database $STAGING_PG_DSN -path ./migrations up

# 2. 构建并推送镜像（CI 自动，或手工触发）
docker build -t ghcr.io/colin4k1024/hermesx:v2.3.0 .
docker push ghcr.io/colin4k1024/hermesx:v2.3.0

# 3. K8s Helm 升级（staging 先）
helm upgrade hermesx ./charts/hermesx \
  --set image.tag=v2.3.0 \
  --namespace hermesx-staging \
  --atomic --timeout 5m

# 4. staging 验证（见下方验证步骤）

# 5. 生产发布
helm upgrade hermesx ./charts/hermesx \
  --set image.tag=v2.3.0 \
  --namespace hermesx-prod \
  --atomic --timeout 5m
```

### 暂停点 / Go-No-Go

| 暂停点 | 判断标准 | 通过则 |
|--------|----------|--------|
| staging 发布后 | 验证步骤全通过，无 5xx，egress warn 日志静默 | 推进到 production |
| production 发布后 | 同上 + 24h 观察窗口开始 | 发布结束，转入观察 |

---

## 验证与监控

### 发布后立即验证

```bash
# 健康检查
curl -f http://hermesx/health

# Admin API 端点验证（需 admin token）
curl -H "Authorization: Bearer $ADMIN_TOKEN" \
  http://hermesx/admin/v1/safety/rules
# 期望: 200 OK

curl -H "Authorization: Bearer $ADMIN_TOKEN" \
  http://hermesx/admin/v1/secrets/canary-tokens
# 期望: 200 OK, tokens 数组（可为空）

curl -H "Authorization: Bearer $ADMIN_TOKEN" \
  http://hermesx/admin/v1/egress/policy
# 期望: 200 OK

# 触发一次对话，检查 safety audit 日志是否产生
# 搜索日志: slog field "safety.audit" 存在即可
```

### 关键观察项（24h 观察窗口）

1. **SafetyInterceptor audit 日志**：`POST /admin/v1/safety/scan` 是否被消费
2. **SecretResolver fallback**：`slog.Warn "secrets: resolve failed"` 是否出现（出现则排查）
3. **egress IsBlockedIP 命中率**：Prometheus `egress_blocked_total` 有无异常激增
4. **Canary token 数量**：`GET /admin/v1/secrets/canary-tokens` count 是否在预期范围
5. **Admin API 错误率**：新增 11 端点 5xx 率应为 0

---

## 回滚方案

### 触发条件

- ModeEnforce 意外激活导致对话被阻断（立即回滚）
- Admin API 5xx 率持续 > 1%（5 分钟内观察）
- SecureTransport 大量阻断合法出站
- SecretResolver fallback 持续 warn 且工具不可用
- staging `go test -race` 出现新增失败（阻止上 production）

### 回滚操作

```bash
# Helm 回滚（推荐）
helm rollback hermesx 0 --namespace hermesx-prod

# 确认回滚完成
kubectl rollout status deployment/hermesx -n hermesx-prod
curl -f http://hermesx/health

# DB 回滚（仅在确认需要时执行）
# 注意：safety_policies / secret_patterns 表新增但未修改旧表，
# v2.2.0 二进制与 v2.3.0 schema 兼容（旧二进制不读新表）
# migrate -database $PG_DSN -path ./migrations down 2  # 仅必要时
```

---

## 放行结论

**✅ 允许发布 — v2.3.0 Security Integration Sprint**

| 放行依据 | 状态 |
|----------|------|
| 5 个安全阻塞项（B-1~B-5）全部修复 | ✅ |
| `go test ./... -race` 26/26 通过，0 新增失败 | ✅ |
| launch-acceptance.md QA 确认 READY | ✅ |
| Admin API 11 端点 RequireScope 覆盖 | ✅ |
| 向后兼容性（fallback 保留，schema 兼容） | ✅ |
| 可接受风险已文档存档 | ✅ |

### 确认记录

| 角色 | 结论 | 日期 |
|------|------|------|
| qa-engineer | READY — 阻塞项修复验证通过 | 2026-05-18 |
| backend-engineer | 修复完成，回归通过 | 2026-05-18 |
| devops-engineer | 发布方案就绪，可执行 | 2026-05-18 |
| tech-lead | 待最终放行确认 | — |

---

## 发布后遗留事项（v2.4.0）

| 事项 | 优先级 | Owner |
|------|--------|-------|
| AllowAllPolicy → per-tenant EgressPolicy | P1 | tech-lead |
| per-call Transport 连接池优化 | P2 | backend-engineer |
| Canary goroutine 双实例 → 统一 Runner | P2 | backend-engineer |
| Admin singleton 完整 DI 重构 | P3 | backend-engineer |
| WASM sandbox (ADR-006) | P3 | architect |
