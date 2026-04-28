# Execute Log: SaaS Readiness — Phase 0-5

| 字段 | 值 |
|------|-----|
| Slug | `saas-readiness` |
| 日期 | 2026-04-28 |
| 主责 | backend-engineer |
| 状态 | completed |
| 阶段 | execute |
| 范围 | Phase 0-5 — T0a-T0d, T1-T18 全量实现 |

---

## Phase 0: 前置基础设施

| 任务 | 计划 | 实际 | 偏差 |
|------|------|------|------|
| T0a: Migration versioning | `[]string` → `[]migration` + `schema_version` | 完成，16 个现有 DDL 编号 v1-v16，新增 api_keys 表 v17-v19 | 无偏差 |
| T0b: Store interface extension | 新增 3 个 sub-store 接口 + PG 实现 + SQLite stubs | 完成，含完整 CRUD 实现（非 placeholder） | PG 实现直接写了完整 CRUD 而非空壳 |
| T0c: Middleware chain builder | 创建 `chain.go` + 统一 stack | 完成，6 slot middleware chain，nil = passthrough | 无偏差 |
| T0d: AuthContext struct | 创建 `auth/context.go` | 完成 | 无偏差 |

## Phase 1: Auth + Tenant + RBAC + Rate Limit + Audit + Health

| 任务 | 计划 | 实际 | 偏差 |
|------|------|------|------|
| T1: CredentialExtractor chain | StaticTokenExtractor + ExtractorChain | 完成 `auth/extractor.go`, `auth/static.go`, `auth/apikey.go` | 同时实现了 APIKeyExtractor（原 P2 scope 前置） |
| T2: Auth middleware | 基于 ExtractorChain 填充 AuthContext | 完成 `middleware/auth.go`，支持 allowAnonymous 开关 | 无偏差 |
| T3: Tenant trust fix | 从 AuthContext 推导 tenant，不信任 X-Tenant-ID | 完成 `middleware/tenant.go` 重写，context key 类型改为 `tenantCtxKey` 避免冲突 | 无偏差 |
| T4: RBAC middleware | 路由 → 角色映射 | 完成 `middleware/rbac.go`，prefix-match + admin bypass | 无偏差 |
| T5: Rate limit middleware | Redis sliding window + local fallback | 完成 `middleware/ratelimit.go`，含 localLimiter 内存降级 | Redis 实际对接留给集成阶段 |
| T6: Audit middleware | 请求完成后写 audit log | 完成 `middleware/audit.go` | 无偏差 |
| T7: Audit endpoint | GET /v1/audit-logs | 完成 `api/audit.go` | 无偏差 |
| T8: Health probes | /health/live + /health/ready | 完成 `api/health.go`，PGStore 新增 `Ping()` | 无偏差 |
| T9: RequestID middleware | X-Request-ID propagation | 完成 `middleware/requestid.go` | 无偏差 |

## Phase 2: Tenant CRUD + JWT + API Key

| 任务 | 计划 | 实际 | 偏差 |
|------|------|------|------|
| T10: Tenant CRUD API | POST/GET/PUT/DELETE /v1/tenants | 完成 `api/tenants.go`，复用 P0 pgTenantStore | 无偏差 |
| T11: JWT RS256 extractor | 基于 golang-jwt 的 RS256 验证 | 完成 `auth/jwt.go`，支持 issuer + expiry 验证 | 新增依赖 `golang-jwt/jwt/v5` |
| T12: API Key lifecycle | POST/GET/DELETE /v1/api-keys + SHA-256 hash | 完成 `api/apikeys.go`，生成 `hk_` 前缀 key | 无偏差 |

## Phase 3: Observability

| 任务 | 计划 | 实际 | 偏差 |
|------|------|------|------|
| T13: Prometheus metrics | request count/latency/in-flight | 完成 `middleware/metrics.go`，新增依赖 `prometheus/client_golang` | 无偏差 |
| T14: Metrics endpoint | GET /v1/metrics | 完成 `api/metrics.go`，复用 promhttp.Handler() | 无偏差 |

## Phase 4: API 治理 + 计费

| 任务 | 计划 | 实际 | 偏差 |
|------|------|------|------|
| T15: OpenAPI spec | Embedded JSON spec | 完成 `api/openapi.go` | 静态 spec 结构，非 codegen |
| T16: Usage/billing endpoint | GET /v1/usage | 完成 `api/usage.go` | 无偏差 |
| T17: Quota enforcement | 会话配额中间件 | 完成 `middleware/quota.go` | 无偏差 |

