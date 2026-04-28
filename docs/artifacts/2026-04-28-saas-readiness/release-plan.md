# Release Plan: SaaS Readiness P0-P5

| 字段 | 值 |
|------|-----|
| Slug | `saas-readiness` |
| 日期 | 2026-04-28 |
| 主责 | devops-engineer |
| 状态 | draft |
| 阶段 | released |

---

## 发布信息

| 项目 | 说明 |
|------|------|
| 发布对象 | hermes-agent-go SaaS Readiness P0-P5 |
| 发布版本 | v0.2.0-saas (suggested semver) |
| 发布类型 | Feature release (multi-tenant SaaS foundation) |
| 发布窗口 | 非高峰时段，建议工作日上午 10:00-12:00 |
| 发布负责人 | devops-engineer |
| 值守人 | backend-engineer (on-call during observation window) |
| 前序条件 | CRIT-1 (ACP auth bypass) 必须在生产部署前修复 |

---

## 变更范围

### 新增文件（23 个）

| 目录 | 文件 | 功能 |
|------|------|------|
| `internal/auth/` | `context.go`, `extractor.go`, `static.go`, `apikey.go`, `jwt.go` | 认证链 + AuthContext |
| `internal/middleware/` | `chain.go`, `auth.go`, `tenant.go`, `rbac.go`, `ratelimit.go`, `audit.go`, `requestid.go`, `metrics.go` | Middleware stack |
| `internal/api/` | `tenants.go`, `apikeys.go`, `audit.go`, `health.go`, `gdpr.go`, `openapi.go`, `usage.go`, `quota.go` | API handlers |
| `internal/config/` | `tls.go` | TLS configuration |
| `internal/secrets/` | `store.go` | SecretStore abstraction |

### 修改文件（3 个）

| 文件 | 变更 |
|------|------|
| `internal/store/store.go` | 新增 TenantStore, AuditLogStore, APIKeyStore 接口 |
| `internal/store/pg/pg.go` | PGStore 扩展子 store 访问器 |
| `internal/store/sqlite/sqlite.go` | SQLiteStore 扩展 noop 子 store |

### 新增子 store 实现

| 文件 | 功能 |
|------|------|
| `internal/store/pg/tenant.go` | PostgreSQL 租户 CRUD |
| `internal/store/pg/auditlog.go` | PostgreSQL 审计日志 |
| `internal/store/pg/apikey.go` | PostgreSQL API Key 生命周期 |
| `internal/store/pg/migrate.go` | 版本化迁移框架 |
| `internal/store/sqlite/noop.go` | SQLite noop stubs |

### 新增依赖

| 依赖 | 版本 | 用途 |
|------|------|------|
| `github.com/golang-jwt/jwt/v5` | v5.x | JWT RS256 验证 |
| `github.com/prometheus/client_golang` | latest | Prometheus metrics |

### 基础设施

| 文件 | 功能 |
|------|------|
| `deploy/helm/hermes-agent/` | Helm chart scaffold |
| `.github/workflows/security.yml` | CI security scanning (govulncheck + gosec + trivy) |

---

## 风险评估

| 风险 | 等级 | 缓解措施 |
|------|------|---------|
| CRIT-1: ACP auth bypass (pre-existing) | CRITICAL | 首次生产部署前必须修复 `HERMES_ACP_TOKEN` 强制校验 |
| Middleware chain 未挂载到 HTTP server | HIGH | 新代码不影响现有路径，独立 PR 挂载 |
| 新增代码缺专门单元测试 | MEDIUM | 1153 现有 tests 全量通过，需补充至少 80% 覆盖 |
| Redis 限流未对接 | MEDIUM | localLimiter 单副本可用，多副本需 Redis |
| Helm secrets 明文注入 | MEDIUM | 开发/测试可接受，生产需 ExternalSecrets |

---

## 执行步骤

### Step 1: Pre-flight 检查

```bash
# 1.1 确认代码编译通过
go build ./...

# 1.2 确认测试全量通过
go test ./...
# 预期: 1153 tests, 28 packages, 0 failures

# 1.3 确认安全扫描无新增 CRITICAL
govulncheck ./...
```

### Step 2: 构建镜像

```bash
# 2.1 构建 Go binary
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o hermes-agent ./cmd/hermes-agent

# 2.2 构建 Docker 镜像（待 Dockerfile 补充后执行）
# docker build -t hermes-agent:v0.2.0-saas .
```

### Step 3: Staging 部署

```bash
# 3.1 切换到 staging 集群
kubectl config use-context staging

# 3.2 Helm 部署
helm upgrade --install hermes-agent deploy/helm/hermes-agent/ \
  -f values-staging.yaml \
  --set image.tag=v0.2.0-saas \
  --wait --timeout 120s

# 3.3 验证 Pod 启动
kubectl get pods -l app=hermes-agent
kubectl logs -l app=hermes-agent --tail=50
```

