# Enterprise Readiness Assessment

> hermesx — Enterprise Agent Runtime / SaaS Control Plane  
> Assessment Date: 2026-05-07  
> Version: v1.4.0

---

## 1. Multi-Tenancy

**Capability:** 数据库级租户隔离，所有业务表强制 `tenant_id`

**Status:** Done

**Evidence:**
- 9 张业务表均包含 `tenant_id UUID NOT NULL REFERENCES tenants(id)`
- 50+ 处 `WHERE tenant_id = $1` 查询（PG Store 层）
- 55 条 RLS policy 定义（`ALTER TABLE ... ENABLE ROW LEVEL SECURITY` + `CREATE POLICY`）
- `TenantMiddleware` 从 `AuthContext` 派生 `tenant_id`，**不从请求头读取**
- 静态分析工具 `go-sql-tenant-enforcement` 验证所有 SQL 路径

**Risk:**
- RLS SELECT policy 仅在 superuser 外生效；应用连接池使用 superuser 时需依赖 `SET LOCAL app.tenant_id`
- 缺少自动化跨租户渗透测试

**Next Action:**
- Week 2: 补 `tests/integration/tenant_isolation_test.go` + `cross_tenant_attack_test.go`

---

## 2. Auth / API Key / RBAC

**Capability:** 4 层链式认证 + method×path 细粒度 RBAC

**Status:** Done

**Evidence:**
- Auth Chain: Static Token → API Key (HMAC-SHA256) → JWT → OIDC (JWKS rotation + ClaimMapper)
- `OIDCExtractor` 支持 JWKS 自动轮换、claim 映射、env 激活
- API Key: raw key 仅创建时返回，DB 存 SHA-256 hash，支持 prefix 查询
- RBAC: `method+path` 组合控制，`admin` 超级权限，`user` 限定路由
- Scope enforcement: `HasScope()` 检查 API key scopes

**Risk:**
- API key 创建时 `tenant_id` 可由 body 传入（admin caller）— 需严格校验调用者角色
- `HasScope` 对 empty scopes 的放行逻辑已修复但需回归测试覆盖
- `rand.Read` 错误未处理（key 生成路径）

**Next Action:**
- Week 3: 修复 tenant_id 越权风险 + rand.Read error handling + 统一 RBAC_MATRIX

---

## 3. Rate Limit / Quota

**Capability:** 双层限流（Redis 分布式 + 本地 LRU 降级）

**Status:** Done

**Evidence:**
- Redis Lua 原子脚本实现 tenant + user 维度限流
- `LocalDualLimiter` 作为 Redis 不可用时的降级方案
- 默认租户配额: 120 RPM
- 压测验证: 10 并发 100 请求中正确触发 429
- `max_memories` per-tenant 配额，超限返回 429

**Risk:**
- 多副本部署时 LocalDualLimiter 精确性下降（已记录为 P3 技术债）
- 无 burst/token bucket 模型，仅简单 RPM 计数

**Next Action:**
- 生产部署时确保 Redis 高可用；评估是否需要 token bucket

---

## 4. Metering / Billing

**Capability:** Token usage 记录 + Dynamic Pricing + Admin CRUD

**Status:** Partial

**Evidence:**
- `usage_records` 表记录每次 LLM 调用的 input/output tokens
- `UpdateTokens()` 在每次 chat 完成后更新 session 计数器
- `PricingRules` Store 支持动态定价（Admin API CRUD）
- GDPR 删除覆盖 `usage_records`

**Risk:**
- 无计费报告 API（前端 dashboard 未实现）
- 无月度/日度聚合 API
- 无用量告警阈值

**Next Action:**
- P1: 聚合查询 API（GET /v1/usage?granularity=daily&from=&to=）
- P2: Admin 用量 dashboard

---

## 5. Audit / Compliance

**Capability:** 全请求审计日志 + 不可篡改触发器

**Status:** Done

**Evidence:**
- `AuditMiddleware` 记录所有认证请求：action, tenant_id, user_id, request_id, status_code, latency_ms
- `audit_logs` 表有 `BEFORE UPDATE OR DELETE` 触发器阻止篡改
- 审计日志包含 `request_id` 可关联 OpenTelemetry trace
- Migration v24-v27 增强审计字段

**Risk:**
- 审计日志无自动归档/轮换策略
- 无外部 SIEM 集成（仅 PG 存储）

**Next Action:**
- 评估审计日志 retention policy + 外部导出（S3/SIEM）

---

## 6. GDPR / Data Lifecycle

**Capability:** 事务性删除 + MinIO 对象清理 + 207 Multi-Status

**Status:** Done

**Evidence:**
- `DELETE /v1/gdpr/data` 级联删除: sessions, messages, memories, api_keys, cron_jobs, audit_logs, usage_records
- `POST /v1/gdpr/cleanup-minio` 清理 MinIO 对象（soul + skills）
- `GET /v1/gdpr/export` 导出租户全量数据
- RLS WITH CHECK 写策略防止跨租户写入

**Risk:**
- 删除操作不可逆且无软删除选项
- MinIO 清理为异步，极端情况可能残留对象
- 无自助数据导出 UI（P4 backlog）

**Next Action:**
- 评估 soft-delete 模式 + 30-day grace period

---

## 7. Observability

**Capability:** OTel Tracing + Prometheus Metrics + Structured Logging