## Phase 5: 安全加固 + 部署

| 任务 | 计划 | 实际 | 偏差 |
|------|------|------|------|
| T18: TLS config | 支持 TLS 1.2/1.3 | 完成 `config/tls.go` | 无偏差 |
| T19: SecretStore interface | 环境变量 backend | 完成 `secrets/secrets.go`，EnvSecretStore | 无偏差 |
| T20: GDPR export/delete | GET /v1/gdpr/export + DELETE /v1/gdpr/delete | 完成 `api/gdpr.go` | 无偏差 |
| T21: Helm chart | deployment + service + values | 完成 `deploy/helm/hermes-agent/` | 无偏差 |
| T22: CI security scanning | govulncheck + gosec + trivy | 完成 `.github/workflows/security.yml` | 无偏差 |

---

## 关键决定

1. **PG sub-stores 直接实现完整 CRUD**（P0）：标准 CRUD SQL 不增加风险，P1 可直接消费。
2. **Middleware chain 不挂载到 server**（P0-P1）：chain.go 就绪但需要 server 重构时一次性挂载，避免破坏现有认证。
3. **APIKeyExtractor 从 P2 前置到 P1**：auth chain 需要完整的 credential extractor 序列才能验证。
4. **Redis rate limiter 使用接口抽象 + local fallback**：分布式限流的 Redis 实际对接留给集成阶段，本地 fallback 保证功能完整。
5. **Tenant middleware 重写**（CHL-2 fix）：不再信任 X-Tenant-ID header，改为从 AuthContext 推导。
6. **JWT 新增外部依赖**：`golang-jwt/jwt/v5`，零依赖方案无法满足 RS256 需求。
7. **Prometheus 新增外部依赖**：`prometheus/client_golang`，行业标准。
8. **OpenAPI 使用静态 JSON**：不引入 codegen 框架，保持零外部工具链。
9. **Helm chart 最小可用**：deployment + service + probes + env，不包含 ingress/HPA/PDB（按需扩展）。

## 影响面

### 新增文件（P1-P5）
- `internal/auth/extractor.go` — CredentialExtractor + ExtractorChain
- `internal/auth/static.go` — StaticTokenExtractor
- `internal/auth/apikey.go` — APIKeyExtractor + HashKey
- `internal/auth/jwt.go` — JWTExtractor (RS256)
- `internal/middleware/auth.go` — Auth middleware
- `internal/middleware/rbac.go` — RBAC middleware
- `internal/middleware/ratelimit.go` — Rate limit + local fallback
- `internal/middleware/audit.go` — Audit write middleware
- `internal/middleware/requestid.go` — X-Request-ID propagation
- `internal/middleware/metrics.go` — Prometheus metrics middleware
- `internal/middleware/quota.go` — Session quota enforcement
- `internal/api/audit.go` — GET /v1/audit-logs handler
- `internal/api/health.go` — /health/live + /health/ready
- `internal/api/tenants.go` — Tenant CRUD handler
- `internal/api/apikeys.go` — API Key lifecycle handler
- `internal/api/openapi.go` — OpenAPI spec handler
- `internal/api/usage.go` — Usage/billing handler
- `internal/api/metrics.go` — Prometheus metrics handler
- `internal/api/gdpr.go` — GDPR export/delete handlers
- `internal/config/tls.go` — TLS configuration
- `internal/secrets/secrets.go` — SecretStore interface + env backend
- `deploy/helm/hermes-agent/` — Helm chart (Chart.yaml, values.yaml, templates/)
- `.github/workflows/security.yml` — CI security scanning

### 修改文件（P1-P5）
- `internal/middleware/tenant.go` — 重写：AuthContext 推导替代 X-Tenant-ID header 信任
- `internal/store/pg/pg.go` — 新增 `Ping()` 方法
- `go.mod` / `go.sum` — 新增 `golang-jwt/jwt/v5`, `prometheus/client_golang`

### 验证
- `go build ./...` PASS
- `go test ./...` 1153 tests PASS, 0 failures, 28 packages

## 未完成项

- Server 挂载 middleware chain（需要 ACP + API server 重构，建议独立 PR）
- Redis rate limiter 实际实现（需要 Redis 连接管理）
- JWT 公钥配置管理（需要 key rotation 策略）
- Helm chart 高级配置（Ingress, HPA, PDB, NetworkPolicy）
- E2E 集成测试（需要 testcontainers PostgreSQL）
- OpenAPI spec 补全所有 path 的 schema 定义

---

*最后更新：2026-04-28*