### Step 4: Staging 验证

```bash
# 4.1 Health probes
curl -s http://staging.hermes.internal:3000/health/live   # 预期: 200
curl -s http://staging.hermes.internal:3000/health/ready  # 预期: 200

# 4.2 Prometheus metrics
curl -s http://staging.hermes.internal:3000/metrics | head -20

# 4.3 功能 smoke test（待 middleware chain 挂载后执行）
# curl -H "Authorization: Bearer <token>" http://staging.hermes.internal:3000/v1/tenants
```

### Step 5: Go / No-Go 决策点

| 检查项 | 条件 | 结果 |
|--------|------|------|
| Pod 正常启动 | Pod 状态 Running, restarts = 0 | |
| Health probes | `/health/live` 200, `/health/ready` 200 | |
| 无 5xx 错误 | `hermes_http_requests_total{status=~"5.."}` = 0 | |
| DB 连接正常 | readiness probe 不失败 | |
| CRIT-1 已修复 | `HERMES_ACP_TOKEN` 设置且强制校验 | **生产部署阻塞项** |

### Step 6: Production 部署（待 CRIT-1 修复后执行）

```bash
# 6.1 切换到 production 集群
kubectl config use-context production

# 6.2 Helm 部署
helm upgrade --install hermes-agent deploy/helm/hermes-agent/ \
  -f values-production.yaml \
  --set image.tag=v0.2.0-saas \
  --wait --timeout 120s

# 6.3 验证
kubectl get pods -l app=hermes-agent
curl -s http://hermes.internal:3000/health/ready
```

---

## 验证与监控

### 发布后即时检查（15 分钟）

| 检查项 | 方式 |
|--------|------|
| Pod 启动状态 | `kubectl get pods -l app=hermes-agent` |
| Readiness probe | `curl /health/ready` |
| DB 连接 | Readiness probe 内置 DB Ping |
| 错误率 | `rate(hermes_http_requests_total{status=~"5.."}[5m])` |

### 短期观察（2 小时）

| 指标 | 阈值 |
|------|------|
| 5xx 错误率 | < 5% |
| P99 延迟 | < 2s |
| 内存使用 | < 256Mi (limit) |
| Pod 重启 | = 0 |

### 中期观察（24 小时）

| 指标 | 关注项 |
|------|--------|
| 速率限制触发 | `hermes_http_requests_total{status="429"}` 比例 |
| 审计日志写入 | 确认 audit log 正常持久化 |
| 配额执行 | quota enforcement 触发情况 |

---

## 回滚方案

### 回滚触发条件

- Readiness probe 连续 3 次失败
- 5xx 错误率超过 5% 持续 5 分钟
- P99 延迟超过 5s 持续 10 分钟
- DB 连接池耗尽

### 回滚路径

```bash
# 1. Helm 回滚到上一版本
helm rollback hermes-agent --wait --timeout 120s

# 2. 验证回滚
kubectl get pods -l app=hermes-agent
curl -s http://<service>:3000/health/ready

# 3. 确认指标恢复
# 检查 Prometheus dashboard: 5xx 率归零、延迟恢复基线
```

### 回滚验证

- `/health/ready` 返回 200
- Prometheus 指标恢复基线
- 无新增 5xx 错误

---

## 放行结论

**结论：Conditional Go — 有条件放行**

### 已满足条件

- [x] 编译通过 (`go build ./...`)
- [x] 1153 tests 全量通过
- [x] 5 CRITICAL + 6 HIGH 安全问题已修复
- [x] Helm chart scaffold 就绪
- [x] CI 安全扫描 workflow 就绪
- [x] Health probes + Prometheus metrics 就绪
- [x] Deployment context 文档完整

### 放行前提

| 前提 | 状态 | 阻塞等级 |
|------|------|---------|
| CRIT-1 (ACP auth bypass) 修复 | 未完成 | **生产部署阻塞** |
| Middleware chain 挂载到 server | 未完成 | staging 可部署，生产需完成 |
| 新增代码补充单元测试 (≥80%) | 未完成 | 不阻塞部署，下一迭代完成 |

### 后续观察项

1. 发布后 15 分钟内确认 Pod 启动和 DB 连接正常
2. 发布后 2 小时确认错误率、延迟和内存使用在基线内
3. 发布后 24 小时确认审计日志、速率限制和配额执行正常

### 确认记录

| 角色 | 结论 | 时间 |
|------|------|------|
| qa-engineer | Conditional Go，3 项前提条件 | 2026-04-28 |
| devops-engineer | Conditional Go，CRIT-1 阻塞生产 | 2026-04-28 |

---

*最后更新：2026-04-28*