**Status:** Done

**Evidence:**
- `InitTracer` 接入 OTel SDK，W3C Trace Context 传播
- Prometheus metrics: request count/latency/concurrent, rate limiter hits, circuit breaker state, LLM call latency
- `normalizePath` 修复高基数问题（UUID/数字/Hex → `:id`）
- Structured logging: slog + tenant_id + request_id 自动注入
- `/metrics` endpoint exposed

**Risk:**
- 无预置 Grafana dashboard JSON
- 无 alert rules（Prometheus alerting 未配置）
- OTel exporter 需要外部 collector（未包含在 docker-compose 中）

**Next Action:**
- Week 7: 补 Grafana dashboard + alert rules + OTel Collector 配置

---

## 8. Sandbox Isolation

**Capability:** 本地进程隔离 + Docker 容器隔离 + per-tenant SandboxPolicy

**Status:** Done

**Evidence:**
- `SandboxConfig`: timeout, max_stdout, allowed_tools, max_tool_calls
- `SandboxPolicy` JSONB 列（per-tenant 配额）
- Docker sandbox: `--network=none`, `--memory`, `--cpus` 资源限制
- 环境变量白名单: 只暴露 PATH/HOME/LANG/TERM/TMPDIR
- 工具白名单 enforcement: 非白名单工具 → 拒绝
- 82 处 sandbox 相关代码引用

**Risk:**
- Docker sandbox 依赖宿主 Docker daemon（K8s 环境需要 DinD 或 kata containers）
- 无 gVisor/Firecracker 级隔离

**Next Action:**
- 评估 K8s 部署场景的 sandbox 方案（sidecar vs. job-based）

---

## 9. Backup / Disaster Recovery

**Capability:** pgBackRest PITR 配置 + 恢复演练脚本

**Status:** Partial

**Evidence:**
- `deploy/pitr/` 目录存在 pgBackRest 配置
- `scripts/pitr-drill.sh` 恢复演练脚本
- README 声明 RPO < 5min, RTO < 1h

**Risk:**
- 未验证在生产级数据量下的 RTO
- Redis/MinIO 无独立备份方案
- 无自动化恢复测试 CI

**Next Action:**
- Week 7: 补 Redis RDB 备份 + MinIO mc mirror + 自动化恢复测试

---

## 10. CI / Security

**Capability:** GitHub Actions (unit + integration + race + Docker push)

**Status:** Done

**Evidence:**
- `.github/workflows/`: unit tests, integration tests (-race), Docker build + push to ghcr.io
- 安全修复记录: HasScope bypass, drainAndFlush infinite loop, SQL injection hardening, Redis race condition, GDPR cascade
- Prompt sanitization 在 compress.go / curator.go（部分覆盖）

**Risk:**
- GHA actions 未 digest-pin（P3 backlog）
- 无 SAST/DAST 扫描集成
- 无 dependency vulnerability 自动扫描

**Next Action:**
- 评估 `govulncheck` + `trivy` 集成到 CI

---

## 11. Known Risks

| # | Risk | Severity | Mitigation |
|---|------|----------|------------|
| 1 | API Key tenant_id 可被 admin body 指定 | HIGH | Week 3 修复 |
| 2 | MiniMaxi Token Plan 并发限制 | MEDIUM | 升级套餐或切换 provider |
| 3 | 多副本 LocalDualLimiter 精确性 | MEDIUM | 生产确保 Redis 高可用 |
| 4 | 审计日志无归档策略 | LOW | 评估 retention + 外部导出 |
| 5 | Docker sandbox 在 K8s 下需要 DinD | MEDIUM | 评估 job-based sandbox |
| 6 | 无 ExecutionReceipt（tool call 不可审计） | HIGH | Week 4-5 实现 |

---

## 12. Roadmap

| Week | Theme | Key Deliverables |
|------|-------|-----------------|
| 1 | 文档冻结 | ENTERPRISE_READINESS, ARCHITECTURE, SECURITY_MODEL, RBAC_MATRIX |
| 2 | 隔离证明 | tenant isolation tests, RLS policy tests, cross-tenant attack tests |
| 3 | 权限加固 | API key fix, rand.Read fix, unified RBAC matrix |
| 4 | ExecutionReceipt 模型 | execution_receipts table, Store interface, migration, tests |
| 5 | Tool Runtime 接入 | before/after hooks, idempotency, dedup, trace binding |
| 6 | OpenAPI + SDK | openapi.yaml, curl/Go/TS examples |
| 7 | 部署闭环 | prod compose, backup/restore, observability, alerts |
| 8 | Demo + Release | enterprise-saas-demo, v1.0-rc |

---

## Summary

| Category | Status |
|----------|--------|
| Multi-Tenancy | ✅ Done |
| Auth / RBAC | ✅ Done (minor fixes needed) |
| Rate Limit | ✅ Done |
| Metering | ⚠️ Partial (no aggregation API) |
| Audit | ✅ Done |
| GDPR | ✅ Done |
| Observability | ✅ Done (no dashboards) |
| Sandbox | ✅ Done |
| Backup/DR | ⚠️ Partial (PG only) |
| CI/Security | ✅ Done (no SAST) |
| ExecutionReceipt | ❌ Missing |
| OpenAPI/SDK | ❌ Missing |
