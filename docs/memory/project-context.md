# Project Context: hermes-agent-go

| 字段 | 值 |
|------|-----|
| 项目名 | hermes-agent-go |
| 当前任务 | `2026-04-28-saas-readiness` |
| 阶段 | `released` (P0-P5 代码交付完成，有条件放行) |
| 更新时间 | 2026-04-28 |

---

## Tech Stack

- Go 1.21+
- PostgreSQL (primary store, pgx/v5)
- SQLite (local dev store)
- Redis (session lock existing, rate limit planned)
- golang-jwt/jwt/v5 (JWT RS256 验证)
- prometheus/client_golang (可观测性指标)
- Helm chart (Kubernetes 部署)
- Docker Compose (local dev)
- http.NewServeMux (routing, 2 separate servers: ACP + API)

## 当前任务

SaaS Readiness Phase 0-5 全量交付完成。23 个新文件 + 3 个修改文件。1153 tests 全量通过。5 CRITICAL + 6 HIGH 安全问题已修复。有条件放行（Conditional Go）。

## 依赖

- Phase 0-2 SaaS 无状态化已交付（`2026-04-27-saas-stateless`, CLOSED）
- Redis 已集成用于 session lock
- PG migration 已版本化（T0a 完成）
- K8s 集群信息待 devops 提供（部署前需要）
- golang-jwt/jwt/v5 和 prometheus/client_golang 已加入 go.mod

## 风险

- R1: CRIT-1 ACP auth bypass — pre-existing code, `HERMES_ACP_TOKEN` 未设置时全放行，生产部署前必须修复
- R2: Middleware chain 已构建但未挂载到 HTTP server — 需独立 PR 完成
- R3: 新增 23 个文件缺少专门单元测试 — 需补充至少 80% 覆盖率
- R4: Redis 分布式限流未对接 — localLimiter 单副本可用，多副本需 Redis
- R5: Helm secrets 以明文环境变量注入 — 生产需 ExternalSecrets

## 下一步

1. **CRIT-1 修复**：`internal/acp/auth.go` 强制 `HERMES_ACP_TOKEN` 校验（生产阻塞项）
2. **Middleware chain 挂载**：将 chain.go 连接到 HTTP server（独立 PR）
3. **补充单元测试**：23 个新文件至少 80% 覆盖率
4. **Redis 限流对接**：分布式 rate limiter 实际连接
5. **Helm 加固**：ExternalSecrets、SecurityContext、Ingress、HPA、PDB
